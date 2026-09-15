package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/incubrix/backend/internal/adapter"
	"github.com/incubrix/backend/internal/cache"
	"github.com/incubrix/backend/internal/handler"
	"github.com/incubrix/backend/internal/service"
)

// rateCacheTTL is how long a base currency's rates are served from memory.
// Frankfurter republishes about once a day, so an hour is well inside the
// window where the numbers cannot have changed.
const rateCacheTTL = time.Hour

// defaultAllowedOrigins covers the ports Vite and Create React App use.
var defaultAllowedOrigins = []string{
	"http://localhost:3000",
	"http://localhost:5173",
	"http://127.0.0.1:3000",
	"http://127.0.0.1:5173",
}

func main() {
	startedAt := time.Now()

	cachedRates := cache.NewRateCache(adapter.NewFrankfurterClient(), rateCacheTTL)
	rates := service.NewCurrencyService(cachedRates)

	allowedOrigins := allowedOrigins()
	log.Printf("CORS allowed origins: %s", strings.Join(allowedOrigins, ", "))

	srv := &http.Server{
		Addr: ":" + port(),
		Handler: handler.NewRouter(handler.Deps{
			StartedAt:      startedAt,
			Rates:          rates,
			AllowedOrigins: allowedOrigins,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
}

// allowedOrigins returns the CORS allowlist from CORS_ALLOWED_ORIGINS, a
// comma-separated list. It defaults to the usual local UI dev servers, and
// accepts "*" to allow every origin.
func allowedOrigins() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if strings.TrimSpace(raw) == "" {
		return defaultAllowedOrigins
	}

	origins := make([]string, 0, len(defaultAllowedOrigins))
	for _, origin := range strings.Split(raw, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

// port returns PORT from the environment, defaulting to 8080.
func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}
