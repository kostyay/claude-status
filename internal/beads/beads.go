package beads

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kostyay/claude-status/internal/tasks"
)

// statsResponse is the full JSON response from bd stats --json.
type statsResponse struct {
	Summary tasks.Stats `json:"summary"`
}

// Client fetches beads statistics.
type Client struct {
	cmd     tasks.Commander
	workDir string
}

// NewClient creates a new beads client for the given working directory.
func NewClient(workDir string) *Client {
	return &Client{
		cmd:     tasks.DefaultCommander{WorkDir: workDir},
		workDir: workDir,
	}
}

// NewClientWithCommander creates a new beads client with a custom commander.
func NewClientWithCommander(cmd tasks.Commander, workDir string) *Client {
	return &Client{
		cmd:     cmd,
		workDir: workDir,
	}
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "beads"
}

// Available checks if beads is available in the working directory.
func (c *Client) Available() bool {
	_, err := os.Stat(filepath.Join(c.workDir, ".beads"))
	if err != nil {
		slog.Debug("beads not available", "workDir", c.workDir, "err", err)
	}
	return err == nil
}

// GetStats runs `bd stats --json` and returns the parsed stats.
func (c *Client) GetStats() (tasks.Stats, error) {
	output, err := c.cmd.Output("bd", "stats", "--json")
	if err != nil {
		return tasks.Stats{}, fmt.Errorf("failed to run bd stats: %w", err)
	}

	var resp statsResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return tasks.Stats{}, fmt.Errorf("failed to parse bd stats output: %w", err)
	}

	return resp.Summary, nil
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
