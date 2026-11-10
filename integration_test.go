package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kostyay/claude-status/internal/config"
	"github.com/kostyay/claude-status/internal/status"
	"github.com/kostyay/claude-status/internal/template"
)

func TestE2E_FullFlow(t *testing.T) {
	tmpDir := t.TempDir()

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

	// Create an uncommitted file
	os.WriteFile(filepath.Join(gitDir, "uncommitted.txt"), []byte("uncommitted"), 0644)

	// Build status
	cfg := config.Default()
	builder, err := status.NewBuilder(&cfg, gitDir, "")
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
	builder, _ := status.NewBuilder(&cfg, gitDir, "")
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

func TestE2E_DefaultTemplateEffort(t *testing.T) {
	engine, err := template.NewEngine(config.Default().Template)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	tests := []struct {
		effort string
		want   string
	}{
		{effort: "xhigh", want: "[Opus 5.5 🧠 xhigh]"},
		{effort: "", want: "[Opus 5.5]"},
	}
	for _, tt := range tests {
		data := template.StatusData{Model: "Opus 5.5", Dir: "p", EffortLevel: tt.effort}
		output, err := engine.Render(data)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}
		if !strings.Contains(output, tt.want) {
			t.Errorf("effort %q: output %q missing %q", tt.effort, output, tt.want)
		}
	}
}

func TestE2E_StdinSchema(t *testing.T) {
	input := `{
		"model": {"id": "claude-opus-5-5", "display_name": "Opus"},
		"workspace": {"current_dir": "/tmp/proj"},
		"context_window": {"used_percentage": 60, "context_window_size": 1000000},
		"fast_mode": true,
		"pr": {"number": 7, "url": "https://gitlab.com/g/p/-/merge_requests/7", "kind": "mr"}
	}`
	var parsed status.Input
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	cfg := config.Default()
	data := status.NewBuilderWithDeps(&cfg, nil, nil, nil, nil, "").Build(parsed)
	if !data.FastMode {
		t.Error("FastMode = false, want true")
	}
	if data.PRKind != "mr" {
		t.Errorf("PRKind = %q, want %q", data.PRKind, "mr")
	}

	engine, err := template.NewEngine(cfg.Template)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	output, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(output, "📊 60.0%") {
		t.Errorf("output %q should show Claude Code's used_percentage as-is", output)
	}
}

func TestE2E_GracefulDegradation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()

	// Use a non-git directory
	builder, _ := status.NewBuilder(&cfg, tmpDir, "")

	input := status.Input{
		Model:     status.ModelInfo{DisplayName: "Claude"},
		Workspace: status.WorkspaceInfo{CurrentDir: tmpDir},
		Version:   "1.0.0",
	}

	data := builder.Build(input)

	// Should still have dir and version
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

func TestE2E_TasksWidget(t *testing.T) {
	engine, err := template.NewEngine(config.DefaultTemplate)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	data := template.StatusData{
		Model:        "Claude",
		Dir:          "myproject",
		GitBranch:    "main",
		TaskProvider: "kt",
		HasTasks:     true,
		TasksReady:   3,
		TasksBlocked: 1,
		ContextPct:   45.2,
	}

	output, err := engine.Render(data)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Check that output contains task info
	if !strings.Contains(output, "3 ready") {
		t.Error("Output missing tasks ready count")
	}
	if !strings.Contains(output, "1 blocked") {
		t.Error("Output missing tasks blocked count")
	}
	if !strings.Contains(output, "📋") {
		t.Error("Output missing tasks emoji")
	}
}
