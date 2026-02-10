package status

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kostyay/claude-status/internal/config"
	"github.com/kostyay/claude-status/internal/git"
	"github.com/kostyay/claude-status/internal/github"
	"github.com/kostyay/claude-status/internal/tasks"
)

// mockGitProvider is a test double for GitProvider.
type mockGitProvider struct {
	branch       string
	branchErr    error
	status       string
	statusErr    error
	diffStats    git.DiffStats
	diffStatsErr error
	remoteURL    string
	remoteErr    error
	gitDir       string
}

func (m *mockGitProvider) Branch() (string, error)           { return m.branch, m.branchErr }
func (m *mockGitProvider) Status() (string, error)           { return m.status, m.statusErr }
func (m *mockGitProvider) DiffStats() (git.DiffStats, error) { return m.diffStats, m.diffStatsErr }
func (m *mockGitProvider) RemoteURL() (string, error)        { return m.remoteURL, m.remoteErr }
func (m *mockGitProvider) GitDir() string                    { return m.gitDir }
func (m *mockGitProvider) HeadPath() string                  { return m.gitDir + "/HEAD" }
func (m *mockGitProvider) IndexPath() string                 { return m.gitDir + "/index" }
func (m *mockGitProvider) RefPath(branch string) string {
	return m.gitDir + "/refs/heads/" + branch
}

// mockGitHubProvider is a test double for GitHubProvider.
type mockGitHubProvider struct {
	status github.BuildStatus
	err    error
}

func (m *mockGitHubProvider) GetBuildStatus(owner, repo, branch string) (github.BuildStatus, error) {
	return m.status, m.err
}

// mockCacheProvider is a test double for CacheProvider.
type mockCacheProvider struct {
	branchValue    string
	statusValue    string
	diffStatsValue git.DiffStats
	buildStatus    github.BuildStatus
	buildErr       error
	taskStats      tasks.Stats
	fetchBranch    bool
	fetchStatus    bool
	fetchDiffStats bool
	fetchBuild     bool
	fetchTasks     bool
}

func (m *mockCacheProvider) EnsureDir() error { return nil }

func (m *mockCacheProvider) GetGitBranch(headPath string, fetchFn func() (string, error)) (string, error) {
	if m.fetchBranch {
		return fetchFn()
	}
	return m.branchValue, nil
}

func (m *mockCacheProvider) GetGitStatus(indexPath string, fetchFn func() (string, error)) (string, error) {
	if m.fetchStatus {
		return fetchFn()
	}
	return m.statusValue, nil
}

func (m *mockCacheProvider) GetGitDiffStats(indexPath string, fetchFn func() (git.DiffStats, error)) (git.DiffStats, error) {
	if m.fetchDiffStats {
		return fetchFn()
	}
	return m.diffStatsValue, nil
}

func (m *mockCacheProvider) GetGitHubBuild(refPath, branch string, ttl time.Duration, fetchFn func() (github.BuildStatus, error)) (github.BuildStatus, error) {
	if m.fetchBuild {
		return fetchFn()
	}
	return m.buildStatus, m.buildErr
}

func (m *mockCacheProvider) GetTaskStats(workDir string, ttl time.Duration, fetchFn func() (tasks.Stats, error)) (tasks.Stats, error) {
	if m.fetchTasks {
		return fetchFn()
	}
	return m.taskStats, nil
}

func (m *mockCacheProvider) GetNextTask(workDir string, ttl time.Duration, fetchFn func() (string, error)) (string, error) {
	return fetchFn()
}

// mockTaskProvider is a test double for tasks.Provider.
type mockTaskProvider struct {
	name      string
	available bool
	stats     tasks.Stats
	err       error
	nextTask  string
}

func (m *mockTaskProvider) Name() string {
	return m.name
}

func (m *mockTaskProvider) Available() bool {
	return m.available
}

func (m *mockTaskProvider) GetStats() (tasks.Stats, error) {
	return m.stats, m.err
}

func (m *mockTaskProvider) GetNextTask() (string, error) {
	return m.nextTask, nil
}

