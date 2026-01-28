package claudetasks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kostyay/claude-status/internal/tasks"
)

// Client reads Claude Code's built-in task files.
type Client struct {
	taskListID string // from CLAUDE_CODE_TASK_LIST_ID or sessionID
	tasksDir   string
}

// task represents a Claude task JSON file.
type task struct {
	ID          string   `json:"id"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	ActiveForm  string   `json:"activeForm"`
	Status      string   `json:"status"` // pending, in_progress, completed
	Blocks      []string `json:"blocks"`
	BlockedBy   []string `json:"blockedBy"`
}

// NewClient creates a new claude tasks client.
// Uses CLAUDE_CODE_TASK_LIST_ID env var if set, otherwise falls back to sessionID.
func NewClient(sessionID string) *Client {
	taskListID := os.Getenv("CLAUDE_CODE_TASK_LIST_ID")
	if taskListID == "" {
		taskListID = sessionID
	}
	return &Client{
		taskListID: taskListID,
		tasksDir:   claudeTasksDir(),
	}
}

// NewClientWithDir creates a client with a custom tasks directory (for testing).
func NewClientWithDir(taskListID, tasksDir string) *Client {
	return &Client{
		taskListID: taskListID,
		tasksDir:   tasksDir,
	}
}

// claudeTasksDir returns the path to Claude's tasks directory.
// Respects CLAUDE_PROFILE env var: ~/.claude-{profile}/tasks or ~/.claude/tasks
func claudeTasksDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	claudeDir := ".claude"
	if profile := os.Getenv("CLAUDE_PROFILE"); profile != "" {
		claudeDir = ".claude-" + profile
	}

	return filepath.Join(home, claudeDir, "tasks")
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "claude"
}

// taskDir returns the path to this task list's directory.
func (c *Client) taskDir() string {
	if c.tasksDir == "" || c.taskListID == "" {
		return ""
	}
	return filepath.Join(c.tasksDir, c.taskListID)
}

// Available checks if claude tasks exist for this session.
func (c *Client) Available() bool {
	dir := c.taskDir()
	if dir == "" {
		return false
	}

	info, err := os.Stat(dir)
	if err != nil {
		slog.Debug("claude tasks not available", "dir", dir, "err", err)
		return false
	}
	return info.IsDir()
}

// readTasks reads all task files from the session directory.
func (c *Client) readTasks() ([]task, error) {
	dir := c.taskDir()
	if dir == "" {
		return nil, fmt.Errorf("no session directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks dir: %w", err)
	}

	var allTasks []task
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			slog.Debug("failed to read task file", "file", entry.Name(), "err", err)
			continue
		}

		var t task
		if err := json.Unmarshal(data, &t); err != nil {
			slog.Debug("failed to parse task file", "file", entry.Name(), "err", err)
			continue
		}

		allTasks = append(allTasks, t)
	}

	return allTasks, nil
}

// GetStats returns task statistics.
func (c *Client) GetStats() (tasks.Stats, error) {
	allTasks, err := c.readTasks()
	if err != nil {
		return tasks.Stats{}, err
	}

	// Build set of completed task IDs for blocked calculation
	completedIDs := make(map[string]bool)
	for _, t := range allTasks {
		if t.Status == "completed" {
			completedIDs[t.ID] = true
		}
	}

	var stats tasks.Stats
	stats.TotalIssues = len(allTasks)

	for _, t := range allTasks {
		switch t.Status {
		case "completed":
			stats.ClosedIssues++
		case "in_progress":
			stats.InProgressIssues++
			stats.OpenIssues++
		default: // pending
			stats.OpenIssues++

			// Check if blocked (has unresolved blockedBy)
			blocked := false
			for _, blockerID := range t.BlockedBy {
				if !completedIDs[blockerID] {
					blocked = true
					break
				}
			}

			if blocked {
				stats.BlockedIssues++
			} else {
				stats.ReadyIssues++
			}
		}
	}

	return stats, nil
}

// GetNextTask returns the subject of the next ready task.
func (c *Client) GetNextTask() (string, error) {
	allTasks, err := c.readTasks()
	if err != nil {
		return "", err
	}

	// Build set of completed task IDs
	completedIDs := make(map[string]bool)
	for _, t := range allTasks {
		if t.Status == "completed" {
			completedIDs[t.ID] = true
		}
	}

	// Find first in_progress task (has activeForm)
	for _, t := range allTasks {
		if t.Status == "in_progress" && t.ActiveForm != "" {
			return t.ActiveForm, nil
		}
	}

	// Find first ready (pending, not blocked) task
	for _, t := range allTasks {
		if t.Status != "" && t.Status != "pending" {
			continue
		}

		// Check if blocked
		blocked := false
		for _, blockerID := range t.BlockedBy {
			if !completedIDs[blockerID] {
				blocked = true
				break
			}
		}

		if !blocked && t.Subject != "" {
			return t.Subject, nil
		}
	}

	return "", nil
}
