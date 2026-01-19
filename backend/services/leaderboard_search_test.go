package services

import (
	"sort"
	"strings"
	"testing"

	"matiks-backend/models"
	"matiks-backend/snapshot"
)

// =============================================================================
// N-GRAM GENERATION TESTS
// =============================================================================

func TestGenerateNGrams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "normal username",
			input:    "rahul",
			expected: []string{"ra", "rah", "rahu", "rahul", "ah", "ahu", "ahul", "hu", "hul", "ul"},
		},
		{
			name:     "short string (2 chars)",
			input:    "ab",
			expected: []string{"ab"},
		},
		{
			name:     "single char (too short)",
			input:    "a",
			expected: []string{},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "string with underscore",
			input:    "amit_kumar",
			expected: []string{"am", "ami", "amit", "amit_", "mi", "mit", "mit_", "mit_k", "it", "it_", "it_k", "it_ku", "t_", "t_k", "t_ku", "t_kum", "_k", "_ku", "_kum", "_kuma", "ku", "kum", "kuma", "kumar", "um", "uma", "umar", "ma", "mar", "ar"},
		},
		{
			name:     "repeated characters",
			input:    "aaa",
			expected: []string{"aa", "aaa"},
		},
		{
			name:     "long string (6+ chars)",
			input:    "priyanka",
			expected: []string{"pr", "pri", "priy", "priya", "ri", "riy", "riya", "riyan", "iy", "iya", "iyan", "iyank", "ya", "yan", "yank", "yanka", "an", "ank", "anka", "nk", "nka", "ka"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateNGrams(tt.input)

			// Sort both for comparison (order doesn't matter)
			sort.Strings(result)
			sort.Strings(tt.expected)

			if len(result) != len(tt.expected) {
				t.Errorf("Length mismatch: got %d, want %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Want: %v", tt.expected)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("At index %d: got %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGenerateNGrams_NoduplicateGrams(t *testing.T) {
	input := "aaaaaa"
	grams := generateNGrams(input)

	// Check for duplicates
	seen := make(map[string]bool)
	for _, gram := range grams {
		if seen[gram] {
			t.Errorf("Duplicate gram found: %q", gram)
		}
		seen[gram] = true
	}
}

func TestGenerateNGrams_MaxLength5(t *testing.T) {
	input := "verylongusername"
	grams := generateNGrams(input)

	for _, gram := range grams {
		if len(gram) > 5 {
			t.Errorf("Gram exceeds max length 5: %q (length %d)", gram, len(gram))
		}
		if len(gram) < 2 {
			t.Errorf("Gram below min length 2: %q (length %d)", gram, len(gram))
		}
	}
}

// =============================================================================
// INDEX BUILD TESTS
// =============================================================================

func TestIndexUsername(t *testing.T) {
	service := &LeaderboardService{
		searchIndex: make(map[string][]int),
	}

	// Index a single username
	service.indexUsername(1, "rahul")

	// Check that grams were added
	expectedGrams := []string{"ra", "rah", "rahu", "rahul", "ah", "ahu", "ahul", "hu", "hul", "ul"}

	for _, gram := range expectedGrams {
		userIDs, exists := service.searchIndex[gram]
		if !exists {
			t.Errorf("Expected gram %q not found in index", gram)
			continue
		}

		if len(userIDs) != 1 || userIDs[0] != 1 {
			t.Errorf("Gram %q: expected [1], got %v", gram, userIDs)
		}
	}
}

func TestIndexUsername_MultipleUsers(t *testing.T) {
	service := &LeaderboardService{
		searchIndex: make(map[string][]int),
	}

	// Index multiple usernames with overlapping grams
	service.indexUsername(1, "rahul")
	service.indexUsername(2, "rahul_kumar")
	service.indexUsername(3, "amit")

	// Check that "ra" gram contains both rahul users
	raUsers := service.searchIndex["ra"]
	if len(raUsers) != 2 {
		t.Errorf("Expected 2 users for gram 'ra', got %d", len(raUsers))
	}

	// Check that "amit" specific grams only contain amit
	amitUsers := service.searchIndex["am"]
	if len(amitUsers) != 1 || amitUsers[0] != 3 {
		t.Errorf("Expected [3] for gram 'am', got %v", amitUsers)
	}
}

func TestIndexUsername_CaseInsensitive(t *testing.T) {
	service := &LeaderboardService{
		searchIndex: make(map[string][]int),
	}

	// Index with different cases
	service.indexUsername(1, "Rahul")
	service.indexUsername(2, "RAHUL")
	service.indexUsername(3, "rahul")

	// All should be indexed under lowercase grams
	raUsers := service.searchIndex["ra"]
	if len(raUsers) != 3 {
		t.Errorf("Expected 3 users for gram 'ra' (case-insensitive), got %d", len(raUsers))
	}
}

// =============================================================================
// SEARCH CORRECTNESS TESTS
// =============================================================================

func TestSearch_ExactMatch(t *testing.T) {
	service := createTestService()

	results := service.Search("amit")

	// Should find all users with "amit" in username
	if len(results) == 0 {
		t.Error("Expected at least one result for 'amit'")
	}

	// Verify all results contain "amit"
	for _, result := range results {
		if !strings.Contains(strings.ToLower(result.Username), "amit") {
			t.Errorf("Result %q does not contain 'amit'", result.Username)
		}
	}
}

func TestSearch_PrefixMatch(t *testing.T) {
	service := createTestService()

	results := service.Search("rahu")

	// Should find usernames starting with "rahu" (rahul, etc.)
	if len(results) == 0 {
		t.Error("Expected at least one result for 'rahu'")
	}

	for _, result := range results {
		if !strings.Contains(strings.ToLower(result.Username), "rahu") {
			t.Errorf("Result %q does not contain 'rahu'", result.Username)
		}
	}
}

func TestSearch_SubstringMatch(t *testing.T) {
	service := createTestService()

	results := service.Search("kumar")

	// Should find usernames containing "kumar" anywhere
	if len(results) == 0 {
		t.Error("Expected at least one result for 'kumar'")
	}

	for _, result := range results {
		if !strings.Contains(strings.ToLower(result.Username), "kumar") {
			t.Errorf("Result %q does not contain 'kumar'", result.Username)
		}
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	service := createTestService()

	// Search with different cases
	lower := service.Search("amit")
	upper := service.Search("AMIT")
	mixed := service.Search("AmIt")

	// All should return same results
	if len(lower) != len(upper) || len(lower) != len(mixed) {
		t.Errorf("Case-insensitive search failed: lower=%d, upper=%d, mixed=%d",
			len(lower), len(upper), len(mixed))
	}
}

func TestSearch_NoFalsePositives(t *testing.T) {
	service := createTestService()

	results := service.Search("xyz123impossible")

	// Should return no results (no username contains this)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for impossible query, got %d", len(results))
	}
}

func TestSearch_SingleCharacter(t *testing.T) {
	service := createTestService()

	// Single char should fallback to linear scan
	results := service.Search("a")

	// Should find all usernames containing 'a'
	if len(results) == 0 {
		t.Error("Expected results for single character 'a'")
	}

	for _, result := range results {
		if !strings.Contains(strings.ToLower(result.Username), "a") {
			t.Errorf("Result %q does not contain 'a'", result.Username)
		}
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	service := createTestService()

	results := service.Search("")

	// Empty query should return empty results
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty query, got %d", len(results))
	}
}

// =============================================================================
// RANK CORRECTNESS TESTS
// =============================================================================

func TestSearch_RankCorrectness(t *testing.T) {
	service := createTestService()

	results := service.Search("amit")

	// Verify each result has correct rank
	snap := service.GetSnapshot()

	for _, result := range results {
		expectedRank := snap.GetRank(result.Rating)
		if result.Rank != expectedRank {
			t.Errorf("User %q: rank mismatch. Got %d, expected %d (rating=%d)",
				result.Username, result.Rank, expectedRank, result.Rating)
		}
	}
}

func TestSearch_LiveRanks(t *testing.T) {
	service := createTestService()

	// Get initial results
	results1 := service.Search("amit")
	if len(results1) == 0 {
		t.Skip("No results found for 'amit'")
	}

	firstResult := results1[0]

	// Find the userID for this user
	var userID int
	for id, user := range service.users {
		if user.Username == firstResult.Username {
			userID = id
			break
		}
	}

	// Update rating directly in writer's working copy
	newRating := 5000 // Set to max rating
	service.writerRatings[userID] = newRating

	// Rebuild snapshot with new rating
	service.rebuildSnapshot()

	// Search again
	results2 := service.Search("amit")

	// Find the same user in new results
	var newResult *models.LeaderboardEntry
	for i := range results2 {
		if results2[i].Username == firstResult.Username {
			newResult = &results2[i]
			break
		}
	}

	if newResult == nil {
		t.Fatal("User not found in second search")
	}

	// Verify rank changed
	if newResult.Rating != newRating {
		t.Errorf("Rating not updated: got %d, want %d", newResult.Rating, newRating)
	}

	if newResult.Rank >= firstResult.Rank && firstResult.Rating != newRating {
		t.Errorf("Rank should improve after rating increase: old rank=%d, new rank=%d",
			firstResult.Rank, newResult.Rank)
	}
}

// =============================================================================
// POSTING LIST INTERSECTION TESTS
// =============================================================================

func TestIntersectPostingLists_SingleGram(t *testing.T) {
	service := &LeaderboardService{
		searchIndex: map[string][]int{
			"ab": {1, 2, 3},
		},
	}

	candidates := service.intersectPostingLists([]string{"ab"})

	if len(candidates) != 3 {
		t.Errorf("Expected 3 candidates, got %d", len(candidates))
	}

	for _, id := range []int{1, 2, 3} {
		if !candidates[id] {
			t.Errorf("Expected user %d in candidates", id)
		}
	}
}

func TestIntersectPostingLists_MultipleGrams(t *testing.T) {
	service := &LeaderboardService{
		searchIndex: map[string][]int{
			"ab": {1, 2, 3, 4},
			"bc": {2, 3, 4, 5},
			"cd": {3, 4, 5, 6},
		},
	}

	// Intersection of all three: only 3 and 4 appear in all
	candidates := service.intersectPostingLists([]string{"ab", "bc", "cd"})

	if len(candidates) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(candidates))
	}

	if !candidates[3] || !candidates[4] {
		t.Errorf("Expected candidates 3 and 4, got %v", candidates)
	}
}

func TestIntersectPostingLists_EmptyIntersection(t *testing.T) {
	service := &LeaderboardService{
		searchIndex: map[string][]int{
			"ab": {1, 2},
			"cd": {3, 4},
		},
	}

	// No common users
	candidates := service.intersectPostingLists([]string{"ab", "cd"})

	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates, got %d", len(candidates))
	}
}

