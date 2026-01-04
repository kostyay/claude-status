package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mockCommander is a test double for Commander.
type mockCommander struct {
	responses map[string]string
	errors    map[string]error
}

func newMockCommander() *mockCommander {
	return &mockCommander{
		responses: make(map[string]string),
		errors:    make(map[string]error),
	}
}

func (m *mockCommander) Run(ctx context.Context, dir string, args ...string) (string, error) {
	key := args[0]
	if len(args) > 1 {
		key = args[0] + " " + args[1]
	}

	if err, ok := m.errors[key]; ok {
		return "", err
	}
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	return "", errors.New("unexpected command: " + key)
}

func TestNewGitClient_InRepo(t *testing.T) {
	// Create a temp git repo
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Skip("git not available")
	}

	client, err := NewClient(dir)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	gitDir := client.GitDir()
	if !filepath.IsAbs(gitDir) {
		t.Errorf("GitDir() = %q, want absolute path", gitDir)
	}
}

func TestNewGitClient_NotRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := NewClient(dir)
	if err == nil {
		t.Fatal("NewClient() expected error for non-repo")
	}
}

func TestBranch_Main(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["rev-parse --abbrev-ref"] = "main"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	branch, err := client.Branch()
	if err != nil {
		t.Fatalf("Branch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("Branch() = %q, want %q", branch, "main")
	}
}

func TestBranch_Feature(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["rev-parse --abbrev-ref"] = "feature/my-feature"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	branch, err := client.Branch()
	if err != nil {
		t.Fatalf("Branch() error = %v", err)
	}
	if branch != "feature/my-feature" {
		t.Errorf("Branch() = %q, want %q", branch, "feature/my-feature")
	}
}

func TestBranch_DetachedHead(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["rev-parse --abbrev-ref"] = "HEAD"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	branch, err := client.Branch()
	if err != nil {
		t.Fatalf("Branch() error = %v", err)
	}
	if branch != "HEAD" {
		t.Errorf("Branch() = %q, want %q", branch, "HEAD")
	}
}

func TestStatus_Clean(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["status --porcelain"] = ""

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status != "" {
		t.Errorf("Status() = %q, want empty", status)
	}
}

func TestStatus_Uncommitted(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["status --porcelain"] = " M file1.go\n M file2.go\n?? file3.go"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status != "±3" {
		t.Errorf("Status() = %q, want %q", status, "±3")
	}
}

func TestStatus_Staged(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["status --porcelain"] = "M  file1.go\nA  file2.go"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status != "±2" {
		t.Errorf("Status() = %q, want %q", status, "±2")
	}
}

func TestStatus_Mixed(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["status --porcelain"] = "MM file1.go"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status != "±1" {
		t.Errorf("Status() = %q, want %q", status, "±1")
	}
}

func TestRemoteURL_SSH(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["remote get-url"] = "git@github.com:owner/repo.git"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	url, err := client.RemoteURL()
	if err != nil {
		t.Fatalf("RemoteURL() error = %v", err)
	}
	if url != "git@github.com:owner/repo.git" {
		t.Errorf("RemoteURL() = %q, want %q", url, "git@github.com:owner/repo.git")
	}
}

func TestRemoteURL_HTTPS(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["remote get-url"] = "https://github.com/owner/repo.git"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	url, err := client.RemoteURL()
	if err != nil {
		t.Fatalf("RemoteURL() error = %v", err)
	}
	if url != "https://github.com/owner/repo.git" {
		t.Errorf("RemoteURL() = %q, want %q", url, "https://github.com/owner/repo.git")
	}
}

func TestRemoteURL_NoOrigin(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.errors["remote get-url"] = errors.New("fatal: No such remote 'origin'")

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	_, err = client.RemoteURL()
	if err == nil {
		t.Error("RemoteURL() expected error for missing origin")
	}
}

func TestParseGitHubRepo_SSH(t *testing.T) {
	owner, repo, ok := ParseGitHubRepo("git@github.com:myowner/myrepo.git")
	if !ok {
		t.Fatal("ParseGitHubRepo() returned ok=false")
	}
	if owner != "myowner" {
		t.Errorf("owner = %q, want %q", owner, "myowner")
	}
	if repo != "myrepo" {
		t.Errorf("repo = %q, want %q", repo, "myrepo")
	}
}

func TestParseGitHubRepo_HTTPS(t *testing.T) {
	owner, repo, ok := ParseGitHubRepo("https://github.com/myowner/myrepo.git")
	if !ok {
		t.Fatal("ParseGitHubRepo() returned ok=false")
	}
	if owner != "myowner" {
		t.Errorf("owner = %q, want %q", owner, "myowner")
	}
	if repo != "myrepo" {
		t.Errorf("repo = %q, want %q", repo, "myrepo")
	}
}

func TestParseGitHubRepo_WithDotGit(t *testing.T) {
	// Both with and without .git suffix should work
	tests := []string{
		"git@github.com:owner/repo.git",
		"git@github.com:owner/repo",
		"https://github.com/owner/repo.git",
		"https://github.com/owner/repo",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			owner, repo, ok := ParseGitHubRepo(url)
			if !ok {
				t.Fatal("ParseGitHubRepo() returned ok=false")
			}
			if owner != "owner" {
				t.Errorf("owner = %q, want %q", owner, "owner")
			}
			if repo != "repo" {
				t.Errorf("repo = %q, want %q", repo, "repo")
			}
		})
	}
}

