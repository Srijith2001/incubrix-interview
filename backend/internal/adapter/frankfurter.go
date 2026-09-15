// Package adapter holds outbound clients for third-party services.
package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultFrankfurterBaseURL = "https://api.frankfurter.dev/v2"

// Errors returned by the Frankfurter client.
var (
	// ErrUnknownCurrency means upstream does not quote a requested code. Its
	// message names the offending code.
	ErrUnknownCurrency = errors.New("frankfurter: unknown currency")
	// ErrUpstreamFailure means upstream was unreachable or misbehaved.
	ErrUpstreamFailure = errors.New("frankfurter: upstream failure")
)

// UnknownCurrencyError names a code the provider does not quote. It carries
// the code itself so callers need not parse the message.
type UnknownCurrencyError struct {
	Code string
}

func (e *UnknownCurrencyError) Error() string {
	return fmt.Sprintf("%s: %s", ErrUnknownCurrency, e.Code)
}

// Unwrap lets errors.Is match this against ErrUnknownCurrency.
func (e *UnknownCurrencyError) Unwrap() error { return ErrUnknownCurrency }

// CurrencyRate is one base-to-quote rate, carrying the day it was published.
// Frankfurter dates each pair separately: a thinly traded currency can be
// several days staler than the rest of the same response.
type CurrencyRate struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

// FrankfurterClient reads exchange rates from the frankfurter.dev v2 API.
type FrankfurterClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewFrankfurterClient builds a client against the public frankfurter.dev API.
func NewFrankfurterClient() *FrankfurterClient {
	return &FrankfurterClient{
		baseURL:    defaultFrankfurterBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchLatestRates returns the newest rate from base into each quote currency.
// An empty quotes slice asks upstream for every currency it knows.
//
// Upstream rejects the whole request with 422 if any single code is unknown, so
// a successful call always covers every currency asked for.
func (c *FrankfurterClient) FetchLatestRates(ctx context.Context, base string, quotes []string) ([]CurrencyRate, error) {
	endpoint, err := c.buildLatestRatesURL(base, quotes)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", ErrUpstreamFailure, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Wrap both so callers can still spot context cancellation or timeout.
		return nil, fmt.Errorf("%w: %w", ErrUpstreamFailure, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, classifyErrorResponse(resp)
	}

	var rates []CurrencyRate
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrUpstreamFailure, err)
	}
	return rates, nil
}

// buildLatestRatesURL assembles the /rates query for one base currency.
func (c *FrankfurterClient) buildLatestRatesURL(base string, quotes []string) (string, error) {
	endpoint, err := url.Parse(c.baseURL + "/rates")
	if err != nil {
		return "", fmt.Errorf("%w: bad base url: %v", ErrUpstreamFailure, err)
	}

	query := endpoint.Query()
	query.Set("base", base)
	if len(quotes) > 0 {
		query.Set("quotes", strings.Join(quotes, ","))
	}
	endpoint.RawQuery = query.Encode()

	return endpoint.String(), nil
}

// classifyErrorResponse turns a non-200 response into a typed error. Upstream
// reports a bad currency code as 422 with a message naming it, which is worth
// passing through to the caller verbatim.
func classifyErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	var upstreamErr struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &upstreamErr)

	message := strings.TrimSpace(upstreamErr.Message)
	if message == "" {
		message = strings.TrimSpace(string(body))
	}

	const unknownCurrencyPrefix = "invalid currency: "
	if resp.StatusCode == http.StatusUnprocessableEntity && strings.HasPrefix(message, unknownCurrencyPrefix) {
		return &UnknownCurrencyError{Code: strings.TrimPrefix(message, unknownCurrencyPrefix)}
	}
	return fmt.Errorf("%w: status %d: %s", ErrUpstreamFailure, resp.StatusCode, message)
}
