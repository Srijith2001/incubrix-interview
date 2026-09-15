package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/incubrix/backend/internal/service"
)

// RateService is the behaviour the rates endpoint needs.
type RateService interface {
	GetLatestRates(ctx context.Context, base string, targets []string) (service.RateSnapshot, error)
}

// latestRatesResponse is the payload returned by the rates endpoint.
type latestRatesResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date,omitempty"`
	Rates map[string]float64 `json:"rates"`
	// StaleDates appears only when a currency was published before Date.
	StaleDates map[string]string `json:"staleDates,omitempty"`
}

// HandleLatestRates serves GET /api/rates?base=USD&targets=EUR,SGD.
func HandleLatestRates(rates RateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Query().Get("base")
		if strings.TrimSpace(base) == "" {
			writeError(w, http.StatusBadRequest, "query parameter 'base' is required")
			return
		}

		snapshot, err := rates.GetLatestRates(r.Context(), base, parseTargets(r))
		switch {
		case errors.Is(err, service.ErrInvalidCurrencyCode),
			errors.Is(err, service.ErrUnsupportedCurrency):
			writeError(w, http.StatusBadRequest, err.Error())
			return
		case errors.Is(err, service.ErrProviderUnavailable):
			writeError(w, http.StatusBadGateway, err.Error())
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, latestRatesResponse{
			Base:       snapshot.Base,
			Date:       snapshot.Date,
			Rates:      snapshot.Rates,
			StaleDates: snapshot.StaleDates,
		})
	}
}

// parseTargets reads the optional, repeatable, comma-separated targets param:
// ?targets=EUR,SGD&targets=JPY.
func parseTargets(r *http.Request) []string {
	var targets []string
	for _, value := range r.URL.Query()["targets"] {
		targets = append(targets, strings.Split(value, ",")...)
	}
	return targets
}
