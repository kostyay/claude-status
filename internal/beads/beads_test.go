package beads

import (
	"errors"
	"os"
	"testing"
)

// mockCommander is a test double for Commander.
type mockCommander struct {
	output []byte
	err    error
}

func (m *mockCommander) Output(name string, args ...string) ([]byte, error) {
	return m.output, m.err
}

func TestClient_GetStats(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
		want    Stats
	}{
		{
			name: "valid stats",
			output: `{
				"summary": {
					"total_issues": 10,
					"open_issues": 5,
					"in_progress_issues": 2,
					"closed_issues": 3,
					"blocked_issues": 1,
					"ready_issues": 4
				}
			}`,
			wantErr: false,
			want: Stats{
				TotalIssues:      10,
				OpenIssues:       5,
				InProgressIssues: 2,
				ClosedIssues:     3,
				BlockedIssues:    1,
				ReadyIssues:      4,
			},
		},
		{
			name: "empty stats",
			output: `{
				"summary": {
					"total_issues": 0,
					"open_issues": 0,
					"in_progress_issues": 0,
					"closed_issues": 0,
					"blocked_issues": 0,
					"ready_issues": 0
				}
			}`,
			wantErr: false,
			want:    Stats{},
		},
		{
			name:    "invalid json",
			output:  `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &mockCommander{output: []byte(tt.output)}
			client := NewClientWithCommander(cmd, "/test")

			got, err := client.GetStats()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got != tt.want {
					t.Errorf("GetStats() = %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}

func TestClient_GetStats_CommandError(t *testing.T) {
	cmd := &mockCommander{err: errors.New("command failed")}
	client := NewClientWithCommander(cmd, "/test")

	_, err := client.GetStats()
	if err == nil {
		t.Error("GetStats() expected error for command failure")
	}
}

func TestClient_HasBeads(t *testing.T) {
	t.Run("beads available", func(t *testing.T) {
		// Create temp directory with .beads folder
		tmpDir := t.TempDir()
		beadsDir := tmpDir + "/.beads"
		if err := os.MkdirAll(beadsDir, 0755); err != nil {
			t.Fatal(err)
		}

		client := NewClient(tmpDir)
		got := client.HasBeads()
		if !got {
			t.Error("HasBeads() = false, want true")
		}
	})

	t.Run("beads not available", func(t *testing.T) {
		// Create temp directory without .beads folder
		tmpDir := t.TempDir()

		client := NewClient(tmpDir)
		got := client.HasBeads()
		if got {
			t.Error("HasBeads() = true, want false")
		}
	})
}

func TestNewClient(t *testing.T) {
	client := NewClient("/workdir")
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.cmd == nil {
		t.Error("NewClient() cmd is nil")
	}
}

func TestClient_GetNextTask(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
		want    string
	}{
		{
			name:    "single task",
			output:  `[{"id": "task-1", "title": "Fix the bug"}]`,
			wantErr: false,
			want:    "Fix the bug",
		},
		{
			name:    "multiple tasks returns first",
			output:  `[{"id": "task-1", "title": "First task"}, {"id": "task-2", "title": "Second task"}]`,
			wantErr: false,
			want:    "First task",
		},
		{
			name:    "empty list",
			output:  `[]`,
			wantErr: false,
			want:    "",
		},
		{
			name:    "invalid json",
			output:  `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &mockCommander{output: []byte(tt.output)}
			client := NewClientWithCommander(cmd, "/test")

			got, err := client.GetNextTask()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNextTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetNextTask() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClient_GetNextTask_CommandError(t *testing.T) {
	cmd := &mockCommander{err: errors.New("command failed")}
	client := NewClientWithCommander(cmd, "/test")

	_, err := client.GetNextTask()
	if err == nil {
		t.Error("GetNextTask() expected error for command failure")
	}
}
