package cache

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/incubrix/backend/internal/adapter"
)

// countingProvider counts upstream calls and records the quotes asked for.
type countingProvider struct {
	mu        sync.Mutex
	calls     atomic.Int64
	gotQuotes [][]string
	rates     []adapter.CurrencyRate
	err       error
	// block, when set, holds every call open until it is closed.
	block chan struct{}
}

func (p *countingProvider) FetchLatestRates(_ context.Context, _ string, quotes []string) ([]adapter.CurrencyRate, error) {
	p.calls.Add(1)

	p.mu.Lock()
	p.gotQuotes = append(p.gotQuotes, quotes)
	p.mu.Unlock()

	if p.block != nil {
		<-p.block
	}
	return p.rates, p.err
}

func allRates() []adapter.CurrencyRate {
	return []adapter.CurrencyRate{
		{Date: "2026-09-15", Base: "USD", Quote: "EUR", Rate: 0.86515},
		{Date: "2026-09-15", Base: "USD", Quote: "SGD", Rate: 1.2708},
		{Date: "2026-09-11", Base: "USD", Quote: "ANG", Rate: 1.79},
	}
}

func TestCacheServesSecondRequestFromMemory(t *testing.T) {
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	for i := 0; i < 3; i++ {
		if _, err := rateCache.FetchLatestRates(context.Background(), "USD", []string{"EUR"}); err != nil {
			t.Fatalf("FetchLatestRates: %v", err)
		}
	}

	if got := provider.calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1", got)
	}
}

func TestCacheFetchesEveryCurrencyOnMiss(t *testing.T) {
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	if _, err := rateCache.FetchLatestRates(context.Background(), "USD", []string{"EUR"}); err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}

	// Asking upstream for everything costs the same as asking for one.
	if got := provider.gotQuotes[0]; got != nil {
		t.Errorf("upstream quotes = %v, want nil so the whole set is cached", got)
	}
}

func TestCacheServesDifferentTargetsFromOneEntry(t *testing.T) {
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	first, err := rateCache.FetchLatestRates(context.Background(), "USD", []string{"EUR"})
	if err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}
	// A different target set must not cost a second upstream call.
	second, err := rateCache.FetchLatestRates(context.Background(), "USD", []string{"SGD", "ANG"})
	if err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}

	if got := provider.calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1", got)
	}
	if len(first) != 1 || first[0].Quote != "EUR" {
		t.Errorf("first = %+v, want EUR only", first)
	}
	if len(second) != 2 {
		t.Errorf("second = %+v, want two rates", second)
	}
}

func TestCacheKeepsBasesSeparate(t *testing.T) {
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	for _, base := range []string{"USD", "EUR", "USD"} {
		if _, err := rateCache.FetchLatestRates(context.Background(), base, nil); err != nil {
			t.Fatalf("FetchLatestRates(%s): %v", base, err)
		}
	}

	if got := provider.calls.Load(); got != 2 {
		t.Errorf("upstream calls = %d, want 2 (one per distinct base)", got)
	}
}

func TestCacheRefetchesAfterTTL(t *testing.T) {
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	clock := time.Now()
	rateCache.now = func() time.Time { return clock }

	if _, err := rateCache.FetchLatestRates(context.Background(), "USD", nil); err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}

	// Still inside the TTL.
	clock = clock.Add(59 * time.Minute)
	if _, err := rateCache.FetchLatestRates(context.Background(), "USD", nil); err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d before expiry, want 1", got)
	}

	// Past it.
	clock = clock.Add(2 * time.Minute)
	if _, err := rateCache.FetchLatestRates(context.Background(), "USD", nil); err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}
	if got := provider.calls.Load(); got != 2 {
		t.Errorf("upstream calls = %d after expiry, want 2", got)
	}
}

func TestCacheCollapsesConcurrentMisses(t *testing.T) {
	provider := &countingProvider{rates: allRates(), block: make(chan struct{})}
	rateCache := NewRateCache(provider, time.Hour)

	const callers = 20
	var wg sync.WaitGroup
	started := make(chan struct{}, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started <- struct{}{}
			if _, err := rateCache.FetchLatestRates(context.Background(), "USD", []string{"EUR"}); err != nil {
				t.Errorf("FetchLatestRates: %v", err)
			}
		}()
	}

	// Let every caller reach the cache before the upstream call completes.
	for i := 0; i < callers; i++ {
		<-started
	}
	close(provider.block)
	wg.Wait()

	if got := provider.calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1 (a stampede reached upstream)", got)
	}
}

func TestCacheDoesNotStoreFailures(t *testing.T) {
	provider := &countingProvider{err: adapter.ErrUpstreamFailure}
	rateCache := NewRateCache(provider, time.Hour)

	for i := 0; i < 2; i++ {
		if _, err := rateCache.FetchLatestRates(context.Background(), "USD", nil); !errors.Is(err, adapter.ErrUpstreamFailure) {
			t.Fatalf("err = %v, want ErrUpstreamFailure", err)
		}
	}

	if got := provider.calls.Load(); got != 2 {
		t.Errorf("upstream calls = %d, want 2 (a failure must not be cached)", got)
	}
}

func TestCacheSurvivesCallerCancellation(t *testing.T) {
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	// The shared fetch must not inherit one caller's cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := rateCache.FetchLatestRates(ctx, "USD", []string{"EUR"})
	if err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got = %+v, want one rate", got)
	}
}

func TestCacheOmitsUnknownQuote(t *testing.T) {
	// The caller, not the cache, decides that a missing quote is an error.
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	got, err := rateCache.FetchLatestRates(context.Background(), "USD", []string{"EUR", "ZZZ"})
	if err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}
	if len(got) != 1 || got[0].Quote != "EUR" {
		t.Errorf("got = %+v, want EUR only", got)
	}
}

func TestCacheReturnsEverythingWhenNoQuotes(t *testing.T) {
	provider := &countingProvider{rates: allRates()}
	rateCache := NewRateCache(provider, time.Hour)

	got, err := rateCache.FetchLatestRates(context.Background(), "USD", nil)
	if err != nil {
		t.Fatalf("FetchLatestRates: %v", err)
	}
	if !reflect.DeepEqual(got, allRates()) {
		t.Errorf("got = %+v, want every rate", got)
	}
}
