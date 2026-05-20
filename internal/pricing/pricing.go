// Package pricing fetches Anthropic per-model rates from models.dev and
// computes session cost. Acts as a fallback when Claude Code does not
// supply total_cost_usd on stdin (older versions, standalone -test runs).
package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kostyay/claude-status/internal/tokens"
)

const (
	DefaultURL = "https://models.dev/api.json"
	DefaultTTL = 7 * 24 * time.Hour

	defaultHTTPTimeout = 5 * time.Second
	cacheFilename      = "models.json"
)

// Price holds per-million-token USD pricing for a single model.
type Price struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

type Table map[string]Price

type modelsDevAPI struct {
	Anthropic struct {
		Models map[string]struct {
			Cost Price `json:"cost"`
		} `json:"models"`
	} `json:"anthropic"`
}

type cacheFile struct {
	SavedAt time.Time `json:"saved_at"`
	Table   Table     `json:"table"`
}

// Provider returns pricing for a model ID. ok is false for unknown models so
// callers can skip cost calculation rather than charging zero.
type Provider interface {
	Lookup(modelID string) (Price, bool)
}

type Static struct{ table Table }

func NewStatic(t Table) *Static { return &Static{table: t} }

func (s *Static) Lookup(modelID string) (Price, bool) {
	return s.table.Lookup(modelID)
}

func (t Table) Lookup(modelID string) (Price, bool) {
	if t == nil {
		return Price{}, false
	}
	if p, ok := t[tokens.NormalizeModelID(modelID)]; ok {
		return p, true
	}
	return Price{}, false
}

// httpClient enforces a transport-level timeout even when callers forget to
// pass a deadline-bearing context.
var httpClient = &http.Client{Timeout: defaultHTTPTimeout}

func Fetch(ctx context.Context, url string) (Table, error) {
	if url == "" {
		url = DefaultURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pricing: build request: %w", err)
	}
	resp, err := httpClient.Do(req)
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

// Load returns a pricing table, preferring fresh on-disk cache and refetching
// when stale. Falls back to stale cache on network failure so cost calc keeps
// working offline.
func Load(ctx context.Context, cacheDir, url string, ttl time.Duration) (Table, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	path := filepath.Join(cacheDir, cacheFilename)
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
		slog.Debug("pricing: write cache failed", "path", path, "err", err)
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

// writeCache writes atomically (tmp + rename) to avoid leaving a partial file
// if the process is killed mid-write or two runs race.
func writeCache(path string, t Table) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cacheFile{SavedAt: time.Now(), Table: t}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), cacheFilename+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// Lazy loads the pricing table from cache or network on first Lookup and
// reuses it for the remainder of the process. Safe for concurrent use.
type Lazy struct {
	cacheDir string
	once     sync.Once
	table    Table
}

func NewLazy(cacheDir string) *Lazy { return &Lazy{cacheDir: cacheDir} }

func (l *Lazy) Lookup(modelID string) (Price, bool) {
	l.once.Do(l.load)
	return l.table.Lookup(modelID)
}

func (l *Lazy) load() {
	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()
	if t, err := Load(ctx, l.cacheDir, "", 0); err == nil {
		l.table = t
	}
}

// Cost returns USD cost for the given token metrics under price p.
// Price fields are $/million tokens.
func Cost(m tokens.Metrics, p Price) float64 {
	const perMillion = 1_000_000.0
	return (float64(m.InputTokens)*p.Input +
		float64(m.OutputTokens)*p.Output +
		float64(m.CacheReadTokens)*p.CacheRead +
		float64(m.CacheCreationTokens)*p.CacheWrite) / perMillion
}
