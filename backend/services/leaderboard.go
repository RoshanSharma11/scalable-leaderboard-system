package services

import (
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"matiks-backend/models"
	"matiks-backend/snapshot"
	"matiks-backend/utils"
)

const (
	MinRating        = 100
	MaxRating        = 5000
	InitialUsers     = 10000
	UpdateIntervalMs = 100
	SnapshotInterval = 100 * time.Millisecond
	UpdateBufferSize = 10000
)

type RatingUpdate struct {
	UserID    int
	NewRating int
}

type LeaderboardService struct {
	users map[int]*models.User

	// N-GRAM SEARCH INDEX
	// Maps n-gram to list of user IDs containing that gram in their username.
	// Used for scalable substring search.
	searchIndex map[string][]int

	currentSnapshot atomic.Value // *snapshot.LeaderboardSnapshot

	// All rating updates are sent to this buffered channel.
	// The writer goroutine consumes them asynchronously.
	updateChan chan RatingUpdate

	writerRatings map[int]int // userID -> rating (writer's working copy)

	// Random source for update simulator (used only by simulator goroutine)
	rng *rand.Rand
}

func NewLeaderboardService() *LeaderboardService {
	service := &LeaderboardService{
		users:         make(map[int]*models.User, InitialUsers),
		searchIndex:   make(map[string][]int),
		updateChan:    make(chan RatingUpdate, UpdateBufferSize),
		writerRatings: make(map[int]int, InitialUsers),
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	service.initializeUsers()

	go service.snapshotWriter()  // Single writer: consumes updates, builds snapshots
	go service.updateSimulator() // Simulator: generates random rating updates

	return service
}

func (s *LeaderboardService) initializeUsers() {
	builder := snapshot.NewSnapshotBuilder()

	for userID := 1; userID <= InitialUsers; userID++ {
		username := utils.GenerateRandomUsername(userID)
		rating := utils.GenerateRandomRating(MinRating, MaxRating)

		user := &models.User{
			ID:       userID,
			Username: username,
		}
		s.users[userID] = user

		s.indexUsername(userID, username)

		// Initialize writer's working copy
		s.writerRatings[userID] = rating

		builder.AddUser(userID, username, rating)
	}

	firstSnapshot := builder.Build()
	s.currentSnapshot.Store(firstSnapshot)
}

// This is the ONLY way readers access leaderboard data.
func (s *LeaderboardService) GetSnapshot() *snapshot.LeaderboardSnapshot {
	return s.currentSnapshot.Load().(*snapshot.LeaderboardSnapshot)
}

func (s *LeaderboardService) GetLeaderboard(limit int) []models.LeaderboardEntry {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	snap := s.GetSnapshot()

	result := make([]models.LeaderboardEntry, 0, limit)

	for rating := MaxRating; rating >= MinRating; rating-- {
		users := snap.UsersByRating[rating]
		if len(users) == 0 {
			continue
		}

		rank := snap.GetRank(rating)

		for _, userSum := range users {
			result = append(result, models.LeaderboardEntry{
				Rank:     rank,
				Username: userSum.Username,
				Rating:   userSum.Rating,
			})

			if len(result) >= limit {
				return result
			}
		}
	}

	return result
}

func (s *LeaderboardService) Search(query string) []models.LeaderboardEntry {
	if query == "" {
		return []models.LeaderboardEntry{}
	}

	query = strings.ToLower(query)

	snap := s.GetSnapshot()

	queryGrams := generateNGrams(query)
	if len(queryGrams) == 0 {
		// Query too short or no valid grams, fallback to linear scan
		return s.linearScanSearch(query, snap)
	}

	candidateIDs := s.intersectPostingLists(queryGrams)

	results := make([]models.LeaderboardEntry, 0, len(candidateIDs))

	// Verify candidates and build results
	for userID := range candidateIDs {
		user := s.users[userID]
		lowerUsername := strings.ToLower(user.Username)

		// Filter false positives
		if !strings.Contains(lowerUsername, query) {
			continue
		}

		rating := snap.GetUserRating(userID)
		rank := snap.GetRank(rating)

		results = append(results, models.LeaderboardEntry{
			Rank:     rank,
			Username: user.Username,
			Rating:   rating,
		})
	}

	return results
}

func (s *LeaderboardService) GetStats() map[string]interface{} {
	snap := s.GetSnapshot()

	return map[string]interface{}{
		"total_users":     snap.TotalUsers(),
		"snapshot_age_ms": time.Since(snap.GeneratedAt).Milliseconds(),
		"min_rating":      MinRating,
		"max_rating":      MaxRating,
	}
}

func (s *LeaderboardService) snapshotWriter() {
	ticker := time.NewTicker(SnapshotInterval)
	defer ticker.Stop()

	pendingUpdates := false

	for {
		select {
		case update := <-s.updateChan:
			s.writerRatings[update.UserID] = update.NewRating
			pendingUpdates = true

		case <-ticker.C:
			if pendingUpdates {
				s.rebuildSnapshot()
				pendingUpdates = false
			}
		}

		drained := false
		for !drained {
			select {
			case update := <-s.updateChan:
				s.writerRatings[update.UserID] = update.NewRating
				pendingUpdates = true
			default:
				drained = true
			}
		}

		// If we drained updates, build snapshot immediately (don't wait for ticker)
		if pendingUpdates {
			s.rebuildSnapshot()
			pendingUpdates = false
		}
	}
}

func (s *LeaderboardService) rebuildSnapshot() {
	builder := snapshot.NewSnapshotBuilder()

	for userID, rating := range s.writerRatings {
		user := s.users[userID]
		builder.AddUser(userID, user.Username, rating)
	}

	newSnapshot := builder.Build()

	// Atomically publish the new snapshot
	// Readers will see either old or new, never partial
	s.currentSnapshot.Store(newSnapshot)
}

func (s *LeaderboardService) updateSimulator() {
	for {
		sleepMs := 50 + s.rng.Intn(51)
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)

		numUpdates := 5 + s.rng.Intn(11) // 5-15 users

		for i := 0; i < numUpdates; i++ {
			userID := 1 + s.rng.Intn(InitialUsers)
			newRating := utils.GenerateRandomRating(MinRating, MaxRating)

			select {
			case s.updateChan <- RatingUpdate{
				UserID:    userID,
				NewRating: newRating,
			}:
			default:
				// Channel full, drop update
			}
		}
	}
}