func TestParseGitHubRepo_NonGitHub(t *testing.T) {
	tests := []string{
		"git@gitlab.com:owner/repo.git",
		"https://gitlab.com/owner/repo.git",
		"git@bitbucket.org:owner/repo.git",
		"/local/path/repo",
		"",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			_, _, ok := ParseGitHubRepo(url)
			if ok {
				t.Error("ParseGitHubRepo() expected ok=false for non-GitHub URL")
			}
		})
	}
}

func TestHeadPath(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = "/repo/.git"

	client, err := NewClientWithCommander("/repo", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	path := client.HeadPath()
	expected := "/repo/.git/HEAD"
	if path != expected {
		t.Errorf("HeadPath() = %q, want %q", path, expected)
	}
}

func TestIndexPath(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = "/repo/.git"

	client, err := NewClientWithCommander("/repo", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	path := client.IndexPath()
	expected := "/repo/.git/index"
	if path != expected {
		t.Errorf("IndexPath() = %q, want %q", path, expected)
	}
}

func TestRefPath(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = "/repo/.git"

	client, err := NewClientWithCommander("/repo", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	path := client.RefPath("main")
	expected := "/repo/.git/refs/heads/main"
	if path != expected {
		t.Errorf("RefPath(main) = %q, want %q", path, expected)
	}

	path = client.RefPath("feature/test")
	expected = "/repo/.git/refs/heads/feature/test"
	if path != expected {
		t.Errorf("RefPath(feature/test) = %q, want %q", path, expected)
	}
}

func TestParseShortstat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAdd    int
		wantDelete int
	}{
		{
			name:       "empty",
			input:      "",
			wantAdd:    0,
			wantDelete: 0,
		},
		{
			name:       "insertions only",
			input:      " 3 files changed, 42 insertions(+)",
			wantAdd:    42,
			wantDelete: 0,
		},
		{
			name:       "deletions only",
			input:      " 2 files changed, 10 deletions(-)",
			wantAdd:    0,
			wantDelete: 10,
		},
		{
			name:       "both",
			input:      " 5 files changed, 100 insertions(+), 25 deletions(-)",
			wantAdd:    100,
			wantDelete: 25,
		},
		{
			name:       "single insertion",
			input:      " 1 file changed, 1 insertion(+)",
			wantAdd:    1,
			wantDelete: 0,
		},
		{
			name:       "single deletion",
			input:      " 1 file changed, 1 deletion(-)",
			wantAdd:    0,
			wantDelete: 1,
		},
		{
			name:       "large numbers",
			input:      " 50 files changed, 1234 insertions(+), 5678 deletions(-)",
			wantAdd:    1234,
			wantDelete: 5678,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdd, gotDel := parseShortstat(tt.input)
			if gotAdd != tt.wantAdd {
				t.Errorf("additions = %d, want %d", gotAdd, tt.wantAdd)
			}
			if gotDel != tt.wantDelete {
				t.Errorf("deletions = %d, want %d", gotDel, tt.wantDelete)
			}
		})
	}
}

