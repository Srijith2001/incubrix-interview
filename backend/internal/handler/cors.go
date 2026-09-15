package handler

import (
	"net/http"
	"strings"
)

// allowAllOrigins is the wildcard an operator can put in the allowlist to open
// the API to every origin. Incompatible with credentialed requests by design:
// browsers reject "*" when a request carries cookies.
const allowAllOrigins = "*"

// preflightMaxAgeSeconds tells browsers how long to cache a preflight result.
const preflightMaxAgeSeconds = "600"

// withCORS answers cross-origin requests from the allowed origins. It wraps the
// router rather than living on each route so that preflight OPTIONS requests
// are answered here, before the mux rejects them as an unregistered method.
func withCORS(allowedOrigins []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Same-origin and non-browser callers send no Origin and need no headers.
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// The response body depends on the origin, so caches must key on it.
		w.Header().Add("Vary", "Origin")

		allowed := isOriginAllowed(allowedOrigins, origin)
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		if isPreflight(r) {
			if !allowed {
				// Rejecting loudly beats a silent 204 the browser blocks later.
				writeError(w, http.StatusForbidden, "origin not allowed: "+origin)
				return
			}
			writePreflightHeaders(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// A disallowed origin still gets served; the browser withholds the body
		// from the page because no Access-Control-Allow-Origin came back.
		next.ServeHTTP(w, r)
	})
}

// isPreflight reports whether this is a CORS preflight probe rather than a
// real request. The request-method header is what distinguishes the two.
func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

// isOriginAllowed matches an origin against the configured allowlist.
func isOriginAllowed(allowedOrigins []string, origin string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == allowAllOrigins || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

// writePreflightHeaders states what the real request is permitted to send.
func writePreflightHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", preflightMaxAgeSeconds)

	// Echo the headers the browser asked about; it only ever asks for ones the
	// page actually intends to send.
	if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
		w.Header().Set("Access-Control-Allow-Headers", requested)
		w.Header().Add("Vary", "Access-Control-Request-Headers")
	}
}
