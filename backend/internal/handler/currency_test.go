package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/incubrix/backend/internal/service"
)

// stubRateService replays a canned answer and records the parsed query.
type stubRateService struct {
	gotBase    string
	gotTargets []string
	snapshot   service.RateSnapshot
	err        error
}

func (s *stubRateService) GetLatestRates(_ context.Context, base string, targets []string) (service.RateSnapshot, error) {
	s.gotBase, s.gotTargets = base, targets
	return s.snapshot, s.err
}

func requestRates(t *testing.T, rates RateService, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	NewRouter(Deps{StartedAt: time.Now(), Rates: rates}).ServeHTTP(rec, req)
	return rec
}

func TestHandleLatestRatesOK(t *testing.T) {
	rates := &stubRateService{snapshot: service.RateSnapshot{
		Base: "USD", Date: "2026-09-15",
		Rates: map[string]float64{"EUR": 0.86515, "SGD": 1.2708},
	}}

	rec := requestRates(t, rates, "/api/rates?base=USD&targets=EUR,SGD")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if rates.gotBase != "USD" {
		t.Errorf("base = %q, want USD", rates.gotBase)
	}
	if want := "[EUR SGD]"; fmt.Sprint(rates.gotTargets) != want {
		t.Errorf("targets = %v, want %s", rates.gotTargets, want)
	}

	var body latestRatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Rates["SGD"] != 1.2708 || body.Date != "2026-09-15" {
		t.Errorf("body = %+v", body)
	}
	if body.StaleDates != nil {
		t.Errorf("staleDates = %v, want it omitted when nothing is stale", body.StaleDates)
	}
}

func TestHandleLatestRatesReportsStaleDates(t *testing.T) {
	rates := &stubRateService{snapshot: service.RateSnapshot{
		Base: "USD", Date: "2026-09-15",
		Rates:      map[string]float64{"EUR": 0.86515, "ANG": 1.79},
		StaleDates: map[string]string{"ANG": "2026-09-11"},
	}}

	rec := requestRates(t, rates, "/api/rates?base=USD&targets=EUR,ANG")

	var body latestRatesResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.StaleDates["ANG"] != "2026-09-11" {
		t.Errorf("staleDates = %v, want ANG dated 2026-09-11", body.StaleDates)
	}
}

func TestHandleLatestRatesSplitsRepeatedTargetsParam(t *testing.T) {
	rates := &stubRateService{snapshot: service.RateSnapshot{Base: "USD"}}

	requestRates(t, rates, "/api/rates?base=USD&targets=EUR,SGD&targets=JPY")

	if want := "[EUR SGD JPY]"; fmt.Sprint(rates.gotTargets) != want {
		t.Errorf("targets = %v, want %s", rates.gotTargets, want)
	}
}

func TestHandleLatestRatesMissingBase(t *testing.T) {
	rec := requestRates(t, &stubRateService{}, "/api/rates?targets=EUR")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleLatestRatesErrorStatusMapping(t *testing.T) {
	cases := map[string]struct {
		serviceErr error
		want       int
	}{
		"invalid code":         {service.ErrInvalidCurrencyCode, http.StatusBadRequest},
		"unsupported currency": {service.ErrUnsupportedCurrency, http.StatusBadRequest},
		"provider down":        {service.ErrProviderUnavailable, http.StatusBadGateway},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := requestRates(t, &stubRateService{err: tc.serviceErr}, "/api/rates?base=USD&targets=EUR")
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			var body errorResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error == "" {
				t.Error("error field is empty")
			}
		})
	}
}
