// Package cache holds in-memory caching for outbound data.
package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/incubrix/backend/internal/adapter"
)

// RateProvider is the upstream a cache sits in front of.
type RateProvider interface {
	FetchLatestRates(ctx context.Context, base string, quotes []string) ([]adapter.CurrencyRate, error)
}

// cacheEntry is one base currency's full rate set and the time it was fetched.
type cacheEntry struct {
	rates     []adapter.CurrencyRate
	fetchedAt time.Time
}

// RateCache caches exchange rates per base currency for a fixed TTL.
//
// A miss fetches every currency upstream quotes for that base, not just the
// ones asked for, because both cost the same single request. Any later request
// for the same base is then served by filtering in memory, whatever its
// targets. The key space is the set of currencies, so the cache is bounded
// without needing eviction.
//
// It satisfies the same interface as the client it wraps, so callers cannot
// tell the difference.
type RateCache struct {
	upstream RateProvider
	ttl      time.Duration
	// now is swappable so tests can age entries without sleeping.
	now func() time.Time

	mu      sync.RWMutex
	entries map[string]cacheEntry

	// fetchGroup collapses concurrent misses on one base into a single
	// upstream call; the rest wait for and share its result.
	fetchGroup singleflight.Group
}

// NewRateCache wraps a provider, holding each base's rates for ttl.
func NewRateCache(upstream RateProvider, ttl time.Duration) *RateCache {
	return &RateCache{
		upstream: upstream,
		ttl:      ttl,
		now:      time.Now,
		entries:  make(map[string]cacheEntry),
	}
}

// FetchLatestRates returns rates for base, serving them from memory when a
// fetch for that base is younger than the TTL.
func (c *RateCache) FetchLatestRates(ctx context.Context, base string, quotes []string) ([]adapter.CurrencyRate, error) {
	if rates, ok := c.lookup(base); ok {
		return filterQuotes(rates, quotes), nil
	}

	rates, err := c.fetchAndStore(ctx, base)
	if err != nil {
		return nil, err
	}
	return filterQuotes(rates, quotes), nil
}

// lookup returns a base's cached rates if the entry has not expired.
func (c *RateCache) lookup(base string) ([]adapter.CurrencyRate, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.entries[base]
	if !found || c.now().Sub(entry.fetchedAt) >= c.ttl {
		return nil, false
	}
	return entry.rates, true
}

// fetchAndStore loads every rate for base from upstream and caches it. Only one
// call per base runs at a time; concurrent callers share its outcome.
func (c *RateCache) fetchAndStore(ctx context.Context, base string) ([]adapter.CurrencyRate, error) {
	fetched, err, _ := c.fetchGroup.Do(base, func() (any, error) {
		// Look again now that this call owns the key. The caller's own lookup
		// missed, but an earlier flight may have stored a fresh entry between
		// then and now; without this, a stampede whose callers arrive just
		// after each other still sends one request per caller upstream.
		if rates, ok := c.lookup(base); ok {
			return rates, nil
		}

		// The result is shared, so the fetch must not die with whichever
		// caller happened to trigger it. The client's own timeout still
		// bounds how long this can run.
		rates, err := c.upstream.FetchLatestRates(context.WithoutCancel(ctx), base, nil)
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.entries[base] = cacheEntry{rates: rates, fetchedAt: c.now()}
		c.mu.Unlock()

		return rates, nil
	})
	if err != nil {
		return nil, err
	}
	return fetched.([]adapter.CurrencyRate), nil
}

// filterQuotes narrows a full rate set to the requested quote currencies. An
// empty quotes slice means the caller wants everything.
//
// A quote the provider does not publish is simply absent from the result; the
// caller is what decides whether that is an error.
func filterQuotes(rates []adapter.CurrencyRate, quotes []string) []adapter.CurrencyRate {
	if len(quotes) == 0 {
		return rates
	}

	wanted := make(map[string]struct{}, len(quotes))
	for _, quote := range quotes {
		wanted[quote] = struct{}{}
	}

	filtered := make([]adapter.CurrencyRate, 0, len(quotes))
	for _, rate := range rates {
		if _, ok := wanted[rate.Quote]; ok {
			filtered = append(filtered, rate)
		}
	}
	return filtered
}
