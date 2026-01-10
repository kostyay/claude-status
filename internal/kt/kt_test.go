package kt

import (
	"errors"
	"os"
	"testing"

	"github.com/kostyay/claude-status/internal/tasks"
)

// mockCommander is a test double for Commander.
type mockCommander struct {
	outputs map[string][]byte
	errs    map[string]error
}

func (m *mockCommander) Output(name string, args ...string) ([]byte, error) {
	key := name
	for _, arg := range args {
		key += " " + arg
	}
	if err, ok := m.errs[key]; ok {
		return nil, err
	}
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return nil, nil
}

func TestClient_GetStats(t *testing.T) {
	tests := []struct {
		name    string
		stats   string
		ready   string
		blocked string
		wantErr bool
		want    tasks.Stats
	}{
		{
			name:    "full stats",
			stats:   `{"open": 5, "in_progress": 2, "closed": 3, "total": 10}`,
			ready:   `[{"id":"kt-001","title":"Task 1"},{"id":"kt-002","title":"Task 2"}]`,
			blocked: `[{"id":"kt-003","title":"Task 3"}]`,
			wantErr: false,
			want: tasks.Stats{
				TotalIssues:      10,
				OpenIssues:       5,
				InProgressIssues: 2,
				ClosedIssues:     3,
				ReadyIssues:      2,
				BlockedIssues:    1,
			},
		},
		{
			name:    "empty stats",
			stats:   `{"open": 0, "in_progress": 0, "closed": 0, "total": 0}`,
			ready:   `[]`,
			blocked: `[]`,
			wantErr: false,
			want:    tasks.Stats{},
		},
		{
			name:    "invalid json",
			stats:   `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &mockCommander{
				outputs: map[string][]byte{
					"kt stats --json":   []byte(tt.stats),
					"kt ready --json":   []byte(tt.ready),
					"kt blocked --json": []byte(tt.blocked),
				},
			}
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
	cmd := &mockCommander{
		errs: map[string]error{
			"kt stats --json": errors.New("command failed"),
		},
	}
	client := NewClientWithCommander(cmd, "/test")

	_, err := client.GetStats()
	if err == nil {
		t.Error("GetStats() expected error for command failure")
	}
}

func TestClient_Available(t *testing.T) {
	t.Run("kt available", func(t *testing.T) {
		tmpDir := t.TempDir()
		ktDir := tmpDir + "/.ktickets"
		if err := os.MkdirAll(ktDir, 0755); err != nil {
			t.Fatal(err)
		}

		client := NewClient(tmpDir)
		got := client.Available()
		if !got {
			t.Error("Available() = false, want true")
		}
	})

	t.Run("kt not available", func(t *testing.T) {
		tmpDir := t.TempDir()

		client := NewClient(tmpDir)
		got := client.Available()
		if got {
			t.Error("Available() = true, want false")
		}
	})
}

func TestClient_Name(t *testing.T) {
	client := NewClient("/test")
	if client.Name() != "kt" {
		t.Errorf("Name() = %q, want %q", client.Name(), "kt")
	}
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
			output:  `[{"id": "kt-001", "title": "Fix the bug"}]`,
			wantErr: false,
			want:    "Fix the bug",
		},
		{
			name:    "multiple tasks returns first",
			output:  `[{"id": "kt-001", "title": "First task"}, {"id": "kt-002", "title": "Second task"}]`,
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
			cmd := &mockCommander{
				outputs: map[string][]byte{
					"kt ready --json": []byte(tt.output),
				},
			}
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