func TestIntersectPostingLists_MissingGram(t *testing.T) {
	service := &LeaderboardService{
		searchIndex: map[string][]int{
			"ab": {1, 2, 3},
		},
	}

	// "xyz" doesn't exist in index
	candidates := service.intersectPostingLists([]string{"ab", "xyz"})

	// Should return empty (one gram has no users)
	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates for missing gram, got %d", len(candidates))
	}
}

// =============================================================================
// PERFORMANCE TESTS
// =============================================================================

func BenchmarkSearch_ShortQuery(b *testing.B) {
	service := createTestService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.Search("amit")
	}
}

func BenchmarkSearch_MediumQuery(b *testing.B) {
	service := createTestService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.Search("kumar")
	}
}

func BenchmarkSearch_LongQuery(b *testing.B) {
	service := createTestService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.Search("rahul_kumar")
	}
}

func BenchmarkGenerateNGrams(b *testing.B) {
	username := "rahul_kumar_sharma"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateNGrams(username)
	}
}

func BenchmarkIntersectPostingLists(b *testing.B) {
	service := &LeaderboardService{
		searchIndex: map[string][]int{
			"ra": makeRange(1, 100),
			"ah": makeRange(20, 120),
			"hu": makeRange(40, 140),
		},
	}

	grams := []string{"ra", "ah", "hu"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.intersectPostingLists(grams)
	}
}

