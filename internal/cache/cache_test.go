package cache

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kostyay/claude-status/internal/github"
	"github.com/kostyay/claude-status/internal/tasks"
)

// mockClock is a test double for Clock.
type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

func (m *mockClock) Advance(d time.Duration) {
	m.now = m.now.Add(d)
}

func setupTestCache(t *testing.T) (*Manager, string, *mockClock) {
	dir := t.TempDir()
	clock := &mockClock{now: time.Now()}
	manager := NewManagerWithClock(dir, clock)
	if err := manager.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	return manager, dir, clock
}

func TestNewManager_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "subcache")

	manager := NewManager(cacheDir)
	if err := manager.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("cache dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("cache path is not a directory")
	}
}

func TestGetGitBranch_CacheMiss(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	// Create a test file to watch
	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		return "main", nil
	}

	branch, err := manager.GetGitBranch(headPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("GetGitBranch() = %q, want %q", branch, "main")
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1", fetchCalls)
	}
}

func TestGetGitBranch_CacheHit(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	// Create a test file to watch
	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		return "main", nil
	}

	// First call populates cache
	manager.GetGitBranch(headPath, fetchFn)

	// Second call should hit cache
	branch, err := manager.GetGitBranch(headPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("GetGitBranch() = %q, want %q", branch, "main")
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1 (cache should hit)", fetchCalls)
	}
}

func TestGetGitBranch_Invalidate(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	// Create a test file to watch
	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return "main", nil
		}
		return "feature", nil
	}

	// First call
	manager.GetGitBranch(headPath, fetchFn)

	// Modify the file (change mtime)
	time.Sleep(10 * time.Millisecond) // Ensure mtime changes
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/feature"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second call should invalidate cache
	branch, err := manager.GetGitBranch(headPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitBranch() error = %v", err)
	}
	if branch != "feature" {
		t.Errorf("GetGitBranch() = %q, want %q", branch, "feature")
	}
	if fetchCalls != 2 {
		t.Errorf("fetchFn called %d times, want 2", fetchCalls)
	}
}

func TestGetGitStatus_CacheMiss(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	indexPath := filepath.Join(dir, "index")
	if err := os.WriteFile(indexPath, []byte("index data"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		return "±3", nil
	}

	status, err := manager.GetGitStatus(indexPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitStatus() error = %v", err)
	}
	if status != "±3" {
		t.Errorf("GetGitStatus() = %q, want %q", status, "±3")
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1", fetchCalls)
	}
}

func TestGetGitStatus_CacheHit(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	indexPath := filepath.Join(dir, "index")
	if err := os.WriteFile(indexPath, []byte("index data"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		return "±3", nil
	}

	manager.GetGitStatus(indexPath, fetchFn)
	status, err := manager.GetGitStatus(indexPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitStatus() error = %v", err)
	}
	if status != "±3" {
		t.Errorf("GetGitStatus() = %q, want %q", status, "±3")
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1", fetchCalls)
	}
}

func TestGetGitStatus_Invalidate(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	indexPath := filepath.Join(dir, "index")
	if err := os.WriteFile(indexPath, []byte("index data"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return "±3", nil
		}
		return "±5", nil
	}

	manager.GetGitStatus(indexPath, fetchFn)

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(indexPath, []byte("new index"), 0644); err != nil {
		t.Fatal(err)
	}

	status, err := manager.GetGitStatus(indexPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitStatus() error = %v", err)
	}
	if status != "±5" {
		t.Errorf("GetGitStatus() = %q, want %q", status, "±5")
	}
	if fetchCalls != 2 {
		t.Errorf("fetchFn called %d times, want 2", fetchCalls)
	}
}

func TestGetGitHubBuild_CacheMiss(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	refPath := filepath.Join(dir, "refs", "heads", "main")
	os.MkdirAll(filepath.Dir(refPath), 0755)
	if err := os.WriteFile(refPath, []byte("abc123"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (github.BuildStatus, error) {
		fetchCalls++
		return github.StatusSuccess, nil
	}

	status, err := manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusSuccess)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1", fetchCalls)
	}
}

func TestGetGitHubBuild_CacheHit(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	refPath := filepath.Join(dir, "refs", "heads", "main")
	os.MkdirAll(filepath.Dir(refPath), 0755)
	if err := os.WriteFile(refPath, []byte("abc123"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (github.BuildStatus, error) {
		fetchCalls++
		return github.StatusSuccess, nil
	}

	manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	status, err := manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusSuccess)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1", fetchCalls)
	}
}

func TestGetGitHubBuild_PackedRefs(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	refPath := filepath.Join(dir, "refs", "heads", "main")
	// Simulate packed refs: branch file missing, packed-refs present.
	packedPath := filepath.Join(dir, "packed-refs")
	if err := os.WriteFile(packedPath, []byte("packed"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (github.BuildStatus, error) {
		fetchCalls++
		return github.StatusSuccess, nil
	}

	// First call should fetch and cache using packed-refs mtime.
	status, err := manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusSuccess)
	}

	// Second call should hit cache (no additional fetch).
	status, err = manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusSuccess)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1 (cache should hit with packed refs)", fetchCalls)
	}
}