func TestBuild_AllData(t *testing.T) {
	cfg := config.Default()

	git := &mockGitProvider{
		branch:    "main",
		status:    "±3",
		remoteURL: "git@github.com:owner/repo.git",
		gitDir:    "/repo/.git",
	}

	gh := &mockGitHubProvider{status: github.StatusSuccess}

	cache := &mockCacheProvider{
		branchValue: "main",
		statusValue: "±3",
		buildStatus: github.StatusSuccess,
	}

	builder := NewBuilderWithDeps(&cfg, cache, git, gh, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/path/to/myproject"},
		Version:   "1.0.0",
	}

	data := builder.Build(input)

	if data.Model != "Claude" {
		t.Errorf("Model = %q, want %q", data.Model, "Claude")
	}
	if data.Dir != "myproject" {
		t.Errorf("Dir = %q, want %q", data.Dir, "myproject")
	}
	if data.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", data.GitBranch, "main")
	}
	if data.GitStatus != "±3" {
		t.Errorf("GitStatus = %q, want %q", data.GitStatus, "±3")
	}
	if data.GitHubStatus != "✅" {
		t.Errorf("GitHubStatus = %q, want %q", data.GitHubStatus, "✅")
	}
	if data.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", data.Version, "1.0.0")
	}
}

func TestBuild_NoGit(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	// nil git provider simulates not being in a git repo
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/path/to/myproject"},
		Version:   "1.0.0",
	}

	data := builder.Build(input)

	if data.Model != "Claude" {
		t.Errorf("Model = %q, want %q", data.Model, "Claude")
	}
	if data.Dir != "myproject" {
		t.Errorf("Dir = %q, want %q", data.Dir, "myproject")
	}
	if data.GitBranch != "" {
		t.Errorf("GitBranch = %q, want empty", data.GitBranch)
	}
	if data.GitStatus != "" {
		t.Errorf("GitStatus = %q, want empty", data.GitStatus)
	}
	if data.GitHubStatus != "" {
		t.Errorf("GitHubStatus = %q, want empty", data.GitHubStatus)
	}
}

func TestBuild_GitNoGitHub(t *testing.T) {
	cfg := config.Default()

	git := &mockGitProvider{
		branch:    "main",
		status:    "±3",
		remoteURL: "git@gitlab.com:owner/repo.git", // Not GitHub
		gitDir:    "/repo/.git",
	}

	cache := &mockCacheProvider{
		branchValue: "main",
		statusValue: "±3",
	}

	builder := NewBuilderWithDeps(&cfg, cache, git, nil, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/path/to/myproject"},
	}

	data := builder.Build(input)

	if data.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", data.GitBranch, "main")
	}
	if data.GitStatus != "±3" {
		t.Errorf("GitStatus = %q, want %q", data.GitStatus, "±3")
	}
	if data.GitHubStatus != "" {
		t.Errorf("GitHubStatus = %q, want empty (not GitHub)", data.GitHubStatus)
	}
}

func TestBuild_GitHubFailure(t *testing.T) {
	cfg := config.Default()

	git := &mockGitProvider{
		branch:    "main",
		status:    "",
		remoteURL: "git@github.com:owner/repo.git",
		gitDir:    "/repo/.git",
	}

	gh := &mockGitHubProvider{err: errors.New("API error")}

	cache := &mockCacheProvider{
		branchValue: "main",
		fetchBuild:  true, // Actually call the fetch function
	}

	builder := NewBuilderWithDeps(&cfg, cache, git, gh, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/path/to/myproject"},
	}

	data := builder.Build(input)

	// Should still have git data
	if data.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", data.GitBranch, "main")
	}
	// GitHub status should be empty (silent fail)
	if data.GitHubStatus != "" {
		t.Errorf("GitHubStatus = %q, want empty (should silent fail)", data.GitHubStatus)
	}
}

