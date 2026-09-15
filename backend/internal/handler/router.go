package handler

import (
	"log"
	"net/http"
	"time"
)

// Deps are the collaborators and settings the routes need.
type Deps struct {
	StartedAt time.Time
	Rates     RateService
	// AllowedOrigins is the CORS allowlist. Empty disables cross-origin access.
	AllowedOrigins []string
}

// NewRouter wires every route the server exposes.
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", HandleHealth(deps.StartedAt))
	mux.HandleFunc("GET /api/rates", HandleLatestRates(deps.Rates))

	// CORS sits inside the log so preflight requests are logged too, and
	// outside the mux so it can answer OPTIONS the mux has no route for.
	return logRequests(withCORS(deps.AllowedOrigins, mux))
}

// logRequests writes one line per request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