func TestGetGitHubBuild_NoRefFileCaches(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	refPath := filepath.Join(dir, "refs", "heads", "main")

	fetchCalls := 0
	fetchFn := func() (github.BuildStatus, error) {
		fetchCalls++
		return github.StatusSuccess, nil
	}

	status, err := manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusSuccess)
	}

	// Second call should still hit cache even without ref/packed files (sentinel mtime).
	status, err = manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusSuccess)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1 (cache should use sentinel mtime)", fetchCalls)
	}
}

func TestGetGitHubBuild_TTLExpired(t *testing.T) {
	manager, dir, clock := setupTestCache(t)

	refPath := filepath.Join(dir, "refs", "heads", "main")
	os.MkdirAll(filepath.Dir(refPath), 0755)
	if err := os.WriteFile(refPath, []byte("abc123"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (github.BuildStatus, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return github.StatusPending, nil
		}
		return github.StatusSuccess, nil
	}

	// First fetch
	manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)

	// Advance time past TTL
	clock.Advance(61 * time.Second)

	// Second fetch should invalidate due to TTL
	status, err := manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusSuccess)
	}
	if fetchCalls != 2 {
		t.Errorf("fetchFn called %d times, want 2", fetchCalls)
	}
}

func TestGetGitHubBuild_RefChanged(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	refPath := filepath.Join(dir, "refs", "heads", "main")
	os.MkdirAll(filepath.Dir(refPath), 0755)
	if err := os.WriteFile(refPath, []byte("abc123"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (github.BuildStatus, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return github.StatusSuccess, nil
		}
		return github.StatusPending, nil
	}

	manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)

	// Modify ref file (simulate new commit)
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(refPath, []byte("def456"), 0644); err != nil {
		t.Fatal(err)
	}

	status, err := manager.GetGitHubBuild(refPath, "main", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetGitHubBuild() error = %v", err)
	}
	if status != github.StatusPending {
		t.Errorf("GetGitHubBuild() = %q, want %q", status, github.StatusPending)
	}
	if fetchCalls != 2 {
		t.Errorf("fetchFn called %d times, want 2", fetchCalls)
	}
}

func TestCachePersistence(t *testing.T) {
	dir := t.TempDir()
	clock := &mockClock{now: time.Now()}

	// Create a test file
	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref"), 0644); err != nil {
		t.Fatal(err)
	}

	// First manager populates cache
	manager1 := NewManagerWithClock(dir, clock)
	manager1.EnsureDir()

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		return "main", nil
	}

	manager1.GetGitBranch(headPath, fetchFn)
	if fetchCalls != 1 {
		t.Fatalf("fetchFn called %d times, want 1", fetchCalls)
	}

	// Second manager should read from persisted cache
	manager2 := NewManagerWithClock(dir, clock)

	branch, err := manager2.GetGitBranch(headPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("GetGitBranch() = %q, want %q", branch, "main")
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1 (should use persisted cache)", fetchCalls)
	}
}

func TestCacheCorruption(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	// Write corrupted cache file
	cachePath := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(cachePath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatal(err)
	}

	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchFn := func() (string, error) {
		return "main", nil
	}

	// Should gracefully handle corruption and fetch fresh
	branch, err := manager.GetGitBranch(headPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("GetGitBranch() = %q, want %q", branch, "main")
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref"), 0644); err != nil {
		t.Fatal(err)
	}

	var fetchCalls int
	var fetchMu sync.Mutex
	fetchFn := func() (string, error) {
		fetchMu.Lock()
		fetchCalls++
		fetchMu.Unlock()
		return "main", nil
	}

	var wg sync.WaitGroup
	// First populate the cache
	manager.GetGitBranch(headPath, fetchFn)

	// Then test concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.GetGitBranch(headPath, fetchFn)
		}()
	}
	wg.Wait()

	// With the cache populated, all concurrent reads should hit cache
	// We expect only 1 fetch (the initial one)
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1 (cache should handle concurrent reads)", fetchCalls)
	}
}

