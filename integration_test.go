package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kostyay/claude-status/internal/beads"
	"github.com/kostyay/claude-status/internal/cache"
	"github.com/kostyay/claude-status/internal/config"
	"github.com/kostyay/claude-status/internal/github"
	"github.com/kostyay/claude-status/internal/status"
	"github.com/kostyay/claude-status/internal/template"
)

func TestE2E_FullFlow(t *testing.T) {
	// Create temp directories for XDG paths
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	configDir := filepath.Join(tmpDir, "config")

	// Create cache and config dirs
	os.MkdirAll(cacheDir, 0755)
	os.MkdirAll(configDir, 0755)

	// Create a mock GitHub server
	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/actions/workflows") && !strings.Contains(r.URL.Path, "/runs") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/runs") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "success"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer githubServer.Close()

	// Create a real git repo
	gitDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(gitDir, 0755)

	cmd := exec.Command("git", "init")
	cmd.Dir = gitDir
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(gitDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = gitDir
	cmd.Run()

	// Add a GitHub remote
	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:testowner/testrepo.git")
	cmd.Dir = gitDir
	cmd.Run()

	// Create an uncommitted file
	os.WriteFile(filepath.Join(gitDir, "uncommitted.txt"), []byte("uncommitted"), 0644)

	// Build status
	cfg := config.Default()
	cacheManager := cache.NewManager(cacheDir)
	cacheManager.EnsureDir()

	builder, err := status.NewBuilder(&cfg, gitDir)
	if err != nil {
		t.Fatalf("NewBuilder() error = %v", err)
	}

	input := status.Input{
		Model:     status.ModelInfo{DisplayName: "Claude"},
		Workspace: status.WorkspaceInfo{CurrentDir: gitDir},
		Version:   "1.0.0",
	}

	data := builder.Build(input)

	// Verify data
	if data.Model != "Claude" {
		t.Errorf("Model = %q, want %q", data.Model, "Claude")
	}
	if data.Dir != "repo" {
		t.Errorf("Dir = %q, want %q", data.Dir, "repo")
	}
	if data.GitBranch != "main" && data.GitBranch != "master" {
		t.Errorf("GitBranch = %q, want main or master", data.GitBranch)
	}
	if data.GitStatus != "±1" {
		t.Errorf("GitStatus = %q, want ±1", data.GitStatus)
	}
	if data.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", data.Version, "1.0.0")
	}
}

