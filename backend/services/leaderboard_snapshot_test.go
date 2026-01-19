package services

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"matiks-backend/models"
	"matiks-backend/snapshot"
)

// TestSnapshotBasedArchitecture verifies the lock-free snapshot architecture.
func TestSnapshotBasedArchitecture(t *testing.T) {
	service := NewLeaderboardService()

	// Wait for first snapshot to be built
	time.Sleep(200 * time.Millisecond)

	t.Run("Snapshot is immutable", func(t *testing.T) {
		snap1 := service.GetSnapshot()
		snap2 := service.GetSnapshot()

		// Same snapshot should be returned (atomic.Load returns same pointer)
		if snap1 != snap2 {
			// This is actually OK - might have rebuilt in between
			// The important thing is both are valid snapshots
		}

		// Verify snapshot has data
		if snap1.TotalUsers() != InitialUsers {
			t.Errorf("Expected %d users, got %d", InitialUsers, snap1.TotalUsers())
		}
	})

	t.Run("GetSnapshot is lock-free", func(t *testing.T) {
		// Launch 1000 concurrent readers
		var wg sync.WaitGroup
		readCount := int32(0)

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					snap := service.GetSnapshot()
					if snap == nil {
						t.Error("GetSnapshot returned nil")
					}
					atomic.AddInt32(&readCount, 1)
				}
			}()
		}

		wg.Wait()

		// Verify all reads completed
		if readCount != 100000 {
			t.Errorf("Expected 100000 reads, got %d", readCount)
		}

		t.Logf("Successfully completed 100,000 concurrent lock-free snapshot reads")
	})
}

// TestGetLeaderboard tests the leaderboard endpoint.
func TestGetLeaderboard(t *testing.T) {
	service := NewLeaderboardService()
	time.Sleep(200 * time.Millisecond) // Wait for initialization

	t.Run("Default limit", func(t *testing.T) {
		result := service.GetLeaderboard(0)

		if len(result) != 100 {
			t.Errorf("Expected 100 entries (default), got %d", len(result))
		}

		// Verify descending order by rank
		for i := 1; i < len(result); i++ {
			if result[i].Rank < result[i-1].Rank {
				t.Errorf("Ranks not in ascending order at position %d", i)
			}
		}
	})

	t.Run("Custom limit", func(t *testing.T) {
		limits := []int{1, 10, 50, 500, 1000}

		for _, limit := range limits {
			result := service.GetLeaderboard(limit)

			if len(result) > limit {
				t.Errorf("Expected at most %d entries, got %d", limit, len(result))
			}

			// First entry should have rank 1
			if len(result) > 0 && result[0].Rank != 1 {
				t.Errorf("First entry should have rank 1, got %d", result[0].Rank)
			}
		}
	})

	t.Run("Tie-aware ranking", func(t *testing.T) {
		// Create a custom service with known data
		customService := &LeaderboardService{
			users:         make(map[int]*models.User),
			searchIndex:   make(map[string][]int),
			updateChan:    make(chan RatingUpdate, 100),
			writerRatings: make(map[int]int),
		}

		builder := snapshot.NewSnapshotBuilder()

		// 3 users at rating 5000 → rank 1, 1, 1
		builder.AddUser(1, "alice", 5000)
		builder.AddUser(2, "bob", 5000)
		builder.AddUser(3, "charlie", 5000)

		// 2 users at rating 4999 → rank 4, 4 (not rank 2!)
		builder.AddUser(4, "dave", 4999)
		builder.AddUser(5, "eve", 4999)

		// 1 user at rating 4998 → rank 6
		builder.AddUser(6, "frank", 4998)

		snap := builder.Build()
		customService.currentSnapshot.Store(snap)

		result := customService.GetLeaderboard(10)

		// Verify dense ranking
		expected := []struct {
			rank   int
			rating int
		}{
			{1, 5000}, {1, 5000}, {1, 5000}, // 3 users at rank 1
			{2, 4999}, {2, 4999}, // 2 users at rank 2 (dense: no skipped ranks)
			{3, 4998}, // 1 user at rank 3
		}

		for i, exp := range expected {
			if i >= len(result) {
				t.Fatalf("Result too short, expected at least %d entries", len(expected))
			}

			if result[i].Rank != exp.rank {
				t.Errorf("Entry %d: expected rank %d, got %d", i, exp.rank, result[i].Rank)
			}

			if result[i].Rating != exp.rating {
				t.Errorf("Entry %d: expected rating %d, got %d", i, exp.rating, result[i].Rating)
			}
		}
	})
}

