package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/kostyay/claude-status/internal/git"
	"github.com/kostyay/claude-status/internal/github"
	"github.com/kostyay/claude-status/internal/tasks"
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

// CachedTaskStats holds cached task statistics.
type CachedTaskStats struct {
	Stats    tasks.Stats `json:"stats"`
	CachedAt time.Time   `json:"cached_at"`
}

// CachedNextTask holds cached next task title.
type CachedNextTask struct {
	Title    string    `json:"title"`
	CachedAt time.Time `json:"cached_at"`
}

// CacheFile is the structure of the cache file on disk.
type CacheFile struct {
	GitBranch    *CachedValue                `json:"git_branch,omitempty"`
	GitStatus    *CachedValue                `json:"git_status,omitempty"`
	GitDiffStats *CachedDiffStats            `json:"git_diff_stats,omitempty"`
	GitHubBuild  *CachedGitHubBuild          `json:"github_build,omitempty"`
	TaskStatsMap map[string]*CachedTaskStats `json:"task_stats_map,omitempty"` // keyed by workDir
	NextTaskMap  map[string]*CachedNextTask  `json:"next_task_map,omitempty"`  // keyed by workDir
}

// Manager handles cache operations with file-based persistence.
type Manager struct {
	cacheDir    string
	cachePath   string
	clock       Clock
	mu          sync.RWMutex
	fileLock    *flock.Flock
	memCache    *CacheFile // In-memory cache to reduce disk I/O
	cacheLoaded bool       // Whether memCache is populated
}

// NewManager creates a new cache manager.
func NewManager(cacheDir string) *Manager {
	return NewManagerWithClock(cacheDir, RealClock{})
}

// NewManagerWithClock creates a new cache manager with a custom clock.
func NewManagerWithClock(cacheDir string, clock Clock) *Manager {
	cachePath := filepath.Join(cacheDir, "cache.json")
	return &Manager{
		cacheDir:  cacheDir,
		cachePath: cachePath,
		clock:     clock,
		fileLock:  flock.New(cachePath + ".lock"),
	}
}

// EnsureDir creates the cache directory if it doesn't exist.
func (m *Manager) EnsureDir() error {
	return os.MkdirAll(m.cacheDir, 0755)
}

// withFileLock acquires an exclusive file lock before executing fn.
// This ensures multi-process safety when multiple instances access the same cache.
// On lock timeout, it proceeds without locking (graceful degradation).
func (m *Manager) withFileLock(fn func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	locked, err := m.fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil || !locked {
		slog.Warn("cache lock timeout, proceeding without lock", "err", err)
		fn()
		return
	}
	defer func() {
		_ = m.fileLock.Unlock()
	}()

	fn()
}

