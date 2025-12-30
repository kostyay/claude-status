package git

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Commander executes git commands. This interface allows for testing.
type Commander interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecCommander is the default Commander that uses os/exec.
type ExecCommander struct{}

// Run executes a git command and returns the output.
func (e *ExecCommander) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// DiffStats holds git diff statistics.
type DiffStats struct {
	Additions     int // Lines added
	Deletions     int // Lines deleted
	NewFiles      int // Untracked or newly staged files
	ModifiedFiles int // Modified files
	DeletedFiles  int // Deleted files
}

// Client provides git operations for a working directory.
type Client struct {
	workDir string
	gitDir  string
	cmd     Commander
}

// NewClient creates a new git client for the given working directory.
// Returns an error if the directory is not a git repository.
func NewClient(workDir string) (*Client, error) {
	return NewClientWithCommander(workDir, &ExecCommander{})
}

// NewClientWithCommander creates a new git client with a custom commander.
func NewClientWithCommander(workDir string, cmd Commander) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	gitDir, err := cmd.Run(ctx, workDir, "rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	// Make gitDir absolute if it's relative
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(workDir, gitDir)
	}

	return &Client{
		workDir: workDir,
		gitDir:  gitDir,
		cmd:     cmd,
	}, nil
}

// GitDir returns the path to the .git directory.
func (c *Client) GitDir() string {
	return c.gitDir
}

// Branch returns the current branch name.
// Returns "HEAD" for detached HEAD state.
func (c *Client) Branch() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return c.cmd.Run(ctx, c.workDir, "rev-parse", "--abbrev-ref", "HEAD")
}

// Status returns a string representing uncommitted changes.
// Returns empty string if the working tree is clean.
// Returns "±N" where N is the number of changed files.
func (c *Client) Status() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := c.cmd.Run(ctx, c.workDir, "status", "--porcelain")
	if err != nil {
		return "", err
	}

	if out == "" {
		return "", nil
	}

	lines := strings.Split(out, "\n")
	return fmt.Sprintf("±%d", len(lines)), nil
}

// RemoteURL returns the URL of the origin remote.
func (c *Client) RemoteURL() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return c.cmd.Run(ctx, c.workDir, "remote", "get-url", "origin")
}

// DiffStats returns statistics about uncommitted changes.
// It combines staged and unstaged changes for line counts,
// and parses file status for file type counts.
func (c *Client) DiffStats() (DiffStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var stats DiffStats

	// Get unstaged diff stats
	unstaged, _ := c.cmd.Run(ctx, c.workDir, "diff", "--shortstat")
	add1, del1 := parseShortstat(unstaged)
	stats.Additions += add1
	stats.Deletions += del1

	// Get staged diff stats
	staged, _ := c.cmd.Run(ctx, c.workDir, "diff", "--shortstat", "--cached")
	add2, del2 := parseShortstat(staged)
	stats.Additions += add2
	stats.Deletions += del2

	// Get file type counts from status
	statusOut, err := c.cmd.Run(ctx, c.workDir, "status", "--porcelain")
	if err != nil {
		return stats, err
	}
	stats.NewFiles, stats.ModifiedFiles, stats.DeletedFiles = parseStatusForTypes(statusOut)

	return stats, nil
}

// parseShortstat parses output from "git diff --shortstat".
// Example: " 3 files changed, 42 insertions(+), 10 deletions(-)"
func parseShortstat(output string) (additions, deletions int) {
	if output == "" {
		return 0, 0
	}

	// Parse insertions
	if idx := strings.Index(output, " insertion"); idx > 0 {
		// Find the number before "insertion"
		start := idx - 1
		for start > 0 && output[start-1] >= '0' && output[start-1] <= '9' {
			start--
		}
		_, _ = fmt.Sscanf(output[start:idx], "%d", &additions)
	}

	// Parse deletions
	if idx := strings.Index(output, " deletion"); idx > 0 {
		// Find the number before "deletion"
		start := idx - 1
		for start > 0 && output[start-1] >= '0' && output[start-1] <= '9' {
			start--
		}
		_, _ = fmt.Sscanf(output[start:idx], "%d", &deletions)
	}

	return additions, deletions
}

// parseStatusForTypes parses "git status --porcelain" output for file type counts.
// Returns (new, modified, deleted) counts.
func parseStatusForTypes(output string) (newFiles, modified, deleted int) {
	if output == "" {
		return 0, 0, 0
	}

	for _, line := range strings.Split(output, "\n") {
		if len(line) < 2 {
			continue
		}
		// First two chars are status codes: XY
		// X = staged status, Y = unstaged status
		x, y := line[0], line[1]

		switch {
		case x == '?' && y == '?':
			// Untracked file (new)
			newFiles++
		case x == 'A':
			// Staged new file
			newFiles++
		case x == 'D' || y == 'D':
			// Deleted file
			deleted++
		case x == 'M' || y == 'M' || x == 'R' || x == 'C':
			// Modified, renamed, or copied
			modified++
		}
	}

	return newFiles, modified, deleted
}

// HeadPath returns the path to the HEAD file for cache invalidation.
func (c *Client) HeadPath() string {
	return filepath.Join(c.gitDir, "HEAD")
}

// IndexPath returns the path to the index file for cache invalidation.
func (c *Client) IndexPath() string {
	return filepath.Join(c.gitDir, "index")
}

// RefPath returns the path to the ref file for a branch.
func (c *Client) RefPath(branch string) string {
	return filepath.Join(c.gitDir, "refs", "heads", branch)
}

// ParseGitHubRepo extracts owner and repo from a GitHub remote URL.
// Supports both SSH (git@github.com:owner/repo.git) and HTTPS
// (https://github.com/owner/repo.git) formats.
// Returns empty strings and false if the URL is not a GitHub URL.
func ParseGitHubRepo(remoteURL string) (owner, repo string, ok bool) {
	// Handle SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@github.com:") {
		path := strings.TrimPrefix(remoteURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	}

	// Handle HTTPS format: https://github.com/owner/repo.git
	if strings.Contains(remoteURL, "github.com/") {
		idx := strings.Index(remoteURL, "github.com/")
		path := remoteURL[idx+len("github.com/"):]
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	}

	return "", "", false
}
