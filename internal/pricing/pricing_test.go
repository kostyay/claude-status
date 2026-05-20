package pricing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kostyay/claude-status/internal/tokens"
)

const sampleAPI = `{
  "anthropic": {
    "models": {
      "claude-opus-4-7": {"cost": {"input": 5, "output": 25, "cache_read": 0.5, "cache_write": 6.25}},
      "claude-sonnet-4-6": {"cost": {"input": 3, "output": 15, "cache_read": 0.3, "cache_write": 3.75}}
    }
  }
}`

func TestParseAPI(t *testing.T) {
	tbl, err := parseAPI([]byte(sampleAPI))
	if err != nil {
		t.Fatalf("parseAPI: %v", err)
	}
	if got, want := tbl["claude-opus-4-7"].Input, 5.0; got != want {
		t.Errorf("opus input = %v, want %v", got, want)
	}
	if got, want := tbl["claude-sonnet-4-6"].CacheWrite, 3.75; got != want {
		t.Errorf("sonnet cache_write = %v, want %v", got, want)
	}
}

func TestParseAPI_NoAnthropicModels(t *testing.T) {
	_, err := parseAPI([]byte(`{"anthropic":{"models":{}}}`))
	if err == nil {
		t.Fatal("expected error for empty models")
	}
}

func TestTableLookup_StripsBracketSuffix(t *testing.T) {
	tbl := Table{"claude-opus-4-7": {Input: 5}}
	p, ok := tbl.Lookup("claude-opus-4-7[1m]")
	if !ok {
		t.Fatal("expected lookup hit when [1m] suffix is present")
	}
	if p.Input != 5 {
		t.Errorf("Input = %v, want 5", p.Input)
	}
}

func TestTableLookup_Miss(t *testing.T) {
	tbl := Table{"claude-opus-4-7": {Input: 5}}
	if _, ok := tbl.Lookup("gpt-4"); ok {
		t.Error("expected miss for unknown model")
	}
	if _, ok := Table(nil).Lookup("anything"); ok {
		t.Error("expected miss for nil table")
	}
}

func TestCost(t *testing.T) {
	m := tokens.Metrics{
		InputTokens:         1_000_000,
		OutputTokens:        500_000,
		CacheReadTokens:     2_000_000,
		CacheCreationTokens: 100_000,
	}
	p := Price{Input: 5, Output: 25, CacheRead: 0.5, CacheWrite: 6.25}
	// 1M*5 + 0.5M*25 + 2M*0.5 + 0.1M*6.25 = 5 + 12.5 + 1 + 0.625 = 19.125
	got := Cost(m, p)
	want := 19.125
	if got != want {
		t.Errorf("Cost = %v, want %v", got, want)
	}
}

func TestLoad_FreshFromNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(sampleAPI))
	}))
	defer srv.Close()

	dir := t.TempDir()
	tbl, err := Load(context.Background(), dir, srv.URL, time.Hour)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := tbl["claude-opus-4-7"]; !ok {
		t.Error("expected opus in fresh table")
	}
	// Should have written cache file
	if _, err := os.Stat(filepath.Join(dir, "models.json")); err != nil {
		t.Errorf("cache file not written: %v", err)
	}
}

func TestLoad_UsesFreshCache(t *testing.T) {
	dir := t.TempDir()
	// Seed a fresh cache; server must never be hit.
	seed := cacheFile{
		SavedAt: time.Now(),
		Table:   Table{"claude-opus-4-7": {Input: 99}},
	}
	b, _ := json.Marshal(seed)
	if err := os.WriteFile(filepath.Join(dir, "models.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Write([]byte(sampleAPI))
	}))
	defer srv.Close()

	tbl, err := Load(context.Background(), dir, srv.URL, time.Hour)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if hits != 0 {
		t.Errorf("server was hit %d times; expected 0 when cache is fresh", hits)
	}
	if tbl["claude-opus-4-7"].Input != 99 {
		t.Errorf("did not return cached table: %+v", tbl)
	}
}

func TestLoad_StaleCacheRefetches(t *testing.T) {
	dir := t.TempDir()
	seed := cacheFile{
		SavedAt: time.Now().Add(-30 * 24 * time.Hour),
		Table:   Table{"claude-opus-4-7": {Input: 99}},
	}
	b, _ := json.Marshal(seed)
	os.WriteFile(filepath.Join(dir, "models.json"), b, 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(sampleAPI))
	}))
	defer srv.Close()

	tbl, err := Load(context.Background(), dir, srv.URL, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tbl["claude-opus-4-7"].Input != 5 {
		t.Errorf("expected refetched price 5, got %v", tbl["claude-opus-4-7"].Input)
	}
}

func TestLoad_FallsBackToStaleOnFetchFailure(t *testing.T) {
	dir := t.TempDir()
	seed := cacheFile{
		SavedAt: time.Now().Add(-30 * 24 * time.Hour),
		Table:   Table{"claude-opus-4-7": {Input: 42}},
	}
	b, _ := json.Marshal(seed)
	os.WriteFile(filepath.Join(dir, "models.json"), b, 0o644)

	// Server that always 500s.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()

	tbl, err := Load(context.Background(), dir, srv.URL, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tbl["claude-opus-4-7"].Input != 42 {
		t.Errorf("expected stale cache value 42, got %v", tbl["claude-opus-4-7"].Input)
	}
}

func TestLoad_NoCacheAndFetchFailureReturnsError(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()
	if _, err := Load(context.Background(), dir, srv.URL, time.Hour); err == nil {
		t.Error("expected error when no cache and fetch fails")
	}
}