func TestBuild_CacheHit(t *testing.T) {
	cfg := config.Default()

	git := &mockGitProvider{
		branch:    "should-not-be-called",
		status:    "should-not-be-called",
		remoteURL: "git@github.com:owner/repo.git",
		gitDir:    "/repo/.git",
	}

	gh := &mockGitHubProvider{status: github.StatusFailure}

	// Cache returns values without calling fetch
	cache := &mockCacheProvider{
		branchValue: "cached-branch",
		statusValue: "±cached",
		buildStatus: github.StatusSuccess,
		fetchBranch: false,
		fetchStatus: false,
		fetchBuild:  false,
	}

	builder := NewBuilderWithDeps(&cfg, cache, git, gh, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	if data.GitBranch != "cached-branch" {
		t.Errorf("GitBranch = %q, want %q (from cache)", data.GitBranch, "cached-branch")
	}
	if data.GitStatus != "±cached" {
		t.Errorf("GitStatus = %q, want %q (from cache)", data.GitStatus, "±cached")
	}
	if data.GitHubStatus != "✅" {
		t.Errorf("GitHubStatus = %q, want %q (from cache)", data.GitHubStatus, "✅")
	}
}

func TestBuild_CacheMiss(t *testing.T) {
	cfg := config.Default()

	git := &mockGitProvider{
		branch:    "fresh-branch",
		status:    "±fresh",
		remoteURL: "git@github.com:owner/repo.git",
		gitDir:    "/repo/.git",
	}

	gh := &mockGitHubProvider{status: github.StatusPending}

	// Cache calls fetch functions
	cache := &mockCacheProvider{
		fetchBranch: true,
		fetchStatus: true,
		fetchBuild:  true,
	}

	builder := NewBuilderWithDeps(&cfg, cache, git, gh, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	if data.GitBranch != "fresh-branch" {
		t.Errorf("GitBranch = %q, want %q (fresh fetch)", data.GitBranch, "fresh-branch")
	}
	if data.GitStatus != "±fresh" {
		t.Errorf("GitStatus = %q, want %q (fresh fetch)", data.GitStatus, "±fresh")
	}
	if data.GitHubStatus != "🔄" {
		t.Errorf("GitHubStatus = %q, want %q (pending)", data.GitHubStatus, "🔄")
	}
}

func TestBuild_PartialFailure(t *testing.T) {
	cfg := config.Default()

	git := &mockGitProvider{
		branch:    "main",
		branchErr: nil,
		status:    "",
		statusErr: errors.New("git status failed"),
		remoteURL: "",
		remoteErr: errors.New("no remote"),
		gitDir:    "/repo/.git",
	}

	cache := &mockCacheProvider{
		fetchBranch: true,
		fetchStatus: true,
	}

	builder := NewBuilderWithDeps(&cfg, cache, git, nil, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	// Should still have branch
	if data.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", data.GitBranch, "main")
	}
	// Status failed, should be empty
	if data.GitStatus != "" {
		t.Errorf("GitStatus = %q, want empty (failed)", data.GitStatus)
	}
	// GitHub status should be empty (no remote)
	if data.GitHubStatus != "" {
		t.Errorf("GitHubStatus = %q, want empty (no remote)", data.GitHubStatus)
	}
}

func TestBuild_DefaultModel(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: ""}, // Empty
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	if data.Model != "Claude" {
		t.Errorf("Model = %q, want %q (default)", data.Model, "Claude")
	}
}

func TestBuild_DirBasename(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/very/long/path/to/myproject"},
	}

	data := builder.Build(input)

	if data.Dir != "myproject" {
		t.Errorf("Dir = %q, want %q (basename only)", data.Dir, "myproject")
	}
}

func TestNewBuilder_NilConfig(t *testing.T) {
	_, err := NewBuilder(nil, "/tmp", "")
	if err == nil {
		t.Error("NewBuilder() expected error for nil config")
	}
	if err != ErrNilConfig {
		t.Errorf("NewBuilder() error = %v, want %v", err, ErrNilConfig)
	}
}

