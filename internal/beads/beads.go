package beads

import (
	"encoding/json"
	"fmt"
	"os/exec"
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

// Output runs a command and returns its output.
func (d DefaultCommander) Output(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if d.workDir != "" {
		cmd.Dir = d.workDir
	}
	return cmd.Output()
}

// Client fetches beads statistics.
type Client struct {
	workDir string
	cmd     Commander
}

// NewClient creates a new beads client for the given working directory.
func NewClient(workDir string) *Client {
	return &Client{
		workDir: workDir,
		cmd:     DefaultCommander{workDir: workDir},
	}
}

// NewClientWithCommander creates a new beads client with a custom commander.
func NewClientWithCommander(workDir string, cmd Commander) *Client {
	return &Client{
		workDir: workDir,
		cmd:     cmd,
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
func (c *Client) HasBeads() bool {
	_, err := c.cmd.Output("bd", "stats", "--json")
	return err == nil
}
