package kt

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kostyay/claude-status/internal/tasks"
)

// Client fetches kt ticket statistics.
type Client struct {
	cmd     tasks.Commander
	workDir string
}

// NewClient creates a new kt client for the given working directory.
func NewClient(workDir string) *Client {
	return &Client{
		cmd:     tasks.DefaultCommander{WorkDir: workDir},
		workDir: workDir,
	}
}

// NewClientWithCommander creates a new kt client with a custom commander.
func NewClientWithCommander(cmd tasks.Commander, workDir string) *Client {
	return &Client{
		cmd:     cmd,
		workDir: workDir,
	}
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "kt"
}

// Available checks if kt is available in the working directory.
func (c *Client) Available() bool {
	_, err := os.Stat(filepath.Join(c.workDir, ".ktickets"))
	if err != nil {
		slog.Debug("kt not available", "workDir", c.workDir, "err", err)
	}
	return err == nil
}

// statsJSON is the JSON response from kt stats --json.
type statsJSON struct {
	Open       int `json:"open"`
	InProgress int `json:"in_progress"`
	Closed     int `json:"closed"`
	Total      int `json:"total"`
}

// ticket represents a kt ticket from kt ready/blocked --json.
type ticket struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// GetStats runs kt commands and returns computed stats.
func (c *Client) GetStats() (tasks.Stats, error) {
	// Get basic stats
	output, err := c.cmd.Output("kt", "stats", "--json")
	if err != nil {
		return tasks.Stats{}, fmt.Errorf("failed to run kt stats: %w", err)
	}

	var rawStats statsJSON
	if err := json.Unmarshal(output, &rawStats); err != nil {
		return tasks.Stats{}, fmt.Errorf("failed to parse kt stats output: %w", err)
	}

	stats := tasks.Stats{
		TotalIssues:      rawStats.Total,
		OpenIssues:       rawStats.Open,
		InProgressIssues: rawStats.InProgress,
		ClosedIssues:     rawStats.Closed,
	}

	// Get ready count
	readyOutput, err := c.cmd.Output("kt", "ready", "--json")
	if err == nil {
		var readyTickets []ticket
		if json.Unmarshal(readyOutput, &readyTickets) == nil {
			stats.ReadyIssues = len(readyTickets)
		}
	}

	// Get blocked count
	blockedOutput, err := c.cmd.Output("kt", "blocked", "--json")
	if err == nil {
		var blockedTickets []ticket
		if json.Unmarshal(blockedOutput, &blockedTickets) == nil {
			stats.BlockedIssues = len(blockedTickets)
		}
	}

	return stats, nil
}

// GetNextTask returns the title of the next ready task, or empty if none.
func (c *Client) GetNextTask() (string, error) {
	output, err := c.cmd.Output("kt", "ready", "--json")
	if err != nil {
		return "", nil
	}

	var tickets []ticket
	if err := json.Unmarshal(output, &tickets); err != nil {
		return "", fmt.Errorf("failed to parse kt ready output: %w", err)
	}

	if len(tickets) == 0 {
		return "", nil
	}

	return tickets[0].Title, nil
}