func TestBuild_TokenMetrics(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	// Create a temporary transcript file
	tmpDir := t.TempDir()
	transcriptPath := tmpDir + "/transcript.jsonl"

	jsonlContent := `{"type":"summary","summary":"Test session"}
{"parentUuid":"123","isSidechain":false,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":10000,"output_tokens":5000,"cache_read_input_tokens":30000,"cache_creation_input_tokens":5000}}}
`
	if err := writeTestFile(transcriptPath, jsonlContent); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	input := Input{
		Model:          ModelInfo{ID: "claude-opus-4-5-20251101", DisplayName: "Claude"},
		Workspace:      WorkspaceInfo{CurrentDir: "/project"},
		TranscriptPath: transcriptPath,
	}

	data := builder.Build(input)

	// Check token metrics are populated (raw values)
	if data.TokensInput != 10000 {
		t.Errorf("TokensInput = %d, want %d", data.TokensInput, 10000)
	}
	if data.TokensOutput != 5000 {
		t.Errorf("TokensOutput = %d, want %d", data.TokensOutput, 5000)
	}
	if data.TokensCached != 35000 {
		t.Errorf("TokensCached = %d, want %d", data.TokensCached, 35000)
	}
	if data.TokensTotal != 50000 {
		t.Errorf("TokensTotal = %d, want %d", data.TokensTotal, 50000)
	}

	// Check context percentage is calculated
	if data.ContextPct == 0 {
		t.Error("ContextPct should not be zero")
	}
}

func TestBuild_TokenMetrics_EmptyPath(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	input := Input{
		Model:          ModelInfo{DisplayName: "Claude"},
		Workspace:      WorkspaceInfo{CurrentDir: "/project"},
		TranscriptPath: "", // Empty path
	}

	data := builder.Build(input)

	// Token metrics should be zero
	if data.TokensInput != 0 {
		t.Errorf("TokensInput = %d, want 0", data.TokensInput)
	}
}

