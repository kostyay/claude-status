package cache

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kostya/claude-status/internal/git"
	"github.com/kostya/claude-status/internal/github"
)

// Clock is an interface for time operations, allowing for testing.
type Clock interface {
	Now() time.Time
}

// RealClock is the default clock implementation using real time.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// CachedValue holds a cached value with its invalidation metadata.
type CachedValue struct {
	Value     string    `json:"value"`
	FileMtime int64     `json:"file_mtime"` // mtime in nanoseconds
	CachedAt  time.Time `json:"cached_at"`
}

// CachedGitHubBuild holds cached GitHub build status.
type CachedGitHubBuild struct {
	Status    github.BuildStatus `json:"status"`
	FileMtime int64              `json:"file_mtime"`
	CachedAt  time.Time          `json:"cached_at"`
	Branch    string             `json:"branch"`
}

// CachedDiffStats holds cached git diff statistics.
type CachedDiffStats struct {
	Stats     git.DiffStats `json:"stats"`
	FileMtime int64         `json:"file_mtime"`
	CachedAt  time.Time     `json:"cached_at"`
}

// CacheFile is the structure of the cache file on disk.
type CacheFile struct {
	GitBranch   *CachedValue       `json:"git_branch,omitempty"`
	GitStatus   *CachedValue       `json:"git_status,omitempty"`
	GitDiffStats *CachedDiffStats  `json:"git_diff_stats,omitempty"`
	GitHubBuild *CachedGitHubBuild `json:"github_build,omitempty"`
}

// Manager handles cache operations with file-based persistence.
type Manager struct {
	cacheDir  string
	cachePath string
	clock     Clock
	mu        sync.RWMutex
}

// NewManager creates a new cache manager.
func NewManager(cacheDir string) *Manager {
	return NewManagerWithClock(cacheDir, RealClock{})
}

// NewManagerWithClock creates a new cache manager with a custom clock.
func NewManagerWithClock(cacheDir string, clock Clock) *Manager {
	return &Manager{
		cacheDir:  cacheDir,
		cachePath: filepath.Join(cacheDir, "cache.json"),
		clock:     clock,
	}
}

// EnsureDir creates the cache directory if it doesn't exist.
func (m *Manager) EnsureDir() error {
	return os.MkdirAll(m.cacheDir, 0755)
}

// GetGitBranch returns the cached git branch or fetches it if the cache is invalid.
func (m *Manager) GetGitBranch(headPath string, fetchFn func() (string, error)) (string, error) {
	// Get current file mtime
	mtime, err := getFileMtime(headPath)
	if err != nil {
		// Can't stat file, just fetch
		return fetchFn()
	}

	// Check cache
	m.mu.RLock()
	cache := m.load()
	m.mu.RUnlock()

	if cache.GitBranch != nil && cache.GitBranch.FileMtime == mtime {
		return cache.GitBranch.Value, nil
	}

	// Cache miss - fetch and store
	value, err := fetchFn()
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check cache after acquiring write lock (TOCTOU protection)
	cache = m.load()
	if cache.GitBranch != nil && cache.GitBranch.FileMtime == mtime {
		return cache.GitBranch.Value, nil
	}

	cache.GitBranch = &CachedValue{
		Value:     value,
		FileMtime: mtime,
		CachedAt:  m.clock.Now(),
	}
	m.save(cache)

	return value, nil
}

// GetGitStatus returns the cached git status or fetches it if the cache is invalid.
func (m *Manager) GetGitStatus(indexPath string, fetchFn func() (string, error)) (string, error) {
	// Get current file mtime
	mtime, err := getFileMtime(indexPath)
	if err != nil {
		// Can't stat file (maybe no commits yet), just fetch
		return fetchFn()
	}

	// Check cache
	m.mu.RLock()
	cache := m.load()
	m.mu.RUnlock()

	if cache.GitStatus != nil && cache.GitStatus.FileMtime == mtime {
		return cache.GitStatus.Value, nil
	}

	// Cache miss - fetch and store
	value, err := fetchFn()
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check cache after acquiring write lock (TOCTOU protection)
	cache = m.load()
	if cache.GitStatus != nil && cache.GitStatus.FileMtime == mtime {
		return cache.GitStatus.Value, nil
	}

	cache.GitStatus = &CachedValue{
		Value:     value,
		FileMtime: mtime,
		CachedAt:  m.clock.Now(),
	}
	m.save(cache)

	return value, nil
}