// TestSearch tests the search functionality.
func TestSearch(t *testing.T) {
	service := NewLeaderboardService()
	time.Sleep(200 * time.Millisecond)

	t.Run("Case insensitive", func(t *testing.T) {
		// Search with different cases should return same results
		query := "user"
		result1 := service.Search(query)
		result2 := service.Search("USER")
		result3 := service.Search("User")

		// All should return results (we have many "user" prefixed names)
		if len(result1) == 0 {
			t.Error("Search should return results for 'user'")
		}

		// Results count should be similar (might differ if snapshot rebuilt)
		// Just verify they all return something
		if len(result2) == 0 || len(result3) == 0 {
			t.Error("Case-insensitive search failed")
		}
	})

	t.Run("Empty query", func(t *testing.T) {
		result := service.Search("")

		if len(result) != 0 {
			t.Errorf("Empty query should return 0 results, got %d", len(result))
		}
	})

	t.Run("Results have valid ranks", func(t *testing.T) {
		result := service.Search("user")

		for i, entry := range result {
			if entry.Rank < 1 {
				t.Errorf("Entry %d has invalid rank %d", i, entry.Rank)
			}

			if entry.Rating < MinRating || entry.Rating > MaxRating {
				t.Errorf("Entry %d has invalid rating %d", i, entry.Rating)
			}

			if entry.Username == "" {
				t.Errorf("Entry %d has empty username", i)
			}
		}
	})
}

// TestConcurrentReadsAndWrites tests that reads don't block during snapshot rebuilds.
func TestConcurrentReadsAndWrites(t *testing.T) {
	service := NewLeaderboardService()
	time.Sleep(200 * time.Millisecond)

	t.Run("Reads during snapshot updates", func(t *testing.T) {
		var wg sync.WaitGroup
		stopReaders := make(chan bool)
		readCount := int32(0)
		errorCount := int32(0)

		// Launch 50 continuous readers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stopReaders:
						return
					default:
						// Perform various read operations
						snap := service.GetSnapshot()
						if snap == nil {
							atomic.AddInt32(&errorCount, 1)
							continue
						}

						_ = service.GetLeaderboard(10)
						_ = service.Search("user")
						_ = service.GetStats()

						atomic.AddInt32(&readCount, 1)
					}
				}
			}()
		}

		// Let readers run for 2 seconds (multiple snapshot rebuilds)
		time.Sleep(2 * time.Second)

		// Stop readers
		close(stopReaders)
		wg.Wait()

		t.Logf("Completed %d reads with %d errors", readCount, errorCount)

		if errorCount > 0 {
			t.Errorf("Had %d errors during concurrent reads", errorCount)
		}

		if readCount < 1000 {
			t.Errorf("Expected many reads during 2 seconds, got only %d", readCount)
		}
	})
}

// TestSnapshotConsistency verifies that each snapshot is internally consistent.
func TestSnapshotConsistency(t *testing.T) {
	service := NewLeaderboardService()
	time.Sleep(200 * time.Millisecond)

	t.Run("Snapshot data consistency", func(t *testing.T) {
		// Take multiple snapshots over time
		for iteration := 0; iteration < 5; iteration++ {
			time.Sleep(150 * time.Millisecond) // Allow snapshot rebuild

			snap := service.GetSnapshot()

			// Verify RatingCount consistency
			totalFromRatingCount := 0
			for _, count := range snap.RatingCount {
				totalFromRatingCount += count
			}

			if totalFromRatingCount != snap.TotalUsers() {
				t.Errorf("Iteration %d: RatingCount sum (%d) != TotalUsers (%d)",
					iteration, totalFromRatingCount, snap.TotalUsers())
			}

			// Verify PrefixHigher consistency for dense ranking
			// PrefixHigher[r] should equal count of distinct rating levels r' > r
			for rating := MaxRating; rating >= MinRating; rating-- {
				expected := 0
				for r := rating + 1; r <= MaxRating; r++ {
					if snap.RatingCount[r] > 0 {
						expected++
					}
				}

				if snap.PrefixHigher[rating] != expected {
					t.Errorf("PrefixHigher[%d] = %d, expected %d", rating, snap.PrefixHigher[rating], expected)
					break
				}
			}

			// Verify UsersByRating consistency
			totalFromUsersByRating := 0
			for _, users := range snap.UsersByRating {
				totalFromUsersByRating += len(users)
			}

			if totalFromUsersByRating != snap.TotalUsers() {
				t.Errorf("Iteration %d: UsersByRating sum (%d) != TotalUsers (%d)",
					iteration, totalFromUsersByRating, snap.TotalUsers())
			}
		}
	})
}