// GetGitBranch returns the cached git branch or fetches it if the cache is invalid.
func (m *Manager) GetGitBranch(headPath string, fetchFn func() (string, error)) (string, error) {
	var result string
	var resultErr error

	m.withFileLock(func() {
		// Get current file mtime
		mtime, err := getFileMtime(headPath)
		if err != nil {
			// Can't stat file, just fetch
			result, resultErr = fetchFn()
			return
		}

		// Check cache
		m.mu.RLock()
		cache := m.load()
		m.mu.RUnlock()

		if cache.GitBranch != nil && cache.GitBranch.FileMtime == mtime {
			result = cache.GitBranch.Value
			return
		}

		// Cache miss - fetch and store
		value, err := fetchFn()
		if err != nil {
			resultErr = err
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Re-check cache after acquiring write lock (TOCTOU protection)
		cache = m.load()
		if cache.GitBranch != nil && cache.GitBranch.FileMtime == mtime {
			result = cache.GitBranch.Value
			return
		}

		cache.GitBranch = &CachedValue{
			Value:     value,
			FileMtime: mtime,
			CachedAt:  m.clock.Now(),
		}
		m.save(cache)

		result = value
	})

	return result, resultErr
}

// GetGitStatus returns the cached git status or fetches it if the cache is invalid.
func (m *Manager) GetGitStatus(indexPath string, fetchFn func() (string, error)) (string, error) {
	var result string
	var resultErr error

	m.withFileLock(func() {
		// Get current file mtime
		mtime, err := getFileMtime(indexPath)
		if err != nil {
			// Can't stat file (maybe no commits yet), just fetch
			result, resultErr = fetchFn()
			return
		}

		// Check cache
		m.mu.RLock()
		cache := m.load()
		m.mu.RUnlock()

		if cache.GitStatus != nil && cache.GitStatus.FileMtime == mtime {
			result = cache.GitStatus.Value
			return
		}

		// Cache miss - fetch and store
		value, err := fetchFn()
		if err != nil {
			resultErr = err
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Re-check cache after acquiring write lock (TOCTOU protection)
		cache = m.load()
		if cache.GitStatus != nil && cache.GitStatus.FileMtime == mtime {
			result = cache.GitStatus.Value
			return
		}

		cache.GitStatus = &CachedValue{
			Value:     value,
			FileMtime: mtime,
			CachedAt:  m.clock.Now(),
		}
		m.save(cache)

		result = value
	})

	return result, resultErr
}

// GetGitDiffStats returns the cached git diff stats or fetches them if the cache is invalid.
func (m *Manager) GetGitDiffStats(indexPath string, fetchFn func() (git.DiffStats, error)) (git.DiffStats, error) {
	var result git.DiffStats
	var resultErr error

	m.withFileLock(func() {
		// Get current file mtime
		mtime, err := getFileMtime(indexPath)
		if err != nil {
			// Can't stat file (maybe no commits yet), just fetch
			result, resultErr = fetchFn()
			return
		}

		// Check cache
		m.mu.RLock()
		cache := m.load()
		m.mu.RUnlock()

		if cache.GitDiffStats != nil && cache.GitDiffStats.FileMtime == mtime {
			result = cache.GitDiffStats.Stats
			return
		}

		// Cache miss - fetch and store
		stats, err := fetchFn()
		if err != nil {
			resultErr = err
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Re-check cache after acquiring write lock (TOCTOU protection)
		cache = m.load()
		if cache.GitDiffStats != nil && cache.GitDiffStats.FileMtime == mtime {
			result = cache.GitDiffStats.Stats
			return
		}

		cache.GitDiffStats = &CachedDiffStats{
			Stats:     stats,
			FileMtime: mtime,
			CachedAt:  m.clock.Now(),
		}
		m.save(cache)

		result = stats
	})

	return result, resultErr
}

// GetGitHubBuild returns the cached GitHub build status or fetches it if invalid.
// The cache is invalidated if either the ref mtime changes OR the TTL expires.
func (m *Manager) GetGitHubBuild(refPath, branch string, ttl time.Duration, fetchFn func() (github.BuildStatus, error)) (github.BuildStatus, error) {
	var result github.BuildStatus
	var resultErr error

	m.withFileLock(func() {
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
				result = cache.GitHubBuild.Status
				return
			}
		}

		// Cache miss - fetch and store
		status, err := fetchFn()
		if err != nil {
			result = github.StatusError
			resultErr = err
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Re-check cache after acquiring write lock (TOCTOU protection)
		cache = m.load()
		if cache.GitHubBuild != nil && cache.GitHubBuild.Branch == branch {
			refMtimeMatches := cache.GitHubBuild.FileMtime == mtime
			ttlValid := m.clock.Now().Sub(cache.GitHubBuild.CachedAt) < ttl

			if refMtimeMatches && ttlValid {
				result = cache.GitHubBuild.Status
				return
			}
		}

		cache.GitHubBuild = &CachedGitHubBuild{
			Status:    status,
			FileMtime: mtime,
			CachedAt:  m.clock.Now(),
			Branch:    branch,
		}
		m.save(cache)

		result = status
	})

	return result, resultErr
}

// GetTaskStats returns cached task stats or fetches them if the cache is invalid.
// The cache is invalidated when the TTL expires. Stats are cached per workDir.
func (m *Manager) GetTaskStats(workDir string, ttl time.Duration, fetchFn func() (tasks.Stats, error)) (tasks.Stats, error) {
	var result tasks.Stats
	var resultErr error

	m.withFileLock(func() {
		// Check cache
		m.mu.RLock()
		cache := m.load()
		m.mu.RUnlock()

		if cache.TaskStatsMap != nil {
			if cached, ok := cache.TaskStatsMap[workDir]; ok {
				ttlValid := m.clock.Now().Sub(cached.CachedAt) < ttl
				if ttlValid {
					result = cached.Stats
					return
				}
			}
		}

		// Cache miss - fetch and store
		stats, err := fetchFn()
		if err != nil {
			resultErr = err
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Re-check cache after acquiring write lock (TOCTOU protection)
		cache = m.load()
		if cache.TaskStatsMap != nil {
			if cached, ok := cache.TaskStatsMap[workDir]; ok {
				ttlValid := m.clock.Now().Sub(cached.CachedAt) < ttl
				if ttlValid {
					result = cached.Stats
					return
				}
			}
		}

		if cache.TaskStatsMap == nil {
			cache.TaskStatsMap = make(map[string]*CachedTaskStats)
		}
		cache.TaskStatsMap[workDir] = &CachedTaskStats{
			Stats:    stats,
			CachedAt: m.clock.Now(),
		}
		m.save(cache)

		result = stats
	})

	return result, resultErr
}

// GetNextTask returns cached next task or fetches it if the cache is invalid.
// The cache is invalidated when the TTL expires. Tasks are cached per workDir.
func (m *Manager) GetNextTask(workDir string, ttl time.Duration, fetchFn func() (string, error)) (string, error) {
	var result string
	var resultErr error

	m.withFileLock(func() {
		// Check cache
		m.mu.RLock()
		cache := m.load()
		m.mu.RUnlock()

		if cache.NextTaskMap != nil {
			if cached, ok := cache.NextTaskMap[workDir]; ok {
				ttlValid := m.clock.Now().Sub(cached.CachedAt) < ttl
				if ttlValid {
					result = cached.Title
					return
				}
			}
		}

		// Cache miss - fetch and store
		title, err := fetchFn()
		if err != nil {
			resultErr = err
			return
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		// Re-check cache after acquiring write lock (TOCTOU protection)
		cache = m.load()
		if cache.NextTaskMap != nil {
			if cached, ok := cache.NextTaskMap[workDir]; ok {
				ttlValid := m.clock.Now().Sub(cached.CachedAt) < ttl
				if ttlValid {
					result = cached.Title
					return
				}
			}
		}

		if cache.NextTaskMap == nil {
			cache.NextTaskMap = make(map[string]*CachedNextTask)
		}
		cache.NextTaskMap[workDir] = &CachedNextTask{
			Title:    title,
			CachedAt: m.clock.Now(),
		}
		m.save(cache)

		result = title
	})

	return result, resultErr
}

// load reads the cache file from disk or returns the in-memory cache.
func (m *Manager) load() *CacheFile {
	// Return in-memory cache if already loaded
	if m.cacheLoaded && m.memCache != nil {
		return m.memCache
	}

	data, err := os.ReadFile(m.cachePath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("failed to read cache file", "path", m.cachePath, "err", err)
		}
		cache := &CacheFile{}
		m.memCache = cache
		m.cacheLoaded = true
		return cache
	}

	var cache CacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		slog.Warn("cache file corrupted, resetting", "err", err)
		cache := &CacheFile{}
		m.memCache = cache
		m.cacheLoaded = true
		return cache
	}

	m.memCache = &cache
	m.cacheLoaded = true
	return &cache
}

// maxCacheAge is the maximum age for cached beads stats entries before eviction.
const maxCacheAge = 7 * 24 * time.Hour // 1 week

// save writes the cache file to disk and updates the in-memory cache.
func (m *Manager) save(cache *CacheFile) {
	// Evict old entries before saving
	m.cleanupOldEntries(cache, maxCacheAge)

	// Update in-memory cache
	m.memCache = cache
	m.cacheLoaded = true

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

// cleanupOldEntries removes entries older than maxAge from TaskStatsMap and NextTaskMap.
func (m *Manager) cleanupOldEntries(cache *CacheFile, maxAge time.Duration) {
	now := m.clock.Now()

	// Clean up old TaskStatsMap entries
	if cache.TaskStatsMap != nil {
		for key, entry := range cache.TaskStatsMap {
			if now.Sub(entry.CachedAt) > maxAge {
				delete(cache.TaskStatsMap, key)
			}
		}
	}

	// Clean up old NextTaskMap entries
	if cache.NextTaskMap != nil {
		for key, entry := range cache.NextTaskMap {
			if now.Sub(entry.CachedAt) > maxAge {
				delete(cache.NextTaskMap, key)
			}
		}
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
	// Invalidate in-memory cache
	m.memCache = nil
	m.cacheLoaded = false
	return os.Remove(m.cachePath)
}
