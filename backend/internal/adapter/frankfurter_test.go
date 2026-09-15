package adapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient points a Frankfurter client at a stub server.
func newTestClient(srv *httptest.Server) *FrankfurterClient {
	return &FrankfurterClient{baseURL: srv.URL, httpClient: srv.Client()}
}

func TestFetchLatestRatesBuildsQueryAndDecodes(t *testing.T) {
	var gotPath, gotBase, gotQuotes string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBase = r.URL.Query().Get("base")
		gotQuotes = r.URL.Query().Get("quotes")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"date":"2026-09-15","base":"USD","quote":"EUR","rate":0.86515},
			{"date":"2026-09-15","base":"USD","quote":"SGD","rate":1.2708}
		]`))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).FetchLatestRates(context.Background(), "USD", []string{"EUR", "SGD"})
	if err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}

	if gotPath != "/rates" {
		t.Errorf("path = %q, want /rates", gotPath)
	}
	if gotBase != "USD" || gotQuotes != "EUR,SGD" {
		t.Errorf("query base=%q quotes=%q, want USD / EUR,SGD", gotBase, gotQuotes)
	}
	if len(got) != 2 || got[0].Quote != "EUR" || got[0].Rate != 0.86515 || got[0].Date != "2026-09-15" {
		t.Errorf("got = %+v", got)
	}
}

func TestFetchLatestRatesOmitsQuotesWhenNoTargets(t *testing.T) {
	var hadQuotes bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadQuotes = r.URL.Query()["quotes"]
		_, _ = w.Write([]byte(`[{"date":"2026-09-15","base":"USD","quote":"EUR","rate":0.865}]`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).FetchLatestRates(context.Background(), "USD", nil); err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}
	if hadQuotes {
		t.Error("quotes param sent, want it omitted")
	}
}

func TestFetchLatestRatesUnknownCurrency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"status":422,"message":"invalid currency: ZZZ"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).FetchLatestRates(context.Background(), "USD", []string{"ZZZ"})
	if !errors.Is(err, ErrUnknownCurrency) {
		t.Fatalf("err = %v, want ErrUnknownCurrency", err)
	}
	// The offending code must survive into the message.
	if got := err.Error(); got != "frankfurter: unknown currency: ZZZ" {
		t.Errorf("err = %q, want it to name ZZZ", got)
	}
}

func TestFetchLatestRatesErrorMapping(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   error
	}{
		"unknown parameter": {http.StatusUnprocessableEntity, `{"status":422,"message":"unknown parameter: symbols"}`, ErrUpstreamFailure},
		"server error":      {http.StatusInternalServerError, `{"status":500,"message":"boom"}`, ErrUpstreamFailure},
		"rate limited":      {http.StatusTooManyRequests, `slow down`, ErrUpstreamFailure},
		"not found":         {http.StatusNotFound, `{"status":404,"message":"not found"}`, ErrUpstreamFailure},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := newTestClient(srv).FetchLatestRates(context.Background(), "USD", nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFetchLatestRatesBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).FetchLatestRates(context.Background(), "USD", nil)
	if !errors.Is(err, ErrUpstreamFailure) {
		t.Fatalf("err = %v, want ErrUpstreamFailure", err)
	}
}

func TestFetchLatestRatesRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newTestClient(srv).FetchLatestRates(ctx, "USD", nil)
	if !errors.Is(err, ErrUpstreamFailure) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want ErrUpstreamFailure wrapping context.Canceled", err)
	}
}