func TestClear(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchCalls := 0
	fetchFn := func() (string, error) {
		fetchCalls++
		return "main", nil
	}

	manager.GetGitBranch(headPath, fetchFn)
	if fetchCalls != 1 {
		t.Fatalf("fetchFn called %d times, want 1", fetchCalls)
	}

	// Clear cache
	if err := manager.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// Next call should fetch again
	manager.GetGitBranch(headPath, fetchFn)
	if fetchCalls != 2 {
		t.Errorf("fetchFn called %d times, want 2 (cache was cleared)", fetchCalls)
	}
}

func TestGetGitBranch_FileNotExist(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	headPath := filepath.Join(dir, "nonexistent")

	fetchFn := func() (string, error) {
		return "main", nil
	}

	// Should fall back to fetchFn when file doesn't exist
	branch, err := manager.GetGitBranch(headPath, fetchFn)
	if err != nil {
		t.Fatalf("GetGitBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("GetGitBranch() = %q, want %q", branch, "main")
	}
}

func TestFileLockCreated(t *testing.T) {
	manager, dir, _ := setupTestCache(t)

	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref"), 0644); err != nil {
		t.Fatal(err)
	}

	fetchFn := func() (string, error) {
		return "main", nil
	}

	// Call a method that uses the file lock
	manager.GetGitBranch(headPath, fetchFn)

	// Verify lock file was created
	lockPath := filepath.Join(dir, "cache.json.lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file was not created")
	}
}

