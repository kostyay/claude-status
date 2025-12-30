package status

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kostya/claude-status/internal/config"
	"github.com/kostya/claude-status/internal/git"
	"github.com/kostya/claude-status/internal/github"
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

func (m *mockGitProvider) Branch() (string, error)               { return m.branch, m.branchErr }
func (m *mockGitProvider) Status() (string, error)               { return m.status, m.statusErr }
func (m *mockGitProvider) DiffStats() (git.DiffStats, error)     { return m.diffStats, m.diffStatsErr }
func (m *mockGitProvider) RemoteURL() (string, error)            { return m.remoteURL, m.remoteErr }
func (m *mockGitProvider) GitDir() string                        { return m.gitDir }
func (m *mockGitProvider) HeadPath() string                      { return m.gitDir + "/HEAD" }
func (m *mockGitProvider) IndexPath() string                     { return m.gitDir + "/index" }
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
	fetchBranch    bool
	fetchStatus    bool
	fetchDiffStats bool
	fetchBuild     bool
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

	builder := NewBuilderWithDeps(&cfg, cache, git, gh)

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
	builder := NewBuilderWithDeps(&cfg, cache, nil, nil)

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

	builder := NewBuilderWithDeps(&cfg, cache, git, nil)

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

	builder := NewBuilderWithDeps(&cfg, cache, git, gh)

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

	builder := NewBuilderWithDeps(&cfg, cache, git, gh)

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

	builder := NewBuilderWithDeps(&cfg, cache, git, gh)

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

	builder := NewBuilderWithDeps(&cfg, cache, git, nil)

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

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil)

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

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil)

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
	_, err := NewBuilder(nil, "/tmp")
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

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil)

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

	// Check token metrics are populated
	if data.TokensInput != "10k" {
		t.Errorf("TokensInput = %q, want %q", data.TokensInput, "10k")
	}
	if data.TokensOutput != "5k" {
		t.Errorf("TokensOutput = %q, want %q", data.TokensOutput, "5k")
	}
	if data.TokensCached != "35k" {
		t.Errorf("TokensCached = %q, want %q", data.TokensCached, "35k")
	}
	if data.TokensTotal != "50k" {
		t.Errorf("TokensTotal = %q, want %q", data.TokensTotal, "50k")
	}

	// Check raw values
	if data.TokensInputRaw != 10000 {
		t.Errorf("TokensInputRaw = %d, want %d", data.TokensInputRaw, 10000)
	}
	if data.TokensOutputRaw != 5000 {
		t.Errorf("TokensOutputRaw = %d, want %d", data.TokensOutputRaw, 5000)
	}
	if data.TokensCachedRaw != 35000 {
		t.Errorf("TokensCachedRaw = %d, want %d", data.TokensCachedRaw, 35000)
	}
	if data.TokensTotalRaw != 50000 {
		t.Errorf("TokensTotalRaw = %d, want %d", data.TokensTotalRaw, 50000)
	}

	// Check context percentage is calculated
	if data.ContextPctRaw == 0 {
		t.Error("ContextPctRaw should not be zero")
	}
}

func TestBuild_TokenMetrics_EmptyPath(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil)

	input := Input{
		Model:          ModelInfo{DisplayName: "Claude"},
		Workspace:      WorkspaceInfo{CurrentDir: "/project"},
		TranscriptPath: "", // Empty path
	}

	data := builder.Build(input)

	// Token metrics should be empty/zero
	if data.TokensInput != "" {
		t.Errorf("TokensInput = %q, want empty", data.TokensInput)
	}
	if data.TokensInputRaw != 0 {
		t.Errorf("TokensInputRaw = %d, want 0", data.TokensInputRaw)
	}
}

func TestBuild_TokenMetrics_InvalidPath(t *testing.T) {
	cfg := config.Default()
	cache := &mockCacheProvider{}

	builder := NewBuilderWithDeps(&cfg, cache, nil, nil)

	input := Input{
		Model:          ModelInfo{DisplayName: "Claude"},
		Workspace:      WorkspaceInfo{CurrentDir: "/project"},
		TranscriptPath: "/nonexistent/path/transcript.jsonl",
	}

	data := builder.Build(input)

	// Token metrics should be empty/zero (silent fail)
	if data.TokensInput != "" {
		t.Errorf("TokensInput = %q, want empty", data.TokensInput)
	}
	if data.TokensInputRaw != 0 {
		t.Errorf("TokensInputRaw = %d, want 0", data.TokensInputRaw)
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
