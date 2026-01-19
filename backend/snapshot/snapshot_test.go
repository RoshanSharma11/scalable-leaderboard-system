package snapshot

import (
	"testing"
)

// TestSnapshotBuilder tests the snapshot building process.
func TestSnapshotBuilder(t *testing.T) {
	t.Run("Empty snapshot", func(t *testing.T) {
		builder := NewSnapshotBuilder()
		snap := builder.Build()

		if snap.TotalUsers() != 0 {
			t.Errorf("Expected 0 users, got %d", snap.TotalUsers())
		}

		// Rank for any rating should be 1 (empty leaderboard)
		if rank := snap.GetRank(3000); rank != 1 {
			t.Errorf("Expected rank 1 for empty snapshot, got %d", rank)
		}
	})

	t.Run("Single user", func(t *testing.T) {
		builder := NewSnapshotBuilder()
		builder.AddUser(1, "alice", 5000)
		snap := builder.Build()

		if snap.TotalUsers() != 1 {
			t.Errorf("Expected 1 user, got %d", snap.TotalUsers())
		}

		if rating := snap.GetUserRating(1); rating != 5000 {
			t.Errorf("Expected rating 5000, got %d", rating)
		}

		if rank := snap.GetRank(5000); rank != 1 {
			t.Errorf("Expected rank 1, got %d", rank)
		}
	})

	t.Run("Multiple users same rating", func(t *testing.T) {
		builder := NewSnapshotBuilder()
		builder.AddUser(1, "alice", 5000)
		builder.AddUser(2, "bob", 5000)
		builder.AddUser(3, "charlie", 5000)
		snap := builder.Build()

		// All should have rank 1
		if rank := snap.GetRank(5000); rank != 1 {
			t.Errorf("Expected rank 1 for rating 5000, got %d", rank)
		}

		// RatingCount should be 3
		if count := snap.RatingCount[5000]; count != 3 {
			t.Errorf("Expected RatingCount[5000] = 3, got %d", count)
		}

		// PrefixHigher[5000] should be 0 (no one higher)
		if prefix := snap.PrefixHigher[5000]; prefix != 0 {
			t.Errorf("Expected PrefixHigher[5000] = 0, got %d", prefix)
		}
	})

	t.Run("Dense ranking correctness", func(t *testing.T) {
		builder := NewSnapshotBuilder()

		// 5 users at rating 5000 → rank 1
		for i := 1; i <= 5; i++ {
			builder.AddUser(i, "user", 5000)
		}

		// 3 users at rating 4999 → rank 2 (dense ranking: no skipped ranks)
		for i := 6; i <= 8; i++ {
			builder.AddUser(i, "user", 4999)
		}

		// 1 user at rating 4998 → rank 3 (dense ranking: 1, 2, 3)
		builder.AddUser(9, "user", 4998)

		snap := builder.Build()

		// Verify PrefixHigher computation for dense ranking
		// PrefixHigher[r] = count of distinct rating levels above r
		if prefix := snap.PrefixHigher[5000]; prefix != 0 {
			t.Errorf("PrefixHigher[5000] should be 0, got %d", prefix)
		}

		if prefix := snap.PrefixHigher[4999]; prefix != 1 {
			t.Errorf("PrefixHigher[4999] should be 1, got %d", prefix)
		}

		if prefix := snap.PrefixHigher[4998]; prefix != 2 {
			t.Errorf("PrefixHigher[4998] should be 2, got %d", prefix)
		}

		// Verify ranks
		if rank := snap.GetRank(5000); rank != 1 {
			t.Errorf("Rank for 5000 should be 1, got %d", rank)
		}

		if rank := snap.GetRank(4999); rank != 2 {
			t.Errorf("Rank for 4999 should be 2, got %d", rank)
		}

		if rank := snap.GetRank(4998); rank != 3 {
			t.Errorf("Rank for 4998 should be 3, got %d", rank)
		}
	})
}

// TestPrefixHigherCorrectness verifies that PrefixHigher is computed correctly.
func TestPrefixHigherCorrectness(t *testing.T) {
	t.Run("Sequential ratings", func(t *testing.T) {
		builder := NewSnapshotBuilder()

		// Create users with ratings 5000, 4999, 4998, ..., 4990
		for i := 0; i < 11; i++ {
			rating := 5000 - i
			builder.AddUser(i+1, "user", rating)
		}

		snap := builder.Build()

		// Verify PrefixHigher[r] = count of users with rating > r
		for i := 0; i < 11; i++ {
			rating := 5000 - i
			expectedPrefix := i // number of users with higher rating

			if prefix := snap.PrefixHigher[rating]; prefix != expectedPrefix {
				t.Errorf("PrefixHigher[%d] should be %d, got %d", rating, expectedPrefix, prefix)
			}

			expectedRank := expectedPrefix + 1
			if rank := snap.GetRank(rating); rank != expectedRank {
				t.Errorf("Rank for rating %d should be %d, got %d", rating, expectedRank, rank)
			}
		}
	})

	t.Run("Boundary ratings", func(t *testing.T) {
		builder := NewSnapshotBuilder()
		builder.AddUser(1, "top", 5000)
		builder.AddUser(2, "bottom", 100)
		snap := builder.Build()

		// Top rating should have rank 1
		if rank := snap.GetRank(5000); rank != 1 {
			t.Errorf("Top rating should have rank 1, got %d", rank)
		}

		// Bottom rating should have rank 2
		if rank := snap.GetRank(100); rank != 2 {
			t.Errorf("Bottom rating should have rank 2, got %d", rank)
		}

		// PrefixHigher[100] = 1 (one user at 5000)
		if prefix := snap.PrefixHigher[100]; prefix != 1 {
			t.Errorf("PrefixHigher[100] should be 1, got %d", prefix)
		}
	})
}