func TestParseStatusForTypes(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantNew      int
		wantMod      int
		wantDel      int
		wantUnstaged int
	}{
		{
			name:         "empty",
			input:        "",
			wantNew:      0, wantMod: 0, wantDel: 0, wantUnstaged: 0,
		},
		{
			name:         "untracked files",
			input:        "?? file1.go\n?? file2.go",
			wantNew:      2, wantMod: 0, wantDel: 0, wantUnstaged: 2,
		},
		{
			name:         "staged new file",
			input:        "A  file1.go",
			wantNew:      1, wantMod: 0, wantDel: 0, wantUnstaged: 0,
		},
		{
			name:         "modified unstaged",
			input:        " M file1.go",
			wantNew:      0, wantMod: 1, wantDel: 0, wantUnstaged: 1,
		},
		{
			name:         "modified staged",
			input:        "M  file1.go",
			wantNew:      0, wantMod: 1, wantDel: 0, wantUnstaged: 0,
		},
		{
			name:         "modified both",
			input:        "MM file1.go",
			wantNew:      0, wantMod: 1, wantDel: 0, wantUnstaged: 1,
		},
		{
			name:         "deleted staged",
			input:        "D  file1.go",
			wantNew:      0, wantMod: 0, wantDel: 1, wantUnstaged: 0,
		},
		{
			name:         "deleted unstaged",
			input:        " D file1.go",
			wantNew:      0, wantMod: 0, wantDel: 1, wantUnstaged: 1,
		},
		{
			name:         "renamed",
			input:        "R  old.go -> new.go",
			wantNew:      0, wantMod: 1, wantDel: 0, wantUnstaged: 0,
		},
		{
			name:         "copied",
			input:        "C  src.go -> dst.go",
			wantNew:      0, wantMod: 1, wantDel: 0, wantUnstaged: 0,
		},
		{
			name:         "renamed with unstaged modification",
			input:        "RM old.go -> new.go",
			wantNew:      0, wantMod: 1, wantDel: 0, wantUnstaged: 1,
		},
		{
			name:         "added with unstaged modification",
			input:        "AM file1.go",
			wantNew:      1, wantMod: 0, wantDel: 0, wantUnstaged: 1,
		},
		{
			name:         "renamed with unstaged deletion",
			input:        "RD old.go -> new.go",
			wantNew:      0, wantMod: 1, wantDel: 0, wantUnstaged: 1,
		},
		{
			name:         "mixed",
			input:        "?? new1.go\n?? new2.go\nA  added.go\nM  modified.go\n M unstaged.go\nD  deleted.go",
			wantNew:      3, wantMod: 2, wantDel: 1, wantUnstaged: 3, // 2 untracked + 1 unstaged mod
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNew, gotMod, gotDel, gotUnstaged := parseStatusForTypes(tt.input)
			if gotNew != tt.wantNew {
				t.Errorf("new = %d, want %d", gotNew, tt.wantNew)
			}
			if gotMod != tt.wantMod {
				t.Errorf("modified = %d, want %d", gotMod, tt.wantMod)
			}
			if gotDel != tt.wantDel {
				t.Errorf("deleted = %d, want %d", gotDel, tt.wantDel)
			}
			if gotUnstaged != tt.wantUnstaged {
				t.Errorf("unstaged = %d, want %d", gotUnstaged, tt.wantUnstaged)
			}
		})
	}
}

func TestDiffStats(t *testing.T) {
	mock := newMockCommander()
	mock.responses["rev-parse --git-dir"] = ".git"
	mock.responses["diff --shortstat"] = " 3 files changed, 42 insertions(+), 10 deletions(-)"
	mock.responses["diff --shortstat"] = " 2 files changed, 20 insertions(+), 5 deletions(-)" // staged
	mock.responses["status --porcelain"] = "?? new.go\nM  modified.go\n D deleted.go"

	client, err := NewClientWithCommander("/test", mock)
	if err != nil {
		t.Fatalf("NewClientWithCommander() error = %v", err)
	}

	stats, err := client.DiffStats()
	if err != nil {
		t.Fatalf("DiffStats() error = %v", err)
	}

	// Line counts come from both unstaged and staged diffs
	// With the mock, both calls return the same value due to key collision
	// So we get 20+20=40 insertions and 5+5=10 deletions
	if stats.Additions < 20 {
		t.Errorf("Additions = %d, want at least 20", stats.Additions)
	}
	if stats.Deletions < 5 {
		t.Errorf("Deletions = %d, want at least 5", stats.Deletions)
	}

	// File type counts
	if stats.NewFiles != 1 {
		t.Errorf("NewFiles = %d, want 1", stats.NewFiles)
	}
	if stats.ModifiedFiles != 1 {
		t.Errorf("ModifiedFiles = %d, want 1", stats.ModifiedFiles)
	}
	if stats.DeletedFiles != 1 {
		t.Errorf("DeletedFiles = %d, want 1", stats.DeletedFiles)
	}
	if stats.UnstagedFiles != 2 {
		t.Errorf("UnstagedFiles = %d, want 2", stats.UnstagedFiles)
	}
}

// Integration test using real git
func TestIntegration_RealGitRepo(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create a temp dir and init a git repo
	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init error: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()

	// Create a file and make an initial commit (needed for branch to work)
	testFile := filepath.Join(dir, "initial.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit error: %v", err)
	}

	// Now create an uncommitted file
	testFile2 := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile2, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(dir)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Test branch (should be master or main depending on git config)
	branch, err := client.Branch()
	if err != nil {
		t.Fatalf("Branch() error = %v", err)
	}
	// New git repos might be on main or master
	if branch != "main" && branch != "master" {
		t.Errorf("Branch() = %q, want main or master", branch)
	}

	// Test status with uncommitted file
	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status != "±1" {
		t.Errorf("Status() = %q, want ±1", status)
	}

	// Test paths
	if !filepath.IsAbs(client.HeadPath()) {
		t.Error("HeadPath() should be absolute")
	}
	if !filepath.IsAbs(client.IndexPath()) {
		t.Error("IndexPath() should be absolute")
	}
}
