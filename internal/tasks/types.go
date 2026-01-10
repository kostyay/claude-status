package tasks

import (
	"context"
	"os/exec"
	"time"
)

// Stats holds task tracking statistics.
type Stats struct {
	TotalIssues      int `json:"total_issues"`
	OpenIssues       int `json:"open_issues"`
	InProgressIssues int `json:"in_progress_issues"`
	ClosedIssues     int `json:"closed_issues"`
	BlockedIssues    int `json:"blocked_issues"`
	ReadyIssues      int `json:"ready_issues"`
}

// Provider is the interface for task tracking systems.
type Provider interface {
	// Name returns the provider name (e.g., "kt", "tk", "beads").
	Name() string

	// Available returns true if this provider is active for the working directory.
	Available() bool

	// GetStats returns task statistics.
	GetStats() (Stats, error)

	// GetNextTask returns the title of the next ready task, or empty if none.
	GetNextTask() (string, error)
}

// Commander is an interface for executing commands.
type Commander interface {
	Output(name string, args ...string) ([]byte, error)
}

// commandTimeout is the maximum time to wait for task commands.
const commandTimeout = 10 * time.Second

// DefaultCommander executes commands using os/exec in a specific directory.
type DefaultCommander struct {
	WorkDir string
}

// Output runs a command and returns its output with a timeout.
func (d DefaultCommander) Output(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if d.WorkDir != "" {
		cmd.Dir = d.WorkDir
	}
	return cmd.Output()
}
