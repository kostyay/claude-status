package tk

import (
	"errors"
	"os"
	"testing"

	"github.com/kostyay/claude-status/internal/beads"
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
		want    beads.Stats
	}{
		{
			name: "mixed statuses",
			output: `{"id":"t-001","title":"Task 1","status":"open","deps":[]}
{"id":"t-002","title":"Task 2","status":"in_progress","deps":[]}
{"id":"t-003","title":"Task 3","status":"closed","deps":[]}
{"id":"t-004","title":"Task 4","status":"open","deps":["t-001"]}`,
			wantErr: false,
			want: beads.Stats{
				TotalIssues:      4,
				OpenIssues:       2,
				InProgressIssues: 1,
				ClosedIssues:     1,
				BlockedIssues:    1, // t-004 blocked by t-001
				ReadyIssues:      2, // t-001 (no deps), t-002 (no deps)
			},
		},
		{
			name: "all ready",
			output: `{"id":"t-001","title":"Task 1","status":"open","deps":[]}
{"id":"t-002","title":"Task 2","status":"open","deps":[]}`,
			wantErr: false,
			want: beads.Stats{
				TotalIssues:      2,
				OpenIssues:       2,
				InProgressIssues: 0,
				ClosedIssues:     0,
				BlockedIssues:    0,
				ReadyIssues:      2,
			},
		},
		{
			name: "deps on closed",
			output: `{"id":"t-001","title":"Task 1","status":"closed","deps":[]}
{"id":"t-002","title":"Task 2","status":"open","deps":["t-001"]}`,
			wantErr: false,
			want: beads.Stats{
				TotalIssues:      2,
				OpenIssues:       1,
				InProgressIssues: 0,
				ClosedIssues:     1,
				BlockedIssues:    0, // t-002 dep is closed
				ReadyIssues:      1, // t-002 is ready
			},
		},
		{
			name:    "empty output",
			output:  ``,
			wantErr: false,
			want:    beads.Stats{},
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

func TestClient_HasTk(t *testing.T) {
	t.Run("tk available", func(t *testing.T) {
		tmpDir := t.TempDir()
		ticketsDir := tmpDir + "/.tickets"
		if err := os.MkdirAll(ticketsDir, 0755); err != nil {
			t.Fatal(err)
		}

		client := NewClient(tmpDir)
		got := client.HasTk()
		if !got {
			t.Error("HasTk() = false, want true")
		}
	})

	t.Run("tk not available", func(t *testing.T) {
		tmpDir := t.TempDir()

		client := NewClient(tmpDir)
		got := client.HasTk()
		if got {
			t.Error("HasTk() = true, want false")
		}
	})
}

func TestClient_HasBeads(t *testing.T) {
	// HasBeads is an alias for HasTk for interface compatibility
	tmpDir := t.TempDir()
	ticketsDir := tmpDir + "/.tickets"
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatal(err)
	}

	client := NewClient(tmpDir)
	if !client.HasBeads() {
		t.Error("HasBeads() = false, want true")
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
		name   string
		output string
		err    error
		want   string
	}{
		{
			name:   "single task",
			output: "pp-461d  [P2][open] - Fix the bug",
			want:   "Fix the bug",
		},
		{
			name:   "multiple tasks returns first",
			output: "pp-461d  [P2][open] - First task\npp-5c46  [P1][open] - Second task",
			want:   "First task",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
		{
			name:   "command error returns empty",
			output: "",
			err:    errors.New("no ready tickets"),
			want:   "",
		},
		{
			name:   "no separator",
			output: "pp-461d  [P2][open]",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &mockCommander{output: []byte(tt.output), err: tt.err}
			client := NewClientWithCommander(cmd, "/test")

			got, err := client.GetNextTask()
			if err != nil {
				t.Errorf("GetNextTask() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("GetNextTask() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComputeStats_BlockedChain(t *testing.T) {
	// Test a chain of dependencies: t-003 -> t-002 -> t-001
	tickets := []ticket{
		{ID: "t-001", Status: "open", Deps: nil},
		{ID: "t-002", Status: "open", Deps: []string{"t-001"}},
		{ID: "t-003", Status: "open", Deps: []string{"t-002"}},
	}

	stats := computeStats(tickets)

	if stats.TotalIssues != 3 {
		t.Errorf("TotalIssues = %d, want 3", stats.TotalIssues)
	}
	if stats.ReadyIssues != 1 {
		t.Errorf("ReadyIssues = %d, want 1 (only t-001)", stats.ReadyIssues)
	}
	if stats.BlockedIssues != 2 {
		t.Errorf("BlockedIssues = %d, want 2 (t-002 and t-003)", stats.BlockedIssues)
	}
}

func TestComputeStats_InProgressBlocked(t *testing.T) {
	// in_progress tickets can also be blocked
	tickets := []ticket{
		{ID: "t-001", Status: "open", Deps: nil},
		{ID: "t-002", Status: "in_progress", Deps: []string{"t-001"}},
	}

	stats := computeStats(tickets)

	if stats.InProgressIssues != 1 {
		t.Errorf("InProgressIssues = %d, want 1", stats.InProgressIssues)
	}
	if stats.BlockedIssues != 1 {
		t.Errorf("BlockedIssues = %d, want 1 (t-002 blocked by t-001)", stats.BlockedIssues)
	}
	if stats.ReadyIssues != 1 {
		t.Errorf("ReadyIssues = %d, want 1 (t-001)", stats.ReadyIssues)
	}
}
