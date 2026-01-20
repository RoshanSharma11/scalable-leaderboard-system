package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"matiks-backend/services"
)

type Handler struct {
	leaderboardService *services.LeaderboardService
}

func NewHandler(service *services.LeaderboardService) *Handler {
	return &Handler{
		leaderboardService: service,
	}
}

func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default

	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
		limit = parsedLimit
	}

	leaderboard := h.leaderboardService.GetLeaderboard(limit)

	// Cache for 2 seconds (matches our snapshot rebuild interval)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=2, s-maxage=2")
	w.Header().Set("CDN-Cache-Control", "max-age=2")

	if err := json.NewEncoder(w).Encode(leaderboard); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query parameter is required", http.StatusBadRequest)
		return
	}

	results := h.leaderboardService.Search(query)

	// Add cache headers (shorter TTL for search since results change)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=1, s-maxage=1")
	w.Header().Set("CDN-Cache-Control", "max-age=1")

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  results,
		"count": len(results),
		"query": query,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.leaderboardService.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}
