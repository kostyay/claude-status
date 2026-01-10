package tk

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/kostyay/claude-status/internal/tasks"
)

// Client fetches tk ticket statistics.
type Client struct {
	cmd     tasks.Commander
	workDir string
}

// NewClient creates a new tk client for the given working directory.
func NewClient(workDir string) *Client {
	return &Client{
		cmd:     tasks.DefaultCommander{WorkDir: workDir},
		workDir: workDir,
	}
}

// NewClientWithCommander creates a new tk client with a custom commander.
func NewClientWithCommander(cmd tasks.Commander, workDir string) *Client {
	return &Client{
		cmd:     cmd,
		workDir: workDir,
	}
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "tk"
}

// Available checks if tk is available in the working directory.
func (c *Client) Available() bool {
	_, err := os.Stat(filepath.Join(c.workDir, ".tickets"))
	if err != nil {
		slog.Debug("tk not available", "workDir", c.workDir, "err", err)
	}
	return err == nil
}

// ticket represents a tk ticket from tk query output.
type ticket struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Deps   []string `json:"deps"`
}

// GetStats runs `tk query` and computes stats from JSONL output.
func (c *Client) GetStats() (tasks.Stats, error) {
	output, err := c.cmd.Output("tk", "query")
	if err != nil {
		return tasks.Stats{}, fmt.Errorf("failed to run tk query: %w", err)
	}

	tickets, err := parseJSONL(output)
	if err != nil {
		return tasks.Stats{}, fmt.Errorf("failed to parse tk query output: %w", err)
	}

	return computeStats(tickets), nil
}

// parseJSONL parses JSONL output (one JSON object per line).
func parseJSONL(data []byte) ([]ticket, error) {
	var tickets []ticket
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var t ticket
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			return nil, fmt.Errorf("failed to parse ticket JSON: %w", err)
		}
		tickets = append(tickets, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan JSONL: %w", err)
	}
	return tickets, nil
}

// computeStats calculates stats from tickets.
// Ready = (open OR in_progress) AND (deps empty OR all deps closed)
// Blocked = (open OR in_progress) AND (has dep that is not closed)
func computeStats(tickets []ticket) tasks.Stats {
	// Build status map for dep resolution
	statusMap := make(map[string]string)
	for _, t := range tickets {
		statusMap[t.ID] = t.Status
	}

	var stats tasks.Stats
	stats.TotalIssues = len(tickets)

	for _, t := range tickets {
		switch t.Status {
		case "open":
			stats.OpenIssues++
		case "in_progress":
			stats.InProgressIssues++
		case "closed":
			stats.ClosedIssues++
		}

		// Only compute ready/blocked for active tickets
		if t.Status != "open" && t.Status != "in_progress" {
			continue
		}

		if isBlocked(t, statusMap) {
			stats.BlockedIssues++
		} else {
			stats.ReadyIssues++
		}
	}

	return stats
}

// isBlocked returns true if ticket has any unresolved dependency.
func isBlocked(t ticket, statusMap map[string]string) bool {
	for _, depID := range t.Deps {
		depStatus, exists := statusMap[depID]
		if !exists {
			// Unknown dep - consider blocked
			continue
		}
		if depStatus != "closed" {
			return true
		}
	}
	return false
}

// GetNextTask returns the title of the next ready task, or empty if none.
// Parses output format: `pp-461d  [P2][open] - Task title here`
func (c *Client) GetNextTask() (string, error) {
	output, err := c.cmd.Output("tk", "ready")
	if err != nil {
		// tk ready exits non-zero when no ready tickets
		return "", nil
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", nil
	}

	// Parse: `pp-461d  [P2][open] - Task title`
	// Find " - " separator and return everything after
	if idx := strings.Index(lines[0], " - "); idx != -1 {
		return strings.TrimSpace(lines[0][idx+3:]), nil
	}

	return "", nil
}