func TestBuild_TokenMetrics_InvalidPath(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	input := Input{
		Model:          ModelInfo{DisplayName: "Claude"},
		Workspace:      WorkspaceInfo{CurrentDir: "/project"},
		TranscriptPath: "/nonexistent/path/transcript.jsonl",
	}

	data := builder.Build(input)

	// Token metrics should be zero (silent fail)
	if data.TokensInput != 0 {
		t.Errorf("TokensInput = %d, want 0", data.TokensInput)
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func TestBuild_TaskStats(t *testing.T) {
	cfg := config.Default()

	taskProvider := &mockTaskProvider{
		name:      "test",
		available: true,
		stats: tasks.Stats{
			TotalIssues:      10,
			OpenIssues:       5,
			InProgressIssues: 2,
			ReadyIssues:      3,
			BlockedIssues:    1,
		},
	}

	cache := &mockCacheProvider{
		fetchTasks: true,
	}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, taskProvider, "/project")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	// Check task stats are populated (raw values)
	if !data.HasTasks {
		t.Error("HasTasks should be true")
	}
	if data.TasksTotal != 10 {
		t.Errorf("TasksTotal = %d, want %d", data.TasksTotal, 10)
	}
	if data.TasksOpen != 5 {
		t.Errorf("TasksOpen = %d, want %d", data.TasksOpen, 5)
	}
	if data.TasksReady != 3 {
		t.Errorf("TasksReady = %d, want %d", data.TasksReady, 3)
	}
	if data.TasksInProgress != 2 {
		t.Errorf("TasksInProgress = %d, want %d", data.TasksInProgress, 2)
	}
	if data.TasksBlocked != 1 {
		t.Errorf("TasksBlocked = %d, want %d", data.TasksBlocked, 1)
	}
}

func TestBuild_NoTasks(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	// nil task provider simulates no task system available
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	if data.HasTasks {
		t.Error("HasTasks should be false when task provider is nil")
	}
	if data.TasksOpen != 0 {
		t.Errorf("TasksOpen = %d, want 0", data.TasksOpen)
	}
}

func TestSetPrefix_Simple(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	builder.SetPrefix("WORK")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	// Prefix is stored as plain string (color applied by template)
	if data.Prefix != "WORK" {
		t.Errorf("Prefix = %q, want %q", data.Prefix, "WORK")
	}
}

func TestSetPrefix_SpecialChars(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	// Prefix is now stored as plain string - special chars are preserved as-is
	// Color is applied by the template via prefixColor function
	builder.SetPrefix("{{WORK}}")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	// Braces are preserved as literal text (no template parsing)
	if data.Prefix != "{{WORK}}" {
		t.Errorf("Prefix = %q, want %q", data.Prefix, "{{WORK}}")
	}
}

func TestSetPrefix_Empty(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	builder.SetPrefix("")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	if data.Prefix != "" {
		t.Errorf("Prefix = %q, want empty", data.Prefix)
	}
}

func TestBuild_ContextWindowFromStdin(t *testing.T) {
	tests := []struct {
		name          string
		contextWindow *ContextWindowInfo
		wantPct       float64
		wantPctUse    float64
		wantInput     int64
		wantOutput    int64
		wantCached    int64
		wantCtxSize   int64
	}{
		{
			name: "full context_window data",
			contextWindow: &ContextWindowInfo{
				UsedPercentage:      ptrFloat64(14.1),
				RemainingPercentage: ptrFloat64(85.9),
				ContextWindowSize:   1_000_000,
				TotalInputTokens:    15234,
				TotalOutputTokens:   4521,
				CurrentUsage: &CurrentUsageInfo{
					InputTokens:              8500,
					OutputTokens:             1200,
					CacheCreationInputTokens: 5000,
					CacheReadInputTokens:     2000,
				},
			},
			wantPct:     14.1,
			wantPctUse:  17.625, // 14.1 * 1.25
			wantInput:   8500,
			wantOutput:  1200,
			wantCached:  7000, // 5000 + 2000
			wantCtxSize: 1_000_000,
		},
		{
			name: "200k context window",
			contextWindow: &ContextWindowInfo{
				UsedPercentage:    ptrFloat64(50.0),
				ContextWindowSize: 200_000,
			},
			wantPct:     50.0,
			wantPctUse:  62.5, // 50 * 1.25
			wantCtxSize: 200_000,
		},
		{
			name: "no current_usage (null before first API call)",
			contextWindow: &ContextWindowInfo{
				UsedPercentage:    ptrFloat64(8.0),
				ContextWindowSize: 200_000,
				CurrentUsage:      nil,
			},
			wantPct:     8.0,
			wantPctUse:  10.0, // 8 * 1.25
			wantCtxSize: 200_000,
		},
		{
			name: "high usage caps usable at 100",
			contextWindow: &ContextWindowInfo{
				UsedPercentage:    ptrFloat64(90.0),
				ContextWindowSize: 200_000,
			},
			wantPct:     90.0,
			wantPctUse:  100.0, // 90 * 1.25 = 112.5, capped at 100
			wantCtxSize: 200_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cache := &mockCacheProvider{}
			builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

			input := Input{
				Model:         ModelInfo{DisplayName: "Claude"},
				Workspace:     WorkspaceInfo{CurrentDir: "/project"},
				ContextWindow: tt.contextWindow,
			}

			data := builder.Build(input)

			if data.ContextPct != tt.wantPct {
				t.Errorf("ContextPct = %v, want %v", data.ContextPct, tt.wantPct)
			}
			if data.ContextPctUse != tt.wantPctUse {
				t.Errorf("ContextPctUse = %v, want %v", data.ContextPctUse, tt.wantPctUse)
			}
			if data.TokensInput != tt.wantInput {
				t.Errorf("TokensInput = %d, want %d", data.TokensInput, tt.wantInput)
			}
			if data.TokensOutput != tt.wantOutput {
				t.Errorf("TokensOutput = %d, want %d", data.TokensOutput, tt.wantOutput)
			}
			if data.TokensCached != tt.wantCached {
				t.Errorf("TokensCached = %d, want %d", data.TokensCached, tt.wantCached)
			}
			if data.ContextWindowSize != tt.wantCtxSize {
				t.Errorf("ContextWindowSize = %d, want %d", data.ContextWindowSize, tt.wantCtxSize)
			}
		})
	}
}

func TestBuild_ContextWindowNullPercentage(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	// context_window present but used_percentage is null (early in session)
	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
		ContextWindow: &ContextWindowInfo{
			UsedPercentage:    nil,
			ContextWindowSize: 200_000,
		},
	}

	data := builder.Build(input)

	// Should fall through to transcript parsing (which returns 0 with no transcript)
	if data.ContextPct != 0 {
		t.Errorf("ContextPct = %v, want 0 (null percentage should fallback)", data.ContextPct)
	}
}