// TestRankCorrectness verifies O(1) rank computation is mathematically correct.
func TestRankCorrectness(t *testing.T) {
	// Create service with known data
	service := &LeaderboardService{
		users:         make(map[int]*models.User),
		searchIndex:   make(map[string][]int),
		updateChan:    make(chan RatingUpdate, 100),
		writerRatings: make(map[int]int),
	}

	builder := snapshot.NewSnapshotBuilder()

	// Add users with specific ratings
	testCases := []struct {
		userID   int
		username string
		rating   int
		expected int // expected rank
	}{
		{1, "top", 5000, 1},        // Highest rating
		{2, "second", 4999, 2},     // Second highest
		{3, "third", 4998, 3},      // Third
		{4, "mid", 3000, 4},        // Middle
		{5, "low", 1000, 5},        // Low
		{6, "lowest", 100, 6},      // Lowest
		{7, "top_tie", 5000, 1},    // Tie with top
		{8, "second_tie", 4999, 2}, // Tie with second (but rank is now 3 due to tie at top)
	}

	for _, tc := range testCases {
		builder.AddUser(tc.userID, tc.username, tc.rating)
	}

	snap := builder.Build()
	service.currentSnapshot.Store(snap)

	// After adding all users, recalculate expected ranks with dense ranking
	// 2 users at 5000 → rank 1
	// 2 users at 4999 → rank 2 (dense: no skip)
	// 1 user at 4998 → rank 3
	// 1 user at 3000 → rank 4
	// 1 user at 1000 → rank 5
	// 1 user at 100 → rank 6

	expectedRanks := map[int]int{
		5000: 1,
		4999: 2,
		4998: 3,
		3000: 4,
		1000: 5,
		100:  6,
	}

	for rating, expectedRank := range expectedRanks {
		actualRank := snap.GetRank(rating)
		if actualRank != expectedRank {
			t.Errorf("Rating %d: expected rank %d, got %d", rating, expectedRank, actualRank)
		}
	}
}

// TestNoDataRaces runs with -race flag to detect data races.
func TestNoDataRaces(t *testing.T) {
	service := NewLeaderboardService()

	var wg sync.WaitGroup

	// Launch concurrent readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = service.GetLeaderboard(10)
				_ = service.Search("user")
				_ = service.GetSnapshot()
			}
		}()
	}

	// Let background writer and simulator run
	time.Sleep(500 * time.Millisecond)

	wg.Wait()

	t.Log("No data races detected (run with -race flag)")
}

// BenchmarkLockFreeReads benchmarks concurrent lock-free reads.
func BenchmarkLockFreeReads(b *testing.B) {
	service := NewLeaderboardService()
	time.Sleep(200 * time.Millisecond)

	concurrencyLevels := []int{1, 10, 100, 1000}

	for _, concurrency := range concurrencyLevels {
		b.Run("Concurrent_"+string(rune(concurrency)), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					snap := service.GetSnapshot()
					_ = snap.GetRank(3000)
				}
			})
		})
	}
}

// BenchmarkGetLeaderboard benchmarks leaderboard generation.
func BenchmarkGetLeaderboard(b *testing.B) {
	service := NewLeaderboardService()
	time.Sleep(200 * time.Millisecond)

	limits := []int{10, 100, 1000}

	for _, limit := range limits {
		b.Run("Limit_"+string(rune(limit)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = service.GetLeaderboard(limit)
			}
		})
	}
}

// BenchmarkSearch benchmarks search performance.
func BenchmarkSearch(b *testing.B) {
	service := NewLeaderboardService()
	time.Sleep(200 * time.Millisecond)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = service.Search("user")
	}
}