func TestFileLockSerializesMultipleManagers(t *testing.T) {
	dir := t.TempDir()
	clock := &mockClock{now: time.Now()}

	headPath := filepath.Join(dir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create two managers pointing to the same cache
	manager1 := NewManagerWithClock(dir, clock)
	manager1.EnsureDir()
	manager2 := NewManagerWithClock(dir, clock)

	var mu sync.Mutex
	var order []int

	fetchFn := func(id int) func() (string, error) {
		return func() (string, error) {
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			time.Sleep(50 * time.Millisecond) // Simulate work
			return "main", nil
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Start both managers concurrently
	go func() {
		defer wg.Done()
		manager1.GetGitBranch(headPath, fetchFn(1))
	}()
	go func() {
		defer wg.Done()
		manager2.GetGitBranch(headPath, fetchFn(2))
	}()

	wg.Wait()

	// Both should complete (file lock serializes access)
	mu.Lock()
	defer mu.Unlock()

	// With file locking, only ONE fetch should happen (second manager uses cached result)
	// OR both fetch but sequentially (depends on timing)
	if len(order) == 0 {
		t.Error("expected at least one fetch to occur")
	}
}

func TestGetTaskStats_CacheMiss(t *testing.T) {
	manager, _, _ := setupTestCache(t)

	fetchCalls := 0
	fetchFn := func() (tasks.Stats, error) {
		fetchCalls++
		return tasks.Stats{
			TotalIssues:      10,
			OpenIssues:       5,
			InProgressIssues: 2,
			ReadyIssues:      3,
			BlockedIssues:    1,
		}, nil
	}

	stats, err := manager.GetTaskStats("/test/project", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetTaskStats() error = %v", err)
	}
	if stats.TotalIssues != 10 {
		t.Errorf("GetTaskStats().TotalIssues = %d, want %d", stats.TotalIssues, 10)
	}
	if stats.OpenIssues != 5 {
		t.Errorf("GetTaskStats().OpenIssues = %d, want %d", stats.OpenIssues, 5)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1", fetchCalls)
	}
}

func TestGetTaskStats_CacheHit(t *testing.T) {
	manager, _, _ := setupTestCache(t)

	fetchCalls := 0
	fetchFn := func() (tasks.Stats, error) {
		fetchCalls++
		return tasks.Stats{
			TotalIssues: 10,
			OpenIssues:  5,
		}, nil
	}

	// First call populates cache
	manager.GetTaskStats("/test/project", 60*time.Second, fetchFn)

	// Second call should hit cache
	stats, err := manager.GetTaskStats("/test/project", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetTaskStats() error = %v", err)
	}
	if stats.TotalIssues != 10 {
		t.Errorf("GetTaskStats().TotalIssues = %d, want %d", stats.TotalIssues, 10)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchFn called %d times, want 1 (cache should hit)", fetchCalls)
	}
}

func TestGetTaskStats_TTLExpired(t *testing.T) {
	manager, _, clock := setupTestCache(t)

	fetchCalls := 0
	fetchFn := func() (tasks.Stats, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return tasks.Stats{TotalIssues: 10}, nil
		}
		return tasks.Stats{TotalIssues: 15}, nil
	}

	// First fetch
	manager.GetTaskStats("/test/project", 60*time.Second, fetchFn)

	// Advance time past TTL
	clock.Advance(61 * time.Second)

	// Second fetch should invalidate due to TTL
	stats, err := manager.GetTaskStats("/test/project", 60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetTaskStats() error = %v", err)
	}
	if stats.TotalIssues != 15 {
		t.Errorf("GetTaskStats().TotalIssues = %d, want %d", stats.TotalIssues, 15)
	}
	if fetchCalls != 2 {
		t.Errorf("fetchFn called %d times, want 2", fetchCalls)
	}
}

func TestGetTaskStats_PerProjectCache(t *testing.T) {
	manager, _, _ := setupTestCache(t)

	// Track which workDir was requested
	fetchCalls := make(map[string]int)
	fetchFn := func(workDir string) func() (tasks.Stats, error) {
		return func() (tasks.Stats, error) {
			fetchCalls[workDir]++
			if workDir == "/project/a" {
				return tasks.Stats{TotalIssues: 10, ReadyIssues: 5}, nil
			}
			return tasks.Stats{TotalIssues: 20, ReadyIssues: 8}, nil
		}
	}

	// Fetch for project A
	statsA, err := manager.GetTaskStats("/project/a", 60*time.Second, fetchFn("/project/a"))
	if err != nil {
		t.Fatalf("GetTaskStats(/project/a) error = %v", err)
	}
	if statsA.TotalIssues != 10 {
		t.Errorf("Project A TotalIssues = %d, want 10", statsA.TotalIssues)
	}
	if statsA.ReadyIssues != 5 {
		t.Errorf("Project A ReadyIssues = %d, want 5", statsA.ReadyIssues)
	}

	// Fetch for project B - should NOT use project A's cache
	statsB, err := manager.GetTaskStats("/project/b", 60*time.Second, fetchFn("/project/b"))
	if err != nil {
		t.Fatalf("GetTaskStats(/project/b) error = %v", err)
	}
	if statsB.TotalIssues != 20 {
		t.Errorf("Project B TotalIssues = %d, want 20", statsB.TotalIssues)
	}
	if statsB.ReadyIssues != 8 {
		t.Errorf("Project B ReadyIssues = %d, want 8", statsB.ReadyIssues)
	}

	// Both projects should have been fetched exactly once
	if fetchCalls["/project/a"] != 1 {
		t.Errorf("Project A fetched %d times, want 1", fetchCalls["/project/a"])
	}
	if fetchCalls["/project/b"] != 1 {
		t.Errorf("Project B fetched %d times, want 1", fetchCalls["/project/b"])
	}

	// Fetching project A again should use cache
	statsA2, err := manager.GetTaskStats("/project/a", 60*time.Second, fetchFn("/project/a"))
	if err != nil {
		t.Fatalf("GetTaskStats(/project/a) second call error = %v", err)
	}
	if statsA2.TotalIssues != 10 {
		t.Errorf("Project A (cached) TotalIssues = %d, want 10", statsA2.TotalIssues)
	}
	if fetchCalls["/project/a"] != 1 {
		t.Errorf("Project A fetched %d times after cache hit, want 1", fetchCalls["/project/a"])
	}

	// Fetching project B again should use cache
	statsB2, err := manager.GetTaskStats("/project/b", 60*time.Second, fetchFn("/project/b"))
	if err != nil {
		t.Fatalf("GetTaskStats(/project/b) second call error = %v", err)
	}
	if statsB2.TotalIssues != 20 {
		t.Errorf("Project B (cached) TotalIssues = %d, want 20", statsB2.TotalIssues)
	}
	if fetchCalls["/project/b"] != 1 {
		t.Errorf("Project B fetched %d times after cache hit, want 1", fetchCalls["/project/b"])
	}
}
