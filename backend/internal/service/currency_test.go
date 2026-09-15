package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/incubrix/backend/internal/adapter"
)

// stubRateProvider records what it was asked and replays a canned answer.
type stubRateProvider struct {
	gotBase   string
	gotQuotes []string
	rates     []adapter.CurrencyRate
	err       error
}

func (s *stubRateProvider) FetchLatestRates(_ context.Context, base string, quotes []string) ([]adapter.CurrencyRate, error) {
	s.gotBase, s.gotQuotes = base, quotes
	return s.rates, s.err
}

func TestGetLatestRatesNormalizesInput(t *testing.T) {
	provider := &stubRateProvider{rates: []adapter.CurrencyRate{
		{Date: "2026-09-15", Base: "USD", Quote: "EUR", Rate: 0.86515},
		{Date: "2026-09-15", Base: "USD", Quote: "SGD", Rate: 1.2708},
	}}

	got, err := NewCurrencyService(provider).GetLatestRates(context.Background(), " usd ", []string{"sgd", "eur", "EUR", ""})
	if err != nil {
		t.Fatalf("GetLatestRates: %v", err)
	}

	if provider.gotBase != "USD" {
		t.Errorf("base = %q, want USD", provider.gotBase)
	}
	if want := []string{"EUR", "SGD"}; !reflect.DeepEqual(provider.gotQuotes, want) {
		t.Errorf("quotes = %v, want %v", provider.gotQuotes, want)
	}
	if want := map[string]float64{"EUR": 0.86515, "SGD": 1.2708}; !reflect.DeepEqual(got.Rates, want) {
		t.Errorf("rates = %v, want %v", got.Rates, want)
	}
	if got.Date != "2026-09-15" {
		t.Errorf("date = %q, want 2026-09-15", got.Date)
	}
	if got.StaleDates != nil {
		t.Errorf("staleDates = %v, want nil when every date agrees", got.StaleDates)
	}
}

func TestGetLatestRatesPassesBaseThroughToProvider(t *testing.T) {
	// v2 quotes the base against itself, so the service must not strip it.
	provider := &stubRateProvider{rates: []adapter.CurrencyRate{
		{Date: "2026-09-15", Base: "USD", Quote: "EUR", Rate: 0.86515},
		{Date: "2026-09-15", Base: "USD", Quote: "USD", Rate: 1},
	}}

	got, err := NewCurrencyService(provider).GetLatestRates(context.Background(), "USD", []string{"USD", "EUR"})
	if err != nil {
		t.Fatalf("GetLatestRates: %v", err)
	}
	if want := []string{"EUR", "USD"}; !reflect.DeepEqual(provider.gotQuotes, want) {
		t.Errorf("quotes = %v, want %v", provider.gotQuotes, want)
	}
	if got.Rates["USD"] != 1 {
		t.Errorf("rates[USD] = %v, want 1", got.Rates["USD"])
	}
}

func TestGetLatestRatesFlagsStaleDates(t *testing.T) {
	// Thinly traded currencies lag: upstream dates each pair separately.
	provider := &stubRateProvider{rates: []adapter.CurrencyRate{
		{Date: "2026-09-15", Base: "USD", Quote: "EUR", Rate: 0.86515},
		{Date: "2026-09-11", Base: "USD", Quote: "ANG", Rate: 1.79},
	}}

	got, err := NewCurrencyService(provider).GetLatestRates(context.Background(), "USD", []string{"EUR", "ANG"})
	if err != nil {
		t.Fatalf("GetLatestRates: %v", err)
	}
	if got.Date != "2026-09-15" {
		t.Errorf("date = %q, want the newest row date 2026-09-15", got.Date)
	}
	if want := map[string]string{"ANG": "2026-09-11"}; !reflect.DeepEqual(got.StaleDates, want) {
		t.Errorf("staleDates = %v, want %v", got.StaleDates, want)
	}
}

func TestGetLatestRatesRejectsMissingTarget(t *testing.T) {
	// Guards against upstream silently dropping a code instead of erroring.
	provider := &stubRateProvider{rates: []adapter.CurrencyRate{
		{Date: "2026-09-15", Base: "USD", Quote: "EUR", Rate: 0.86515},
	}}

	_, err := NewCurrencyService(provider).GetLatestRates(context.Background(), "USD", []string{"EUR", "SGD"})
	if !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("err = %v, want ErrUnsupportedCurrency", err)
	}
}

func TestGetLatestRatesInvalidCodes(t *testing.T) {
	cases := map[string]struct {
		base    string
		targets []string
	}{
		"short base":   {"US", nil},
		"long base":    {"USDD", nil},
		"digits base":  {"US1", nil},
		"empty base":   {"", nil},
		"bad target":   {"USD", []string{"EU"}},
		"digit target": {"USD", []string{"EU1"}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewCurrencyService(&stubRateProvider{}).GetLatestRates(context.Background(), tc.base, tc.targets)
			if !errors.Is(err, ErrInvalidCurrencyCode) {
				t.Fatalf("err = %v, want ErrInvalidCurrencyCode", err)
			}
		})
	}
}

func TestGetLatestRatesTranslatesProviderErrors(t *testing.T) {
	cases := map[string]struct {
		providerErr error
		want        error
	}{
		"unknown currency": {&adapter.UnknownCurrencyError{Code: "ZZZ"}, ErrUnsupportedCurrency},
		"upstream failure": {adapter.ErrUpstreamFailure, ErrProviderUnavailable},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			provider := &stubRateProvider{err: tc.providerErr}
			_, err := NewCurrencyService(provider).GetLatestRates(context.Background(), "USD", []string{"EUR"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestGetLatestRatesNamesTheBadCode(t *testing.T) {
	provider := &stubRateProvider{err: &adapter.UnknownCurrencyError{Code: "ZZZ"}}

	_, err := NewCurrencyService(provider).GetLatestRates(context.Background(), "USD", []string{"ZZZ"})
	if got := err.Error(); got != "unsupported currency: ZZZ" {
		t.Fatalf("err = %q, want \"unsupported currency: ZZZ\"", got)
	}
}
