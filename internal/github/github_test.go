package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockTokenGetter is a test double for TokenGetter.
type mockTokenGetter struct {
	token string
	err   error
}

func (m *mockTokenGetter) GetToken() (string, error) {
	return m.token, m.err
}

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClientWithDeps("build_and_test", &http.Client{Timeout: 5 * time.Second},
		&mockTokenGetter{token: "test-token"})
	if err != nil {
		t.Fatalf("NewClientWithDeps() error = %v", err)
	}
	client.SetBaseURL(server.URL)

	return server, client
}

func TestGetBuildStatus_Success(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "success"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusSuccess {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusSuccess)
	}
}

func TestGetBuildStatus_Failure(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "failure"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusFailure {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusFailure)
	}
}

func TestGetBuildStatus_Pending(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "in_progress", "conclusion": ""},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusPending {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusPending)
	}
}

func TestGetBuildStatus_Queued(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "queued", "conclusion": ""},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusPending {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusPending)
	}
}

func TestGetBuildStatus_Cancelled(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "cancelled"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusFailure {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusFailure)
	}
}

func TestGetBuildStatus_NoWorkflow(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "other_workflow", "path": ".github/workflows/other.yml"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetBuildStatus("owner", "repo", "main")
	if err == nil {
		t.Error("GetBuildStatus() expected error for missing workflow")
	}
}

func TestGetBuildStatus_NoRuns(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetBuildStatus("owner", "repo", "main")
	if err == nil {
		t.Error("GetBuildStatus() expected error for no runs")
	}
}

func TestGetBuildStatus_NotFound(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetBuildStatus("owner", "repo", "main")
	if err == nil {
		t.Error("GetBuildStatus() expected error for not found")
	}
}

func TestGetBuildStatus_NetworkError(t *testing.T) {
	// Use a URL that will fail to connect
	client, err := NewClientWithDeps("build_and_test", &http.Client{Timeout: 100 * time.Millisecond},
		&mockTokenGetter{token: "test-token"})
	if err != nil {
		t.Fatalf("NewClientWithDeps() error = %v", err)
	}
	client.SetBaseURL("http://127.0.0.1:1") // Port 1 should fail

	_, err = client.GetBuildStatus("owner", "repo", "main")
	if err == nil {
		t.Error("GetBuildStatus() expected network error")
	}
}

func TestStatusToEmoji(t *testing.T) {
	tests := []struct {
		status BuildStatus
		want   string
	}{
		{StatusSuccess, "✅"},
		{StatusFailure, "❌"},
		{StatusPending, "🔄"},
		{StatusError, "⚠️"},
		{BuildStatus("unknown"), "⚠️"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := StatusToEmoji(tt.status)
			if got != tt.want {
				t.Errorf("StatusToEmoji(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestNewClientWithDeps(t *testing.T) {
	tokenGetter := &mockTokenGetter{token: "test-token"}
	httpClient := &http.Client{Timeout: 5 * time.Second}

	client, err := NewClientWithDeps("build_and_test", httpClient, tokenGetter)
	if err != nil {
		t.Fatalf("NewClientWithDeps() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClientWithDeps() returned nil")
	}
}

func TestNewClientWithDeps_TokenError(t *testing.T) {
	tokenGetter := &mockTokenGetter{err: errors.New("no token")}
	httpClient := &http.Client{Timeout: 5 * time.Second}

	_, err := NewClientWithDeps("build_and_test", httpClient, tokenGetter)
	if err == nil {
		t.Error("NewClientWithDeps() expected error when token getter fails")
	}
}

func TestNewClientWithDeps_EmptyToken(t *testing.T) {
	tokenGetter := &mockTokenGetter{token: ""}
	httpClient := &http.Client{Timeout: 5 * time.Second}

	_, err := NewClientWithDeps("build_and_test", httpClient, tokenGetter)
	if err == nil {
		t.Error("NewClientWithDeps() expected error for empty token")
	}
	if err != ErrEmptyToken {
		t.Errorf("NewClientWithDeps() error = %v, want %v", err, ErrEmptyToken)
	}
}

func TestWorkflowFoundByPath(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 456, "name": "CI", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/456/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "success"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusSuccess {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusSuccess)
	}
}

func TestWorkflowFoundByYamlPath(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 789, "name": "CI", "path": ".github/workflows/build_and_test.yaml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/789/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "success"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusSuccess {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusSuccess)
	}
}

func TestGetBuildStatus_MalformedWorkflowsJSON(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			w.Write([]byte("not valid json{"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetBuildStatus("owner", "repo", "main")
	if err == nil {
		t.Error("GetBuildStatus() expected error for malformed workflows JSON")
	}
}

func TestGetBuildStatus_MalformedRunsJSON(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			w.Write([]byte("not valid json{"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetBuildStatus("owner", "repo", "main")
	if err == nil {
		t.Error("GetBuildStatus() expected error for malformed runs JSON")
	}
}

func TestGetBuildStatus_Waiting(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "waiting", "conclusion": ""},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusPending {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusPending)
	}
}

func TestGetBuildStatus_TimedOut(t *testing.T) {
	_, client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/actions/workflows" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflows": []map[string]interface{}{
					{"id": 123, "name": "build_and_test", "path": ".github/workflows/build_and_test.yml"},
				},
			})
			return
		}
		if r.URL.Path == "/repos/owner/repo/actions/workflows/123/runs" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"workflow_runs": []map[string]interface{}{
					{"status": "completed", "conclusion": "timed_out"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	status, err := client.GetBuildStatus("owner", "repo", "main")
	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}
	if status != StatusFailure {
		t.Errorf("GetBuildStatus() = %q, want %q", status, StatusFailure)
	}
}