// =============================================================================
// TEST HELPERS
// =============================================================================

// createTestService creates a minimal service for testing search functionality
func createTestService() *LeaderboardService {
	service := &LeaderboardService{
		users:         make(map[int]*models.User),
		searchIndex:   make(map[string][]int),
		writerRatings: make(map[int]int),
	}

	// Create test users with realistic names
	testUsers := []struct {
		id       int
		username string
		rating   int
	}{
		{1, "amit", 4500},
		{2, "amit_kumar", 4300},
		{3, "rahul", 4700},
		{4, "rahul_sharma", 4200},
		{5, "priya", 4600},
		{6, "rahul_kumar", 4100},
		{7, "neha", 4400},
		{8, "amit_sharma", 4000},
		{9, "deepak", 3900},
		{10, "priyanka", 3800},
	}

	// Build snapshot
	builder := snapshot.NewSnapshotBuilder()

	for _, u := range testUsers {
		user := &models.User{
			ID:       u.id,
			Username: u.username,
		}
		service.users[u.id] = user
		service.writerRatings[u.id] = u.rating
		service.indexUsername(u.id, u.username)
		builder.AddUser(u.id, u.username, u.rating)
	}

	// Build and store snapshot
	snap := builder.Build()
	service.currentSnapshot.Store(snap)

	return service
}

// makeRange creates a slice of integers from start to end (inclusive)
func makeRange(start, end int) []int {
	result := make([]int, end-start+1)
	for i := range result {
		result[i] = start + i
	}
	return result
}
