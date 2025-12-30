package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// Default timeout for GitHub API requests.
const apiTimeout = 5 * time.Second

// HTTPClient is an interface for HTTP operations, allowing for testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// TokenGetter is an interface for getting GitHub tokens.
type TokenGetter interface {
	GetToken() (string, error)
}

// GHCLITokenGetter gets tokens from the gh CLI.
type GHCLITokenGetter struct{}

// GetToken gets the GitHub token from the gh CLI.
func (g *GHCLITokenGetter) GetToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Client provides GitHub API operations.
type Client struct {
	token      string
	httpClient HTTPClient
	workflow   string
	baseURL    string
}

// NewClient creates a new GitHub client.
func NewClient(workflow string) (*Client, error) {
	return NewClientWithDeps(workflow, &http.Client{Timeout: 5 * time.Second}, &GHCLITokenGetter{})
}

// NewClientWithDeps creates a new GitHub client with injected dependencies.
func NewClientWithDeps(workflow string, httpClient HTTPClient, tokenGetter TokenGetter) (*Client, error) {
	token, err := tokenGetter.GetToken()
	if err != nil {
		return nil, err
	}

	if token == "" {
		return nil, ErrEmptyToken
	}

	return &Client{
		token:      token,
		httpClient: httpClient,
		workflow:   workflow,
		baseURL:    "https://api.github.com",
	}, nil
}

// ErrEmptyToken is returned when an empty token is provided.
var ErrEmptyToken = errors.New("github token cannot be empty")

// NewClientWithToken creates a new GitHub client with an explicit token.
// Returns an error if the token is empty.
func NewClientWithToken(workflow, token string, httpClient HTTPClient) (*Client, error) {
	if token == "" {
		return nil, ErrEmptyToken
	}
	return &Client{
		token:      token,
		httpClient: httpClient,
		workflow:   workflow,
		baseURL:    "https://api.github.com",
	}, nil
}

// SetBaseURL sets the base URL for API requests (useful for testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// BuildStatus represents the status of a GitHub workflow run.
type BuildStatus string

const (
	StatusSuccess BuildStatus = "success"
	StatusFailure BuildStatus = "failure"
	StatusPending BuildStatus = "pending"
	StatusError   BuildStatus = "error"
)

// GetBuildStatus fetches the latest build status for the configured workflow.
func (c *Client) GetBuildStatus(owner, repo, branch string) (BuildStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	return c.GetBuildStatusWithContext(ctx, owner, repo, branch)
}

// GetBuildStatusWithContext fetches the latest build status with a custom context.
func (c *Client) GetBuildStatusWithContext(ctx context.Context, owner, repo, branch string) (BuildStatus, error) {
	// First, get the workflow ID
	workflowID, err := c.getWorkflowID(ctx, owner, repo)
	if err != nil {
		return StatusError, err
	}

	// Then get the latest run for this workflow and branch
	return c.getLatestRunStatus(ctx, owner, repo, workflowID, branch)
}

func (c *Client) getWorkflowID(ctx context.Context, owner, repo string) (int64, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/actions/workflows", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return 0, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GitHub API request to %s returned %d", apiURL, resp.StatusCode)
	}

	var result struct {
		Workflows []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"workflows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode workflows response: %w", err)
	}

	workflowLower := strings.ToLower(c.workflow)
	for _, w := range result.Workflows {
		pathLower := strings.ToLower(w.Path)
		if strings.EqualFold(w.Name, c.workflow) ||
			strings.HasSuffix(pathLower, workflowLower+".yml") ||
			strings.HasSuffix(pathLower, workflowLower+".yaml") {
			return w.ID, nil
		}
	}

	return 0, fmt.Errorf("workflow %q not found", c.workflow)
}

func (c *Client) getLatestRunStatus(ctx context.Context, owner, repo string, workflowID int64, branch string) (BuildStatus, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/actions/workflows/%d/runs?branch=%s&per_page=1",
		c.baseURL, owner, repo, workflowID, url.QueryEscape(branch))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return StatusError, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return StatusError, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return StatusError, fmt.Errorf("GitHub API request to %s returned %d", apiURL, resp.StatusCode)
	}

	var result struct {
		WorkflowRuns []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"workflow_runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return StatusError, fmt.Errorf("failed to decode workflow runs response: %w", err)
	}

	if len(result.WorkflowRuns) == 0 {
		return StatusError, fmt.Errorf("no workflow runs found")
	}

	run := result.WorkflowRuns[0]

	switch run.Status {
	case "completed":
		switch run.Conclusion {
		case "success":
			return StatusSuccess, nil
		case "failure", "timed_out", "cancelled":
			return StatusFailure, nil
		default:
			return StatusError, nil
		}
	case "queued", "in_progress", "waiting":
		return StatusPending, nil
	default:
		return StatusError, nil
	}
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

// StatusToEmoji converts a BuildStatus to an emoji string.
func StatusToEmoji(status BuildStatus) string {
	switch status {
	case StatusSuccess:
		return "✅"
	case StatusFailure:
		return "❌"
	case StatusPending:
		return "🔄"
	default:
		return "⚠️"
	}
}
