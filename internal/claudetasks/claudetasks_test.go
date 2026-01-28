package claudetasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClient_Name(t *testing.T) {
	c := NewClient("test-session")
	if c.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", c.Name(), "claude")
	}
}

func TestClient_Available_NoDir(t *testing.T) {
	c := NewClientWithDir("test-session", "/nonexistent/path")
	if c.Available() {
		t.Error("Available() = true, want false for nonexistent dir")
	}
}

func TestClient_Available_EmptySessionID(t *testing.T) {
	c := NewClient("")
	if c.Available() {
		t.Error("Available() = true, want false for empty sessionID")
	}
}

func TestClient_Available_WithTasks(t *testing.T) {
	tmpDir := t.TempDir()
	sessionDir := filepath.Join(tmpDir, "test-session")
	os.MkdirAll(sessionDir, 0755)

	c := NewClientWithDir("test-session", tmpDir)
	if !c.Available() {
		t.Error("Available() = false, want true when session dir exists")
	}
}

func TestClient_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session"
	sessionDir := filepath.Join(tmpDir, sessionID)
	os.MkdirAll(sessionDir, 0755)

	// Create test tasks
	tasks := []task{
		{ID: "1", Subject: "Task 1", Status: "completed"},
		{ID: "2", Subject: "Task 2", Status: "in_progress"},
		{ID: "3", Subject: "Task 3", Status: "pending", BlockedBy: []string{"1"}}, // not blocked (1 is completed)
		{ID: "4", Subject: "Task 4", Status: "pending", BlockedBy: []string{"2"}}, // blocked (2 is in_progress)
		{ID: "5", Subject: "Task 5", Status: "pending"},                            // ready
	}

	for i, task := range tasks {
		data, _ := json.Marshal(task)
		os.WriteFile(filepath.Join(sessionDir, task.ID+".json"), data, 0644)
		_ = i
	}

	c := NewClientWithDir(sessionID, tmpDir)
	stats, err := c.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.TotalIssues != 5 {
		t.Errorf("TotalIssues = %d, want 5", stats.TotalIssues)
	}
	if stats.ClosedIssues != 1 {
		t.Errorf("ClosedIssues = %d, want 1", stats.ClosedIssues)
	}
	if stats.InProgressIssues != 1 {
		t.Errorf("InProgressIssues = %d, want 1", stats.InProgressIssues)
	}
	if stats.OpenIssues != 4 { // in_progress + pending
		t.Errorf("OpenIssues = %d, want 4", stats.OpenIssues)
	}
	if stats.BlockedIssues != 1 {
		t.Errorf("BlockedIssues = %d, want 1", stats.BlockedIssues)
	}
	if stats.ReadyIssues != 2 { // tasks 3 and 5 are ready
		t.Errorf("ReadyIssues = %d, want 2", stats.ReadyIssues)
	}
}

func TestClient_GetNextTask_InProgress(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session"
	sessionDir := filepath.Join(tmpDir, sessionID)
	os.MkdirAll(sessionDir, 0755)

	// Create an in-progress task with activeForm
	task := task{
		ID:         "1",
		Subject:    "Main task",
		Status:     "in_progress",
		ActiveForm: "Working on main task",
	}
	data, _ := json.Marshal(task)
	os.WriteFile(filepath.Join(sessionDir, "1.json"), data, 0644)

	c := NewClientWithDir(sessionID, tmpDir)
	next, err := c.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}

	if next != "Working on main task" {
		t.Errorf("GetNextTask() = %q, want %q", next, "Working on main task")
	}
}

func TestClient_GetNextTask_Ready(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session"
	sessionDir := filepath.Join(tmpDir, sessionID)
	os.MkdirAll(sessionDir, 0755)

	// Create a pending task (ready)
	task := task{
		ID:      "1",
		Subject: "Ready task",
		Status:  "pending",
	}
	data, _ := json.Marshal(task)
	os.WriteFile(filepath.Join(sessionDir, "1.json"), data, 0644)

	c := NewClientWithDir(sessionID, tmpDir)
	next, err := c.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}

	if next != "Ready task" {
		t.Errorf("GetNextTask() = %q, want %q", next, "Ready task")
	}
}

func TestClient_GetNextTask_Blocked(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session"
	sessionDir := filepath.Join(tmpDir, sessionID)
	os.MkdirAll(sessionDir, 0755)

	// Create tasks where all are blocked
	tasks := []task{
		{ID: "1", Subject: "Blocker", Status: "in_progress"},
		{ID: "2", Subject: "Blocked task", Status: "pending", BlockedBy: []string{"1"}},
	}
	for _, task := range tasks {
		data, _ := json.Marshal(task)
		os.WriteFile(filepath.Join(sessionDir, task.ID+".json"), data, 0644)
	}

	c := NewClientWithDir(sessionID, tmpDir)
	next, err := c.GetNextTask()
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}

	// Should return empty since in_progress task has no activeForm
	if next != "" {
		t.Errorf("GetNextTask() = %q, want empty", next)
	}
}

func TestClaudeTasksDir_Default(t *testing.T) {
	// Unset CLAUDE_PROFILE
	os.Unsetenv("CLAUDE_PROFILE")

	dir := claudeTasksDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude", "tasks")

	if dir != expected {
		t.Errorf("claudeTasksDir() = %q, want %q", dir, expected)
	}
}

func TestClaudeTasksDir_WithProfile(t *testing.T) {
	os.Setenv("CLAUDE_PROFILE", "work")
	defer os.Unsetenv("CLAUDE_PROFILE")

	dir := claudeTasksDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude-work", "tasks")

	if dir != expected {
		t.Errorf("claudeTasksDir() = %q, want %q", dir, expected)
	}
}

func TestNewClient_TaskListIDEnvVar(t *testing.T) {
	os.Setenv("CLAUDE_CODE_TASK_LIST_ID", "shared-list")
	defer os.Unsetenv("CLAUDE_CODE_TASK_LIST_ID")

	c := NewClient("session-123")

	// Should use env var over sessionID
	if c.taskListID != "shared-list" {
		t.Errorf("taskListID = %q, want %q", c.taskListID, "shared-list")
	}
}

func TestNewClient_FallbackToSessionID(t *testing.T) {
	os.Unsetenv("CLAUDE_CODE_TASK_LIST_ID")

	c := NewClient("session-456")

	if c.taskListID != "session-456" {
		t.Errorf("taskListID = %q, want %q", c.taskListID, "session-456")
	}
}
