package snapshot

import (
	"sort"
	"time"
)

type UserSummary struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Rating   int    `json:"rating"`
}

type LeaderboardSnapshot struct {
	UserRatings map[int]int // userID -> rating

	RatingCount [5001]int // rating -> count

	// PrefixHigher[rating] = number of DISTINCT rating levels above this rating (dense ranking)
	// - Makes rank lookup O(1) instead of O(R) where R = rating range
	// - Formula: rank = PrefixHigher[rating] + 1
	// - This scales to millions of users without performance degradation
	//
	// Example (Dense Ranking):
	//   5 users at rating 5000
	//   3 users at rating 4999
	//   2 users at rating 4998
	//
	//   PrefixHigher[5000] = 0    : rank 1
	//   PrefixHigher[4999] = 1    : rank 2 (1 + 1)
	//   PrefixHigher[4998] = 2    : rank 3 (1 + 2)
	PrefixHigher [5001]int // rating -> distinct rating levels above

	UsersByRating map[int][]UserSummary // rating -> users at that rating

	GeneratedAt time.Time
}

func (s *LeaderboardSnapshot) GetRank(rating int) int {
	if rating < 0 || rating >= len(s.PrefixHigher) {
		return 1 // Default to rank 1 for out-of-bounds ratings
	}
	return s.PrefixHigher[rating] + 1
}

func (s *LeaderboardSnapshot) GetUserRating(userID int) int {
	return s.UserRatings[userID]
}

func (s *LeaderboardSnapshot) TotalUsers() int {
	return len(s.UserRatings)
}

// SnapshotBuilder helps construct a new immutable LeaderboardSnapshot.
type SnapshotBuilder struct {
	userRatings map[int]int
	usernames   map[int]string
}

func NewSnapshotBuilder() *SnapshotBuilder {
	return &SnapshotBuilder{
		userRatings: make(map[int]int),
		usernames:   make(map[int]string),
	}
}

func (b *SnapshotBuilder) AddUser(userID int, username string, rating int) {
	b.userRatings[userID] = rating
	b.usernames[userID] = username
}

func (b *SnapshotBuilder) Build() *LeaderboardSnapshot {
	snap := &LeaderboardSnapshot{
		UserRatings:   make(map[int]int, len(b.userRatings)),
		UsersByRating: make(map[int][]UserSummary),
		GeneratedAt:   time.Now(),
	}

	// Copy user ratings and count rating frequencies
	for userID, rating := range b.userRatings {
		snap.UserRatings[userID] = rating
		if rating >= 0 && rating < len(snap.RatingCount) {
			snap.RatingCount[rating]++
		}
	}

	// Compute PrefixHigher for dense ranking
	distinctLevels := 0
	for rating := 5000; rating >= 0; rating-- {
		snap.PrefixHigher[rating] = distinctLevels
		if snap.RatingCount[rating] > 0 {
			distinctLevels++
		}
	}

	// Group users by rating for leaderboard generation
	for userID, rating := range b.userRatings {
		username := b.usernames[userID]
		summary := UserSummary{
			ID:       userID,
			Username: username,
			Rating:   rating,
		}
		snap.UsersByRating[rating] = append(snap.UsersByRating[rating], summary)
	}

	for rating := range snap.UsersByRating {
		users := snap.UsersByRating[rating]
		if len(users) > 1 {
			sort.Slice(users, func(i, j int) bool {
				return users[i].ID < users[j].ID
			})
			snap.UsersByRating[rating] = users
		}
	}

	return snap
}
