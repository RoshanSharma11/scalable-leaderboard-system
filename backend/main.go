package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"matiks-backend/handlers"
	"matiks-backend/services"
)

const (
	DefaultPort = "8000"
)

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the next handler
		next.ServeHTTP(w, r)

		log.Printf("%s %s %s", r.Method, r.RequestURI, time.Since(start))
	})
}

// Recovery middleware for panic handling
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}
	serverAddr := ":" + port

	log.Println("Initializing leaderboard service...")
	startTime := time.Now()

	leaderboardService := services.NewLeaderboardService()

	elapsed := time.Since(startTime)
	log.Printf("Leaderboard service initialized in %v", elapsed)

	stats := leaderboardService.GetStats()
	log.Printf("Stats: %+v", stats)

	handler := handlers.NewHandler(leaderboardService)

	mux := http.NewServeMux()

	mux.HandleFunc("/leaderboard", handler.GetLeaderboard)
	mux.HandleFunc("/search", handler.Search)

	mux.HandleFunc("/health", handler.HealthCheck)
	mux.HandleFunc("/stats", handler.GetStats)

	var handlerWithMiddleware http.Handler = mux
	handlerWithMiddleware = corsMiddleware(handlerWithMiddleware)
	handlerWithMiddleware = loggingMiddleware(handlerWithMiddleware)
	handlerWithMiddleware = recoveryMiddleware(handlerWithMiddleware)

	log.Printf("Starting server on port %s", port)
	log.Println("Available endpoints:")
	log.Println("  GET /leaderboard?limit=N  - Get top N users (default: 100)")
	log.Println("  GET /search?query=xyz     - Search users by username")
	log.Println("  GET /health               - Health check")
	log.Println("  GET /stats                - Service statistics")
	log.Println("CORS enabled for all origins")

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      handlerWithMiddleware,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