func (s *LeaderboardService) indexUsername(userID int, username string) {
	grams := generateNGrams(strings.ToLower(username))
	seen := make(map[string]bool)

	for _, gram := range grams {
		if !seen[gram] {
			s.searchIndex[gram] = append(s.searchIndex[gram], userID)
			seen[gram] = true
		}
	}
}

func generateNGrams(s string) []string {
	if len(s) < 2 {
		return []string{}
	}

	grams := make([]string, 0)
	seen := make(map[string]bool)

	// Generate n-grams of length 2 to 5
	for n := 2; n <= 5 && n <= len(s); n++ {
		for i := 0; i <= len(s)-n; i++ {
			gram := s[i : i+n]
			if !seen[gram] {
				grams = append(grams, gram)
				seen[gram] = true
			}
		}
	}

	return grams
}

func (s *LeaderboardService) intersectPostingLists(grams []string) map[int]bool {
	if len(grams) == 0 {
		return make(map[int]bool)
	}

	// Find shortest posting list to start with (optimization)
	shortestIdx := 0
	shortestLen := len(s.searchIndex[grams[0]])

	for i, gram := range grams {
		listLen := len(s.searchIndex[gram])
		if listLen < shortestLen {
			shortestLen = listLen
			shortestIdx = i
		}
	}

	candidates := make(map[int]bool)
	for _, userID := range s.searchIndex[grams[shortestIdx]] {
		candidates[userID] = true
	}

	// Intersect with remaining lists
	for i, gram := range grams {
		if i == shortestIdx {
			continue
		}

		postingList := s.searchIndex[gram]
		if len(postingList) == 0 {
			return make(map[int]bool)
		}

		postingSet := make(map[int]bool)
		for _, userID := range postingList {
			postingSet[userID] = true
		}

		for userID := range candidates {
			if !postingSet[userID] {
				delete(candidates, userID)
			}
		}

		if len(candidates) == 0 {
			return candidates
		}
	}

	return candidates
}

func (s *LeaderboardService) linearScanSearch(query string, snap *snapshot.LeaderboardSnapshot) []models.LeaderboardEntry {
	results := make([]models.LeaderboardEntry, 0)

	for userID, user := range s.users {
		lowerUsername := strings.ToLower(user.Username)
		if strings.Contains(lowerUsername, query) {
			rating := snap.GetUserRating(userID)
			rank := snap.GetRank(rating)

			results = append(results, models.LeaderboardEntry{
				Rank:     rank,
				Username: user.Username,
				Rating:   rating,
			})
		}
	}

	return results
}
