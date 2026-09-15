package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/incubrix/backend/internal/service"
)

var testOrigins = []string{"http://localhost:3000"}

// requestWithCORS sends a request through the full router with a CORS allowlist.
func requestWithCORS(t *testing.T, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	rec := httptest.NewRecorder()
	NewRouter(Deps{
		StartedAt:      time.Now(),
		Rates:          &stubRateService{snapshot: service.RateSnapshot{Base: "USD"}},
		AllowedOrigins: testOrigins,
	}).ServeHTTP(rec, req)
	return rec
}

func TestCORSAllowsListedOrigin(t *testing.T) {
	rec := requestWithCORS(t, http.MethodGet, "/api/rates?base=USD", map[string]string{
		"Origin": "http://localhost:3000",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the caller's origin", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin so caches do not cross-serve", got)
	}
}

func TestCORSWithholdsHeaderFromUnlistedOrigin(t *testing.T) {
	rec := requestWithCORS(t, http.MethodGet, "/api/rates?base=USD", map[string]string{
		"Origin": "http://evil.example",
	})

	// The request is still served; the browser is what withholds the body.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want it absent", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin even when the origin is rejected", got)
	}
}

func TestCORSAnswersPreflight(t *testing.T) {
	rec := requestWithCORS(t, http.MethodOptions, "/api/rates", map[string]string{
		"Origin":                         "http://localhost:3000",
		"Access-Control-Request-Method":  http.MethodGet,
		"Access-Control-Request-Headers": "Authorization",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (the mux would have said 405)", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Authorization" {
		t.Errorf("Access-Control-Allow-Headers = %q, want the requested header echoed", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != preflightMaxAgeSeconds {
		t.Errorf("Access-Control-Max-Age = %q", got)
	}
}

func TestCORSRejectsPreflightFromUnlistedOrigin(t *testing.T) {
	rec := requestWithCORS(t, http.MethodOptions, "/api/rates", map[string]string{
		"Origin":                        "http://evil.example",
		"Access-Control-Request-Method": http.MethodGet,
	})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want it absent", got)
	}
}

func TestCORSIgnoresRequestsWithoutOrigin(t *testing.T) {
	rec := requestWithCORS(t, http.MethodGet, "/api/rates?base=USD", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Vary"); got != "" {
		t.Errorf("Vary = %q, want nothing for a non-browser caller", got)
	}
}

func TestCORSWildcardAllowsAnyOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/rates?base=USD", nil)
	req.Header.Set("Origin", "http://anywhere.example")

	rec := httptest.NewRecorder()
	NewRouter(Deps{
		StartedAt:      time.Now(),
		Rates:          &stubRateService{snapshot: service.RateSnapshot{Base: "USD"}},
		AllowedOrigins: []string{allowAllOrigins},
	}).ServeHTTP(rec, req)

	// The caller's origin is echoed rather than "*", so the same code path
	// keeps working if credentialed requests are ever enabled.
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://anywhere.example" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSOriginMatchIsCaseInsensitive(t *testing.T) {
	rec := requestWithCORS(t, http.MethodGet, "/api/rates?base=USD", map[string]string{
		"Origin": "http://LOCALHOST:3000",
	})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("Access-Control-Allow-Origin absent, want scheme/host matched case-insensitively")
	}
}

func TestCORSAppliesToHealthEndpoint(t *testing.T) {
	rec := requestWithCORS(t, http.MethodGet, "/api/health", map[string]string{
		"Origin": "http://localhost:3000",
	})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want CORS on every route", got)
	}
}