// TestUsersByRating verifies that users are correctly grouped by rating.
func TestUsersByRating(t *testing.T) {
	builder := NewSnapshotBuilder()

	builder.AddUser(1, "alice", 5000)
	builder.AddUser(2, "bob", 5000)
	builder.AddUser(3, "charlie", 4999)

	snap := builder.Build()

	// Check users at rating 5000
	users5000 := snap.UsersByRating[5000]
	if len(users5000) != 2 {
		t.Errorf("Expected 2 users at rating 5000, got %d", len(users5000))
	}

	// Check users at rating 4999
	users4999 := snap.UsersByRating[4999]
	if len(users4999) != 1 {
		t.Errorf("Expected 1 user at rating 4999, got %d", len(users4999))
	}

	// Verify user summaries
	for _, user := range users5000 {
		if user.Rating != 5000 {
			t.Errorf("User in UsersByRating[5000] has wrong rating: %d", user.Rating)
		}
	}
}

// TestRatingCountAccuracy verifies that RatingCount is accurate.
func TestRatingCountAccuracy(t *testing.T) {
	builder := NewSnapshotBuilder()

	// Add 10 users at each rating: 5000, 4000, 3000, 2000, 1000
	ratings := []int{5000, 4000, 3000, 2000, 1000}
	userID := 1

	for _, rating := range ratings {
		for i := 0; i < 10; i++ {
			builder.AddUser(userID, "user", rating)
			userID++
		}
	}

	snap := builder.Build()

	// Verify RatingCount
	for _, rating := range ratings {
		if count := snap.RatingCount[rating]; count != 10 {
			t.Errorf("RatingCount[%d] should be 10, got %d", rating, count)
		}
	}

	// Total users should be 50
	if total := snap.TotalUsers(); total != 50 {
		t.Errorf("Expected 50 total users, got %d", total)
	}
}

// TestConcurrentSnapshotReads tests that snapshots can be read concurrently.
func TestConcurrentSnapshotReads(t *testing.T) {
	builder := NewSnapshotBuilder()

	// Build a snapshot with 1000 users
	for i := 1; i <= 1000; i++ {
		rating := 100 + (i % 4900) // ratings from 100 to 5000
		builder.AddUser(i, "user", rating)
	}

	snap := builder.Build()

	// Launch 100 goroutines reading from the same snapshot
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func() {
			// Each goroutine performs 1000 reads
			for j := 0; j < 1000; j++ {
				rating := 100 + (j % 4900)
				_ = snap.GetRank(rating)
				_ = snap.GetUserRating(1 + (j % 1000))
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	// If we reach here without panic, concurrent reads are safe
	t.Log("Concurrent snapshot reads completed successfully")
}

// BenchmarkSnapshotBuild benchmarks snapshot construction.
func BenchmarkSnapshotBuild(b *testing.B) {
	userCounts := []int{1000, 10000, 100000}

	for _, count := range userCounts {
		b.Run(benchName(count, "users"), func(b *testing.B) {
			// Pre-generate user data
			userIDs := make([]int, count)
			usernames := make([]string, count)
			ratings := make([]int, count)

			for i := 0; i < count; i++ {
				userIDs[i] = i + 1
				usernames[i] = "user"
				ratings[i] = 100 + (i % 4900)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				builder := NewSnapshotBuilder()
				for j := 0; j < count; j++ {
					builder.AddUser(userIDs[j], usernames[j], ratings[j])
				}
				_ = builder.Build()
			}
		})
	}
}

// BenchmarkGetRank benchmarks O(1) rank lookup.
func BenchmarkGetRank(b *testing.B) {
	builder := NewSnapshotBuilder()

	// Build snapshot with 10,000 users
	for i := 1; i <= 10000; i++ {
		rating := 100 + (i % 4900)
		builder.AddUser(i, "user", rating)
	}

	snap := builder.Build()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rating := 100 + (i % 4900)
		_ = snap.GetRank(rating)
	}
}

func benchName(value int, suffix string) string {
	return string(rune(value)) + suffix
}