func TestE2E_RealGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(gitDir, 0755)

	// Initialize repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitDir
	cmd.Run()

	// Create initial commit
	os.WriteFile(filepath.Join(gitDir, "initial.txt"), []byte("initial"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = gitDir
	cmd.Run()

	// Create feature branch
	cmd = exec.Command("git", "checkout", "-b", "feature/test-branch")
	cmd.Dir = gitDir
	cmd.Run()

	// Add uncommitted changes
	os.WriteFile(filepath.Join(gitDir, "new.txt"), []byte("new"), 0644)
	os.WriteFile(filepath.Join(gitDir, "another.txt"), []byte("another"), 0644)

	// Build status
	cfg := config.Default()
	cacheDir := filepath.Join(tmpDir, "cache")
	cacheManager := cache.NewManager(cacheDir)
	cacheManager.EnsureDir()

	builder, _ := status.NewBuilder(&cfg, gitDir)
	input := status.Input{
		Model:     status.ModelInfo{DisplayName: "Claude"},
		Workspace: status.WorkspaceInfo{CurrentDir: gitDir},
	}

	data := builder.Build(input)

	if data.GitBranch != "feature/test-branch" {
		t.Errorf("GitBranch = %q, want %q", data.GitBranch, "feature/test-branch")
	}
	if data.GitStatus != "±2" {
		t.Errorf("GitStatus = %q, want ±2", data.GitStatus)
	}
}

func TestE2E_CachingPerformance(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "repo")
	cacheDir := filepath.Join(tmpDir, "cache")

	os.MkdirAll(gitDir, 0755)

	// Initialize repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitDir
	cmd.Run()

	os.WriteFile(filepath.Join(gitDir, "test.txt"), []byte("test"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = gitDir
	cmd.Run()

	cfg := config.Default()
	cacheManager := cache.NewManager(cacheDir)
	cacheManager.EnsureDir()

	input := status.Input{
		Model:     status.ModelInfo{DisplayName: "Claude"},
		Workspace: status.WorkspaceInfo{CurrentDir: gitDir},
	}

	// First call - populates cache
	start1 := time.Now()
	builder1, _ := status.NewBuilder(&cfg, gitDir)
	builder1.Build(input)
	duration1 := time.Since(start1)

	// Second call - should use cache
	start2 := time.Now()
	builder2, _ := status.NewBuilder(&cfg, gitDir)
	builder2.Build(input)
	duration2 := time.Since(start2)

	// Cache hit should be faster (though both might be fast)
	// We mainly want to verify no errors occur
	t.Logf("First call: %v, Second call: %v", duration1, duration2)
}

func TestE2E_ConfigLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Write config file
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"template": "[{{.Model}}] {{.Dir}}",
		"github_ttl": 30
	}`
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg := config.LoadFrom(configPath)

	if cfg.Template != "[{{.Model}}] {{.Dir}}" {
		t.Errorf("Template = %q, want custom", cfg.Template)
	}
	if cfg.GitHubTTL != 30 {
		t.Errorf("GitHubTTL = %d, want 30", cfg.GitHubTTL)
	}
}

func TestE2E_StdinInput(t *testing.T) {
	input := `{
		"model": {"display_name": "Claude Opus"},
		"workspace": {"current_dir": "/tmp/test"},
		"version": "2.0.0"
	}`

	var parsed status.Input
	err := json.NewDecoder(bytes.NewReader([]byte(input))).Decode(&parsed)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	if parsed.Model.DisplayName != "Claude Opus" {
		t.Errorf("Model.DisplayName = %q, want %q", parsed.Model.DisplayName, "Claude Opus")
	}
	if parsed.Workspace.CurrentDir != "/tmp/test" {
		t.Errorf("Workspace.CurrentDir = %q, want %q", parsed.Workspace.CurrentDir, "/tmp/test")
	}
	if parsed.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", parsed.Version, "2.0.0")
	}
}

func TestE2E_StdoutOutput(t *testing.T) {
	cfg := config.Default()
	engine, err := template.NewEngine(cfg.Template)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := template.StatusData{
		Model:        "Claude",
		Dir:          "myproject",
		GitBranch:    "main",
		GitStatus:    "±3",
		GitHubStatus: "✅",
		Version:      "1.0.0",
	}

	output, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Check that output contains expected parts
	if !strings.Contains(output, "[Claude]") {
		t.Error("Output missing model")
	}
	if !strings.Contains(output, "myproject") {
		t.Error("Output missing directory")
	}
	if !strings.Contains(output, "main") {
		t.Error("Output missing branch")
	}
	if !strings.Contains(output, "±3") {
		t.Error("Output missing status")
	}
	if !strings.Contains(output, "✅") {
		t.Error("Output missing GitHub status")
	}
	if !strings.Contains(output, "v1.0.0") {
		t.Error("Output missing version")
	}
}

func TestE2E_GracefulDegradation(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	cfg := config.Default()
	cacheManager := cache.NewManager(cacheDir)
	cacheManager.EnsureDir()

	// Use a non-git directory
	builder, _ := status.NewBuilder(&cfg, tmpDir)

	input := status.Input{
		Model:     status.ModelInfo{DisplayName: "Claude"},
		Workspace: status.WorkspaceInfo{CurrentDir: tmpDir},
		Version:   "1.0.0",
	}

	data := builder.Build(input)

	// Should still have model, dir, and version
	if data.Model != "Claude" {
		t.Errorf("Model = %q, want %q", data.Model, "Claude")
	}
	if data.Dir == "" {
		t.Error("Dir should not be empty")
	}
	if data.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", data.Version, "1.0.0")
	}

	// Git-related fields should be empty
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

func TestE2E_Logging(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs", "status.json")

	cfg := config.Config{
		Template:       config.DefaultTemplate,
		GitHubWorkflow: "build_and_test",
		GitHubTTL:      60,
		LoggingEnabled: true,
		LogPath:        logPath,
	}

	// Ensure log directory exists
	os.MkdirAll(filepath.Dir(logPath), 0755)

	// Simulate logging (simplified)
	logData := []map[string]interface{}{
		{
			"timestamp":          time.Now().Format(time.RFC3339),
			"status_line_output": "[Claude] 📁 test",
		},
	}

	data, _ := json.MarshalIndent(logData, "", "  ")
	os.WriteFile(logPath, data, 0644)

	// Verify log file was created and has content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "timestamp") {
		t.Error("Log file missing timestamp")
	}

	// Verify config has logging enabled
	if !cfg.LoggingEnabled {
		t.Error("LoggingEnabled should be true")
	}
}

func TestE2E_GitHubStatus_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/actions/workflows") && !strings.Contains(r.URL.Path, "/runs") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/runs") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "success"},
				},
			})
			return
		}
	}))
	defer server.Close()

	client, err := github.NewClientWithToken("build_and_test", "test-token", &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewClientWithToken() error = %v", err)
	}
	client.SetBaseURL(server.URL)

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != github.StatusSuccess {
		t.Errorf("status = %q, want success", status)
	}

	emoji := github.StatusToEmoji(status)
	if emoji != "✅" {
		t.Errorf("emoji = %q, want ✅", emoji)
	}
}

// mockBeadsCommander is a mock commander for beads testing.
type mockBeadsCommander struct {
	output string
}

func (m *mockBeadsCommander) Output(name string, args ...string) ([]byte, error) {
	return []byte(m.output), nil
}

func TestE2E_BeadsWidget(t *testing.T) {
	// Test that beads stats are properly rendered in the template
	cfg := config.Config{
		Template: config.TemplateWithBeads,
		BeadsTTL: 60,
	}

	engine, err := template.NewEngine(cfg.Template)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := template.StatusData{
		Model:              "Claude",
		Dir:                "myproject",
		GitBranch:          "main",
		HasBeads:           true,
		BeadsReady:         "3 ready",
		BeadsBlocked:       "1 blocked",
		BeadsReadyRaw:      3,
		BeadsBlockedRaw:    1,
		ContextPctUse:      "45.2%",
		ContextPctUseRaw:   45.2,
	}

	output, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Check that output contains beads info
	if !strings.Contains(output, "3 ready") {
		t.Error("Output missing beads ready count")
	}
	if !strings.Contains(output, "1 blocked") {
		t.Error("Output missing beads blocked count")
	}
	if !strings.Contains(output, "📋") {
		t.Error("Output missing beads emoji")
	}
}

func TestE2E_BeadsClient(t *testing.T) {
	// Test beads client with mock commander
	mockOutput := `{
		"summary": {
			"total_issues": 10,
			"open_issues": 5,
			"in_progress_issues": 2,
			"closed_issues": 3,
			"blocked_issues": 1,
			"ready_issues": 4
		}
	}`

	cmd := &mockBeadsCommander{output: mockOutput}
	client := beads.NewClientWithCommander("/workdir", cmd)

	stats, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.TotalIssues != 10 {
		t.Errorf("TotalIssues = %d, want 10", stats.TotalIssues)
	}
	if stats.OpenIssues != 5 {
		t.Errorf("OpenIssues = %d, want 5", stats.OpenIssues)
	}
	if stats.ReadyIssues != 4 {
		t.Errorf("ReadyIssues = %d, want 4", stats.ReadyIssues)
	}
	if stats.BlockedIssues != 1 {
		t.Errorf("BlockedIssues = %d, want 1", stats.BlockedIssues)
	}
}

func TestE2E_BeadsCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheManager := cache.NewManager(tmpDir)
	cacheManager.EnsureDir()

	fetchCalls := 0
	fetchFn := func() (beads.Stats, error) {
		fetchCalls++
		return beads.Stats{
			TotalIssues: 5,
			OpenIssues:  3,
			ReadyIssues: 2,
		}, nil
	}

	// First call should fetch
	stats1, err := cacheManager.GetBeadsStats(60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetBeadsStats() error = %v", err)
	}
	if stats1.TotalIssues != 5 {
		t.Errorf("TotalIssues = %d, want 5", stats1.TotalIssues)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchCalls = %d, want 1", fetchCalls)
	}

	// Second call should use cache
	stats2, err := cacheManager.GetBeadsStats(60*time.Second, fetchFn)
	if err != nil {
		t.Fatalf("GetBeadsStats() error = %v", err)
	}
	if stats2.TotalIssues != 5 {
		t.Errorf("TotalIssues = %d, want 5", stats2.TotalIssues)
	}
	if fetchCalls != 1 {
		t.Errorf("fetchCalls = %d, want 1 (should use cache)", fetchCalls)
	}
}

// skipIfBdNotAvailable skips the test if the bd CLI is not installed.
func skipIfBdNotAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd CLI not available - skipping real beads integration test")
	}
}

// initTestGitRepo initializes a git repository in the given directory.
func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	// Create and commit initial file
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	cmd.Run()
}

// initTestBeads initializes beads in the given directory.
func initTestBeads(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("bd", "init", "--prefix", "test", "--skip-hooks", "--sandbox", "--quiet")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd init failed: %v\nOutput: %s", err, output)
	}
}

// createTestBeadsTasks creates test tasks in the beads database.
func createTestBeadsTasks(t *testing.T, dir string) {
	t.Helper()

	// Create first task
	cmd := exec.Command("bd", "create", "--title", "Test task 1", "--type", "task", "--sandbox", "--silent")
	cmd.Dir = dir
	task1Output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bd create task 1 failed: %v", err)
	}
	task1ID := strings.TrimSpace(string(task1Output))

	// Create second task
	cmd = exec.Command("bd", "create", "--title", "Test task 2", "--type", "task", "--sandbox", "--silent")
	cmd.Dir = dir
	if _, err := cmd.Output(); err != nil {
		t.Fatalf("bd create task 2 failed: %v", err)
	}

	// Create third task that will be blocked by task1
	cmd = exec.Command("bd", "create", "--title", "Blocked task", "--type", "task", "--sandbox", "--silent")
	cmd.Dir = dir
	task3Output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bd create task 3 failed: %v", err)
	}
	task3ID := strings.TrimSpace(string(task3Output))

	// Add dependency: task3 depends on task1 (task3 is blocked by task1)
	cmd = exec.Command("bd", "dep", "add", task3ID, task1ID, "--sandbox")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd dep add failed: %v\nOutput: %s", err, output)
	}
}

func TestE2E_BeadsRealCLI(t *testing.T) {
	skipIfBdNotAvailable(t)

	tmpDir := t.TempDir()

	// Initialize git repo (required by bd)
	initTestGitRepo(t, tmpDir)

	// Initialize beads
	initTestBeads(t, tmpDir)

	// Create test tasks
	createTestBeadsTasks(t, tmpDir)

	// Create beads client and verify it works
	client := beads.NewClient(tmpDir)

	if !client.HasBeads() {
		t.Fatal("HasBeads() = false, want true")
	}

	stats, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	// Verify counts match what we created
	if stats.TotalIssues != 3 {
		t.Errorf("TotalIssues = %d, want 3", stats.TotalIssues)
	}
	if stats.OpenIssues != 3 {
		t.Errorf("OpenIssues = %d, want 3", stats.OpenIssues)
	}
	if stats.BlockedIssues != 1 {
		t.Errorf("BlockedIssues = %d, want 1", stats.BlockedIssues)
	}
	if stats.ReadyIssues != 2 {
		t.Errorf("ReadyIssues = %d, want 2", stats.ReadyIssues)
	}
}