func TestBuild_ContextWindowFallback(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

	// Create a transcript file for fallback
	tmpDir := t.TempDir()
	transcriptPath := tmpDir + "/transcript.jsonl"
	jsonlContent := `{"type":"summary","summary":"Test session"}
{"parentUuid":"123","isSidechain":false,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":10000,"output_tokens":5000,"cache_read_input_tokens":30000,"cache_creation_input_tokens":5000}}}
`
	if err := writeTestFile(transcriptPath, jsonlContent); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// No context_window — should fall back to transcript
	input := Input{
		Model:          ModelInfo{ID: "claude-opus-4-5-20251101", DisplayName: "Claude"},
		Workspace:      WorkspaceInfo{CurrentDir: "/project"},
		TranscriptPath: transcriptPath,
	}

	data := builder.Build(input)

	// Should have token data from transcript
	if data.TokensInput != 10000 {
		t.Errorf("TokensInput = %d, want 10000 (from transcript fallback)", data.TokensInput)
	}
	if data.ContextPct == 0 {
		t.Error("ContextPct should not be zero (transcript has data)")
	}
}

func TestBuild_CostMetrics(t *testing.T) {
	tests := []struct {
		name         string
		cost         *CostInfo
		wantCostUSD  float64
		wantDuration int64
		wantAPIDur   int64
	}{
		{
			name: "full cost data",
			cost: &CostInfo{
				TotalCostUSD:       0.05,
				TotalDurationMS:    125000,
				TotalAPIDurationMS: 2300,
			},
			wantCostUSD:  0.05,
			wantDuration: 125000,
			wantAPIDur:   2300,
		},
		{
			name:         "nil cost (not provided)",
			cost:         nil,
			wantCostUSD:  0,
			wantDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cache := &mockCacheProvider{}
			builder := NewBuilderWithDeps(&cfg, cache, nil, nil, nil, "")

			input := Input{
				Model:     ModelInfo{DisplayName: "Claude"},
				Workspace: WorkspaceInfo{CurrentDir: "/project"},
				Cost:      tt.cost,
			}

			data := builder.Build(input)

			if data.CostUSD != tt.wantCostUSD {
				t.Errorf("CostUSD = %v, want %v", data.CostUSD, tt.wantCostUSD)
			}
			if data.DurationMS != tt.wantDuration {
				t.Errorf("DurationMS = %d, want %d", data.DurationMS, tt.wantDuration)
			}
			if data.APIDurationMS != tt.wantAPIDur {
				t.Errorf("APIDurationMS = %d, want %d", data.APIDurationMS, tt.wantAPIDur)
			}
		})
	}
}

func ptrFloat64(f float64) *float64 {
	return &f
}

func TestBuild_TasksZeroValues(t *testing.T) {
	cfg := config.Default()

	taskProvider := &mockTaskProvider{
		name:      "test",
		available: true,
		stats: tasks.Stats{
			TotalIssues:      0,
			OpenIssues:       0,
			InProgressIssues: 0,
			ReadyIssues:      0,
			BlockedIssues:    0,
		},
	}

	cache := &mockCacheProvider{
		fetchTasks: true,
	}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil, taskProvider, "/project")

	input := Input{
		Model:     ModelInfo{DisplayName: "Claude"},
		Workspace: WorkspaceInfo{CurrentDir: "/project"},
	}

	data := builder.Build(input)

	// Should have HasTasks true even with zero values
	if !data.HasTasks {
		t.Error("HasTasks should be true even with zero stats")
	}

	// Values should be zero
	if data.TasksOpen != 0 {
		t.Errorf("TasksOpen = %d, want 0", data.TasksOpen)
	}
	if data.TasksReady != 0 {
		t.Errorf("TasksReady = %d, want 0", data.TasksReady)
	}
	if data.TasksInProgress != 0 {
		t.Errorf("TasksInProgress = %d, want 0", data.TasksInProgress)
	}
	if data.TasksBlocked != 0 {
		t.Errorf("TasksBlocked = %d, want 0", data.TasksBlocked)
	}
}
