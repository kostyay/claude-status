// Package pricing fetches Anthropic model pricing from models.dev and
// computes session cost from token usage. Used as a fallback when Claude
// Code does not provide total_cost_usd on stdin (e.g., older versions or
// standalone -test mode).
package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kostyay/claude-status/internal/tokens"
)

// DefaultURL is the models.dev API endpoint.
const DefaultURL = "https://models.dev/api.json"

// DefaultTTL is how long a cached pricing snapshot is considered fresh.
const DefaultTTL = 7 * 24 * time.Hour

// Price holds per-million-token USD pricing for a single model.
type Price struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

// Table maps Anthropic model IDs to their Price.
type Table map[string]Price

// modelsDevAPI is the shape we care about in api.json: providers → models → cost.
type modelsDevAPI struct {
	Anthropic struct {
		Models map[string]struct {
			Cost Price `json:"cost"`
		} `json:"models"`
	} `json:"anthropic"`
}

// cacheFile is what we persist locally. SavedAt drives TTL checks.
type cacheFile struct {
	SavedAt time.Time `json:"saved_at"`
	Table   Table     `json:"table"`
}

// Provider returns pricing for a model ID. Returned bool is false when the
// model is unknown so callers can skip cost calculation.
type Provider interface {
	Lookup(modelID string) (Price, bool)
}

// Static is an in-memory Provider, useful for tests.
type Static struct{ table Table }

// NewStatic wraps a Table as a Provider.
func NewStatic(t Table) *Static { return &Static{table: t} }

// Lookup implements Provider.
func (s *Static) Lookup(modelID string) (Price, bool) {
	return s.table.Lookup(modelID)
}

// Lookup returns the price for modelID, stripping the "[1m]" context suffix
// Claude Code uses to signal 1M-token mode (pricing is unaffected).
func (t Table) Lookup(modelID string) (Price, bool) {
	if t == nil {
		return Price{}, false
	}
	id := strings.TrimSuffix(modelID, "[1m]")
	if p, ok := t[id]; ok {
		return p, true
	}
	return Price{}, false
}

// Fetch retrieves the latest pricing table from models.dev.
func Fetch(ctx context.Context, url string) (Table, error) {
	if url == "" {
		url = DefaultURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pricing: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pricing: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing: fetch %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pricing: read body: %w", err)
	}
	return parseAPI(body)
}

func parseAPI(body []byte) (Table, error) {
	var api modelsDevAPI
	if err := json.Unmarshal(body, &api); err != nil {
		return nil, fmt.Errorf("pricing: decode api.json: %w", err)
	}
	t := make(Table, len(api.Anthropic.Models))
	for id, m := range api.Anthropic.Models {
		t[id] = m.Cost
	}
	if len(t) == 0 {
		return nil, fmt.Errorf("pricing: no anthropic models in response")
	}
	return t, nil
}

// Load returns a pricing table, preferring a fresh on-disk cache. If the
// cache is missing or older than ttl, it refetches and rewrites. Returns
// the stale cache on fetch failure so cost calc keeps working offline.
func Load(ctx context.Context, cacheDir, url string, ttl time.Duration) (Table, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	path := filepath.Join(cacheDir, "models.json")
	cached, cachedErr := readCache(path)
	if cachedErr == nil && time.Since(cached.SavedAt) < ttl {
		return cached.Table, nil
	}
	fresh, fetchErr := Fetch(ctx, url)
	if fetchErr != nil {
		if cachedErr == nil {
			return cached.Table, nil
		}
		return nil, fetchErr
	}
	if err := writeCache(path, fresh); err != nil {
		return fresh, nil
	}
	return fresh, nil
}

func readCache(path string) (cacheFile, error) {
	var c cacheFile
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}

func writeCache(path string, t Table) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cacheFile{SavedAt: time.Now(), Table: t}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Lazy is a Provider that loads the pricing table from cache or network on
// first Lookup and reuses it afterwards. Safe for concurrent use.
type Lazy struct {
	CacheDir string
	URL      string
	TTL      time.Duration
	Timeout  time.Duration

	once  sync.Once
	table Table
}

// Lookup implements Provider, triggering a one-time load.
func (l *Lazy) Lookup(modelID string) (Price, bool) {
	l.once.Do(l.load)
	return l.table.Lookup(modelID)
}

func (l *Lazy) load() {
	timeout := l.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	t, err := Load(ctx, l.CacheDir, l.URL, l.TTL)
	if err == nil {
		l.table = t
	}
}

// Cost returns the USD cost of the given token metrics under price p.
// Prices are $/million tokens.
func Cost(m tokens.Metrics, p Price) float64 {
	const perMillion = 1_000_000.0
	return (float64(m.InputTokens)*p.Input +
		float64(m.OutputTokens)*p.Output +
		float64(m.CacheReadTokens)*p.CacheRead +
		float64(m.CacheCreationTokens)*p.CacheWrite) / perMillion
}
