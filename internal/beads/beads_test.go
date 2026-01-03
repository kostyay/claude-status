package beads

import (
	"errors"
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
			client := NewClientWithCommander("/workdir", cmd)

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
	client := NewClientWithCommander("/workdir", cmd)

	_, err := client.GetStats()
	if err == nil {
		t.Error("GetStats() expected error for command failure")
	}
}

func TestClient_HasBeads(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		want   bool
	}{
		{
			name:   "beads available",
			output: `{"summary": {}}`,
			err:    nil,
			want:   true,
		},
		{
			name:   "beads not available",
			output: "",
			err:    errors.New("bd not found"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &mockCommander{output: []byte(tt.output), err: tt.err}
			client := NewClientWithCommander("/workdir", cmd)

			got := client.HasBeads()
			if got != tt.want {
				t.Errorf("HasBeads() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("/workdir")
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.workDir != "/workdir" {
		t.Errorf("NewClient() workDir = %q, want %q", client.workDir, "/workdir")
	}
}
