package beads

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Stats holds the beads statistics summary.
type Stats struct {
	TotalIssues      int `json:"total_issues"`
	OpenIssues       int `json:"open_issues"`
	InProgressIssues int `json:"in_progress_issues"`
	ClosedIssues     int `json:"closed_issues"`
	BlockedIssues    int `json:"blocked_issues"`
	ReadyIssues      int `json:"ready_issues"`
}

// statsResponse is the full JSON response from bd stats --json.
type statsResponse struct {
	Summary Stats `json:"summary"`
}

// Commander is an interface for executing commands.
type Commander interface {
	Output(name string, args ...string) ([]byte, error)
}

// DefaultCommander executes commands using os/exec in a specific directory.
type DefaultCommander struct {
	workDir string
}

// commandTimeout is the maximum time to wait for beads commands.
// This must be longer than bd's daemon startup timeout (5s) plus execution time.
const commandTimeout = 10 * time.Second

// Output runs a command and returns its output with a timeout.
func (d DefaultCommander) Output(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if d.workDir != "" {
		cmd.Dir = d.workDir
	}
	return cmd.Output()
}

// Client fetches beads statistics.
type Client struct {
	cmd     Commander
	workDir string
}

// NewClient creates a new beads client for the given working directory.
func NewClient(workDir string) *Client {
	return &Client{
		cmd:     DefaultCommander{workDir: workDir},
		workDir: workDir,
	}
}

// NewClientWithCommander creates a new beads client with a custom commander.
func NewClientWithCommander(cmd Commander, workDir string) *Client {
	return &Client{
		cmd:     cmd,
		workDir: workDir,
	}
}

// GetStats runs `bd stats --json` and returns the parsed stats.
func (c *Client) GetStats() (Stats, error) {
	output, err := c.cmd.Output("bd", "stats", "--json")
	if err != nil {
		return Stats{}, fmt.Errorf("failed to run bd stats: %w", err)
	}

	var resp statsResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return Stats{}, fmt.Errorf("failed to parse bd stats output: %w", err)
	}

	return resp.Summary, nil
}

// HasBeads checks if the beads system is available in the working directory.
// Uses directory check instead of running bd stats to avoid duplicate subprocess calls.
func (c *Client) HasBeads() bool {
	beadsDir := filepath.Join(c.workDir, ".beads")
	_, err := os.Stat(beadsDir)
	if err != nil {
		slog.Debug("beads not available", "workDir", c.workDir, "err", err)
		return false
	}
	return true
}

// Issue represents a beads issue from bd ready --json.
type Issue struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// GetNextTask returns the title of the next ready task, or empty if none.
func (c *Client) GetNextTask() (string, error) {
	output, err := c.cmd.Output("bd", "ready", "--json")
	if err != nil {
		return "", fmt.Errorf("failed to run bd ready: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal(output, &issues); err != nil {
		return "", fmt.Errorf("failed to parse bd ready output: %w", err)
	}

	if len(issues) == 0 {
		return "", nil
	}

	return issues[0].Title, nil
}
