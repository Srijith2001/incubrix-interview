// Package service holds the application's business rules.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/incubrix/backend/internal/adapter"
)

// Errors returned by the currency service.
var (
	// ErrInvalidCurrencyCode means a code is not three alphabetic characters.
	ErrInvalidCurrencyCode = errors.New("invalid currency code")
	// ErrUnsupportedCurrency means the provider does not quote a requested code.
	ErrUnsupportedCurrency = errors.New("unsupported currency")
	// ErrProviderUnavailable means the rate provider could not be reached.
	ErrProviderUnavailable = errors.New("exchange rate provider unavailable")
)

// RateProvider is the upstream this service reads rates from.
type RateProvider interface {
	FetchLatestRates(ctx context.Context, base string, quotes []string) ([]adapter.CurrencyRate, error)
}

// RateSnapshot is the quote set this service hands back to callers.
type RateSnapshot struct {
	Base  string
	Date  string
	Rates map[string]float64
	// StaleDates holds the publication date of any currency quoted earlier
	// than Date, so callers can tell a fresh rate from a days-old one.
	StaleDates map[string]string
}

// CurrencyService answers exchange rate questions.
type CurrencyService struct {
	provider RateProvider
}

// NewCurrencyService builds the service over a rate provider.
func NewCurrencyService(provider RateProvider) *CurrencyService {
	return &CurrencyService{provider: provider}
}

// GetLatestRates returns the newest rate from base into each target currency.
//
// Base and targets are case-insensitive and de-duplicated. An empty targets
// slice returns every currency the provider quotes. A base listed among the
// targets comes back as 1.
func (s *CurrencyService) GetLatestRates(ctx context.Context, base string, targets []string) (RateSnapshot, error) {
	base, err := normalizeCurrencyCode(base)
	if err != nil {
		return RateSnapshot{}, err
	}

	quotes, err := normalizeTargetCodes(targets)
	if err != nil {
		return RateSnapshot{}, err
	}

	rates, err := s.provider.FetchLatestRates(ctx, base, quotes)
	if err != nil {
		return RateSnapshot{}, translateProviderError(err)
	}

	snapshot := foldRates(base, rates)

	// The provider rejects unknown codes outright, so a gap here means it
	// changed its behaviour. Fail loudly rather than return a quiet hole.
	if missing := missingCodes(quotes, snapshot.Rates); len(missing) > 0 {
		return RateSnapshot{}, fmt.Errorf("%w: %s", ErrUnsupportedCurrency, strings.Join(missing, ", "))
	}

	return snapshot, nil
}

// foldRates collapses per-pair rows into a rate map, recording the newest date
// as the snapshot date and flagging any pair published before it.
func foldRates(base string, rates []adapter.CurrencyRate) RateSnapshot {
	snapshot := RateSnapshot{Base: base, Rates: make(map[string]float64, len(rates))}

	for _, rate := range rates {
		snapshot.Rates[rate.Quote] = rate.Rate
		if rate.Date > snapshot.Date {
			snapshot.Date = rate.Date
		}
	}

	// Dates are ISO-8601, so a plain string compare orders them correctly.
	for _, rate := range rates {
		if rate.Date != "" && rate.Date < snapshot.Date {
			if snapshot.StaleDates == nil {
				snapshot.StaleDates = make(map[string]string)
			}
			snapshot.StaleDates[rate.Quote] = rate.Date
		}
	}

	return snapshot
}

// missingCodes lists requested codes absent from the returned rates.
func missingCodes(requested []string, rates map[string]float64) []string {
	var missing []string
	for _, code := range requested {
		if _, ok := rates[code]; !ok {
			missing = append(missing, code)
		}
	}
	return missing
}

// translateProviderError maps adapter errors onto this package's vocabulary.
func translateProviderError(err error) error {
	var unknownCurrency *adapter.UnknownCurrencyError

	switch {
	case errors.As(err, &unknownCurrency):
		return fmt.Errorf("%w: %s", ErrUnsupportedCurrency, unknownCurrency.Code)
	case errors.Is(err, adapter.ErrUpstreamFailure):
		return fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	default:
		return err
	}
}

// normalizeCurrencyCode upper-cases a currency code and checks its shape.
func normalizeCurrencyCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 3 {
		return "", fmt.Errorf("%w: %q", ErrInvalidCurrencyCode, code)
	}
	for _, char := range code {
		if char < 'A' || char > 'Z' {
			return "", fmt.Errorf("%w: %q", ErrInvalidCurrencyCode, code)
		}
	}
	return code, nil
}

// normalizeTargetCodes validates, upper-cases, de-duplicates and sorts codes.
func normalizeTargetCodes(targets []string) ([]string, error) {
	seen := make(map[string]struct{}, len(targets))
	codes := make([]string, 0, len(targets))

	for _, target := range targets {
		if strings.TrimSpace(target) == "" {
			continue
		}
		code, err := normalizeCurrencyCode(target)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[code]; duplicate {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}

	sort.Strings(codes)
	return codes, nil
}