// GetGitDiffStats returns the cached git diff stats or fetches them if the cache is invalid.
func (m *Manager) GetGitDiffStats(indexPath string, fetchFn func() (git.DiffStats, error)) (git.DiffStats, error) {
	// Get current file mtime
	mtime, err := getFileMtime(indexPath)
	if err != nil {
		// Can't stat file (maybe no commits yet), just fetch
		return fetchFn()
	}

	// Check cache
	m.mu.RLock()
	cache := m.load()
	m.mu.RUnlock()

	if cache.GitDiffStats != nil && cache.GitDiffStats.FileMtime == mtime {
		return cache.GitDiffStats.Stats, nil
	}

	// Cache miss - fetch and store
	stats, err := fetchFn()
	if err != nil {
		return git.DiffStats{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check cache after acquiring write lock (TOCTOU protection)
	cache = m.load()
	if cache.GitDiffStats != nil && cache.GitDiffStats.FileMtime == mtime {
		return cache.GitDiffStats.Stats, nil
	}

	cache.GitDiffStats = &CachedDiffStats{
		Stats:     stats,
		FileMtime: mtime,
		CachedAt:  m.clock.Now(),
	}
	m.save(cache)

	return stats, nil
}

// GetGitHubBuild returns the cached GitHub build status or fetches it if invalid.
// The cache is invalidated if either the ref mtime changes OR the TTL expires.
func (m *Manager) GetGitHubBuild(refPath, branch string, ttl time.Duration, fetchFn func() (github.BuildStatus, error)) (github.BuildStatus, error) {
	// Get current ref file mtime; fall back to packed-refs if branch ref file is packed.
	mtime, err := getFileMtime(refPath)
	if err != nil {
		if packedMtime, packedErr := getPackedRefsMtime(refPath); packedErr == nil {
			mtime = packedMtime
		} else {
			// Missing ref file entirely; use a sentinel so we still cache and rely on TTL.
			mtime = 0
		}
	}

	// Check cache
	m.mu.RLock()
	cache := m.load()
	m.mu.RUnlock()

	if cache.GitHubBuild != nil && cache.GitHubBuild.Branch == branch {
		refMtimeMatches := cache.GitHubBuild.FileMtime == mtime
		ttlValid := m.clock.Now().Sub(cache.GitHubBuild.CachedAt) < ttl

		if refMtimeMatches && ttlValid {
			return cache.GitHubBuild.Status, nil
		}
	}

	// Cache miss - fetch and store
	status, err := fetchFn()
	if err != nil {
		return github.StatusError, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check cache after acquiring write lock (TOCTOU protection)
	cache = m.load()
	if cache.GitHubBuild != nil && cache.GitHubBuild.Branch == branch {
		refMtimeMatches := cache.GitHubBuild.FileMtime == mtime
		ttlValid := m.clock.Now().Sub(cache.GitHubBuild.CachedAt) < ttl

		if refMtimeMatches && ttlValid {
			return cache.GitHubBuild.Status, nil
		}
	}

	cache.GitHubBuild = &CachedGitHubBuild{
		Status:    status,
		FileMtime: mtime,
		CachedAt:  m.clock.Now(),
		Branch:    branch,
	}
	m.save(cache)

	return status, nil
}

// load reads the cache file from disk.
func (m *Manager) load() *CacheFile {
	data, err := os.ReadFile(m.cachePath)
	if err != nil {
		return &CacheFile{}
	}

	var cache CacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		slog.Warn("cache file corrupted, resetting", "err", err)
		return &CacheFile{}
	}

	return &cache
}

// save writes the cache file to disk.
func (m *Manager) save(cache *CacheFile) {
	data, err := json.Marshal(cache)
	if err != nil {
		slog.Error("failed to marshal cache", "err", err)
		return
	}

	// Write atomically
	tmpPath := m.cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		slog.Error("failed to write cache temp file", "err", err)
		return
	}
	if err := os.Rename(tmpPath, m.cachePath); err != nil {
		slog.Error("failed to rename cache file", "err", err)
		os.Remove(tmpPath) // Clean up temp file
	}
}

// getFileMtime returns the modification time of a file in nanoseconds.
func getFileMtime(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.ModTime().UnixNano(), nil
}

// getPackedRefsMtime tries to read the packed-refs file mtime for repos where branch refs are packed.
func getPackedRefsMtime(refPath string) (int64, error) {
	// refPath is .../.git/refs/heads/<branch>; packed-refs lives under .git
	gitDir := filepath.Dir(filepath.Dir(filepath.Dir(refPath)))
	packedRefs := filepath.Join(gitDir, "packed-refs")
	return getFileMtime(packedRefs)
}

// Clear removes all cached data.
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return os.Remove(m.cachePath)
}
