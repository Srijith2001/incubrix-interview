// Command index is the Vercel entrypoint for the API.
//
// Vercel's Go runtime runs a normal HTTP server that listens on $PORT, so this
// wires up the same router cmd/server does. It exists separately because Vercel
// requires the entrypoint to live under api/, and because the lifecycle differs:
// the platform handles shutdown, so there is no signal handling here.
//
// Everything below the handler package — service, cache, adapter — is shared
// with the App Runner deployment unchanged.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/incubrix/backend/internal/adapter"
	"github.com/incubrix/backend/internal/cache"
	"github.com/incubrix/backend/internal/handler"
	"github.com/incubrix/backend/internal/service"
)

// rateCacheTTL matches cmd/server. Note that on a serverless platform the cache
// is per-instance and dies with it, so it helps within a warm instance and not
// across cold starts.
const rateCacheTTL = time.Hour

func main() {
	startedAt := time.Now()

	cachedRates := cache.NewRateCache(adapter.NewFrankfurterClient(), rateCacheTTL)
	rates := service.NewCurrencyService(cachedRates)

	// The frontend proxies /api/* to this deployment from its own origin, so
	// requests arrive same-origin and no allowlist is needed. Direct callers
	// still get an allowlist; "*" here keeps the API usable from anywhere,
	// which is appropriate for a read-only public rates endpoint.
	allowedOrigins := []string{"*"}
	if raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); raw != "" {
		allowedOrigins = nil
		for _, origin := range strings.Split(raw, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}

	router := handler.NewRouter(handler.Deps{
		StartedAt:      startedAt,
		Rates:          rates,
		AllowedOrigins: allowedOrigins,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           withAPIPrefix(router),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

// withAPIPrefix makes the router indifferent to whether the platform forwards
// the original path or strips the /api mount point. The routes are registered
// as /api/health and /api/rates; a request that arrives as /health is rewritten
// so it still matches, rather than 404ing depending on how the request was
// routed upstream.
func withAPIPrefix(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			r.URL.Path = "/api" + r.URL.Path
		}
		next.ServeHTTP(w, r)
	})
}
