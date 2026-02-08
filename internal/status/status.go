package status

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kostyay/claude-status/internal/cache"
	"github.com/kostyay/claude-status/internal/config"
	"github.com/kostyay/claude-status/internal/git"
	"github.com/kostyay/claude-status/internal/github"
	"github.com/kostyay/claude-status/internal/tasks"
	"github.com/kostyay/claude-status/internal/template"
	"github.com/kostyay/claude-status/internal/tokens"

	// Task providers (priority controlled by RegisterWithPriority, not import order)
	_ "github.com/kostyay/claude-status/internal/beads"
	_ "github.com/kostyay/claude-status/internal/claudetasks"
	_ "github.com/kostyay/claude-status/internal/kt"
	_ "github.com/kostyay/claude-status/internal/tk"
)

// Input represents the JSON input from stdin.
type Input struct {
	Model          ModelInfo          `json:"model"`
	Workspace      WorkspaceInfo      `json:"workspace"`
	Version        string             `json:"version"`
	SessionID      string             `json:"session_id"`
	TranscriptPath string             `json:"transcript_path"`
	ContextWindow  *ContextWindowInfo `json:"context_window"`
	Cost           *CostInfo          `json:"cost"`
	Exceeds200k    bool               `json:"exceeds_200k_tokens"`
}

// ModelInfo contains information about the model.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// WorkspaceInfo contains workspace information.
type WorkspaceInfo struct {
	CurrentDir string `json:"current_dir"`
}

// ContextWindowInfo contains context window data provided by Claude Code.
type ContextWindowInfo struct {
	UsedPercentage      *float64          `json:"used_percentage"`
	RemainingPercentage *float64          `json:"remaining_percentage"`
	ContextWindowSize   int64             `json:"context_window_size"`
	TotalInputTokens    int64             `json:"total_input_tokens"`
	TotalOutputTokens   int64             `json:"total_output_tokens"`
	CurrentUsage        *CurrentUsageInfo `json:"current_usage"`
}

// CurrentUsageInfo contains token counts from the last API call.
type CurrentUsageInfo struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// CostInfo contains session cost data provided by Claude Code.
type CostInfo struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMS    int64   `json:"total_duration_ms"`
	TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
}

// GitProvider is an interface for git operations.
type GitProvider interface {
	Branch() (string, error)
	Status() (string, error)
	DiffStats() (git.DiffStats, error)
	RemoteURL() (string, error)
	GitDir() string
	HeadPath() string
	IndexPath() string
	RefPath(branch string) string
}

// GitHubProvider is an interface for GitHub operations.
type GitHubProvider interface {
	GetBuildStatus(owner, repo, branch string) (github.BuildStatus, error)
}

// CacheProvider is an interface for cache operations.
type CacheProvider interface {
	GetGitBranch(headPath string, fetchFn func() (string, error)) (string, error)
	GetGitStatus(indexPath string, fetchFn func() (string, error)) (string, error)
	GetGitDiffStats(indexPath string, fetchFn func() (git.DiffStats, error)) (git.DiffStats, error)
	GetGitHubBuild(refPath, branch string, ttl time.Duration, fetchFn func() (github.BuildStatus, error)) (github.BuildStatus, error)
	GetTaskStats(workDir string, ttl time.Duration, fetchFn func() (tasks.Stats, error)) (tasks.Stats, error)
	GetNextTask(workDir string, ttl time.Duration, fetchFn func() (string, error)) (string, error)
	EnsureDir() error
}

// Builder constructs StatusData from various sources.
type Builder struct {
	config       *config.Config
	cache        CacheProvider
	git          GitProvider
	gh           GitHubProvider
	taskProvider tasks.Provider
	workDir      string
	prefix       string // User-provided prefix text
	prefixColor  string // ANSI color code for prefix
}

// ErrNilConfig is returned when a nil config is provided to NewBuilder.
var ErrNilConfig = errors.New("config cannot be nil")

// NewBuilder creates a new status builder.
func NewBuilder(cfg *config.Config, workDir, sessionID string) (*Builder, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}

	// Initialize cache
	cacheManager := cache.NewManager(config.CacheDir())
	if err := cacheManager.EnsureDir(); err != nil {
		return nil, err
	}

	b := &Builder{
		config:  cfg,
		cache:   cacheManager,
		workDir: workDir,
	}

	// Try to initialize git client (may fail if not in git repo)
	if gitClient, err := git.NewClient(workDir); err == nil {
		b.git = gitClient
	} else {
		slog.Debug("git client initialization skipped", "workDir", workDir, "err", err)
	}

	// Initialize task tracker via registry (priority: claude > kt > tk > beads)
	b.taskProvider = tasks.SelectProvider(workDir, sessionID)

	return b, nil
}

// NewBuilderWithDeps creates a new status builder with injected dependencies.
func NewBuilderWithDeps(cfg *config.Config, cache CacheProvider, git GitProvider, gh GitHubProvider, taskProvider tasks.Provider, workDir string) *Builder {
	return &Builder{
		config:       cfg,
		cache:        cache,
		git:          git,
		gh:           gh,
		taskProvider: taskProvider,
		workDir:      workDir,
	}
}

// Build constructs StatusData from the input.
func (b *Builder) Build(input Input) template.StatusData {
	data := template.StatusData{
		Prefix:      b.prefix,
		PrefixColor: b.prefixColor,
		Model:       input.Model.DisplayName,
		Dir:         filepath.Base(input.Workspace.CurrentDir),
		Version:     input.Version,
	}

	if data.Model == "" {
		data.Model = "Claude"
	}

	// Parse token metrics from transcript
	b.populateTokenMetrics(&data, input)

	// Populate cost metrics from Claude Code stdin
	b.populateCostMetrics(&data, input)

	// Get task stats (cached with TTL) - independent of git
	b.fetchTaskStats(&data)

	if b.git == nil {
		return data
	}

	// Get git branch (cached)
	branch, err := b.cache.GetGitBranch(b.git.HeadPath(), b.git.Branch)
	if err == nil && branch != "" {
		data.GitBranch = branch
	}

	// Get git status (cached)
	status, err := b.cache.GetGitStatus(b.git.IndexPath(), b.git.Status)
	if err == nil && status != "" {
		data.GitStatus = status
	}

	// Get git diff stats (cached)
	diffStats, err := b.cache.GetGitDiffStats(b.git.IndexPath(), b.git.DiffStats)
	if err == nil {
		b.populateDiffStats(&data, diffStats)
	}

	// Get GitHub build status (cached with TTL)
	if data.GitBranch != "" {
		b.fetchGitHubStatus(&data, data.GitBranch)
	}

	return data
}

// populateTokenMetrics populates token metrics from context_window (preferred) or transcript (fallback).
func (b *Builder) populateTokenMetrics(data *template.StatusData, input Input) {
	// Prefer context_window data from Claude Code stdin when available
	if input.ContextWindow != nil && input.ContextWindow.UsedPercentage != nil {
		b.populateFromContextWindow(data, input)
		return
	}

	// Fall back to transcript parsing
	b.populateFromTranscript(data, input)
}

// autoCompactScaleFactor converts a percentage of the full context window to a percentage
// of the usable context (80% auto-compact threshold). This matches the calculation in
// tokens.GetContextConfig where UsableTokens = MaxTokens * 0.8.
const autoCompactScaleFactor = 1.0 / 0.8 // 1.25

// populateFromContextWindow uses pre-calculated context data from Claude Code.
func (b *Builder) populateFromContextWindow(data *template.StatusData, input Input) {
	cw := input.ContextWindow

	data.ContextPct = *cw.UsedPercentage
	data.ContextWindowSize = cw.ContextWindowSize

	// Calculate usable percentage (against 80% auto-compact threshold)
	usablePct := *cw.UsedPercentage * autoCompactScaleFactor
	if usablePct > 100 {
		usablePct = 100
	}
	data.ContextPctUse = usablePct

	// Populate token metrics from current_usage if available
	if cw.CurrentUsage != nil {
		u := cw.CurrentUsage
		data.TokensInput = u.InputTokens
		data.TokensOutput = u.OutputTokens
		data.TokensCached = u.CacheReadInputTokens + u.CacheCreationInputTokens
		data.ContextLength = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
	}

	// Use cumulative totals for total tokens
	data.TokensTotal = cw.TotalInputTokens + cw.TotalOutputTokens

	data.Exceeds200k = input.Exceeds200k
}

// populateFromTranscript parses the transcript JSONL file for token metrics.
func (b *Builder) populateFromTranscript(data *template.StatusData, input Input) {
	if input.TranscriptPath == "" {
		return
	}

	metrics, err := tokens.ParseTranscript(input.TranscriptPath)
	if err != nil {
		slog.Debug("failed to parse transcript", "path", input.TranscriptPath, "err", err)
		return
	}

	// Get context config based on model
	ctxCfg := tokens.GetContextConfig(input.Model.ID)

	// Populate raw values (formatting is done in templates via fmtTokens/fmtPct)
	data.TokensInput = metrics.InputTokens
	data.TokensOutput = metrics.OutputTokens
	data.TokensCached = metrics.CachedTokens
	data.TokensTotal = metrics.TotalTokens
	data.ContextLength = metrics.ContextLength
	data.ContextPct = metrics.ContextPercentage(ctxCfg)
	data.ContextPctUse = metrics.ContextPercentageUsable(ctxCfg)
}

// populateCostMetrics populates cost data from Claude Code stdin.
func (b *Builder) populateCostMetrics(data *template.StatusData, input Input) {
	if input.Cost == nil {
		return
	}

	data.CostUSD = input.Cost.TotalCostUSD
	data.DurationMS = input.Cost.TotalDurationMS
	data.APIDurationMS = input.Cost.TotalAPIDurationMS
	data.SessionLinesAdded = input.Cost.TotalLinesAdded
	data.SessionLinesRemoved = input.Cost.TotalLinesRemoved
}

// populateDiffStats populates git diff statistics into StatusData.
func (b *Builder) populateDiffStats(data *template.StatusData, stats git.DiffStats) {
	// Raw values only (formatting is done in templates via fmtSigned)
	data.GitAdditions = stats.Additions
	data.GitDeletions = stats.Deletions
	data.GitNewFiles = stats.NewFiles
	data.GitModifiedFiles = stats.ModifiedFiles
	data.GitDeletedFiles = stats.DeletedFiles
	data.GitUnstagedFiles = stats.UnstagedFiles
}

func (b *Builder) fetchGitHubStatus(data *template.StatusData, branch string) {
	// Get remote URL
	remoteURL, err := b.git.RemoteURL()
	if err != nil {
		slog.Debug("failed to get remote URL", "err", err)
		return
	}

	// Parse owner/repo
	owner, repo, ok := git.ParseGitHubRepo(remoteURL)
	if !ok {
		slog.Debug("not a GitHub repository", "remoteURL", remoteURL)
		return
	}

	// Lazily initialize GitHub client if needed
	if b.gh == nil {
		ghClient, err := github.NewClient(b.config.GitHubWorkflow)
		if err != nil {
			slog.Debug("failed to create GitHub client", "err", err)
			return
		}
		b.gh = ghClient
	}

	// Get build status with caching
	ttl := time.Duration(b.config.GitHubTTL) * time.Second
	refPath := b.git.RefPath(branch)

	buildStatus, err := b.cache.GetGitHubBuild(refPath, branch, ttl, func() (github.BuildStatus, error) {
		return b.gh.GetBuildStatus(owner, repo, branch)
	})
	if err != nil {
		slog.Debug("failed to get GitHub build status", "owner", owner, "repo", repo, "branch", branch, "err", err)
		return
	}

	data.GitHubStatus = github.StatusToEmoji(buildStatus)
}

// SetGitHubClient sets the GitHub client (for lazy initialization or testing).
func (b *Builder) SetGitHubClient(gh GitHubProvider) {
	b.gh = gh
}

// SetPrefix sets a prefix to be displayed at the start of the status line.
func (b *Builder) SetPrefix(prefix string) {
	b.prefix = prefix
}

// SetPrefixColor sets the ANSI color code for the prefix.
func (b *Builder) SetPrefixColor(color string) {
	b.prefixColor = color
}

// fetchTaskStats fetches task stats and populates the data.
func (b *Builder) fetchTaskStats(data *template.StatusData) {
	if b.taskProvider == nil {
		return
	}

	ttl := time.Duration(b.config.TasksTTL) * time.Second
	stats, err := b.cache.GetTaskStats(b.workDir, ttl, b.taskProvider.GetStats)
	if err != nil {
		slog.Debug("failed to get task stats", "err", err)
		return
	}

	b.populateTaskStats(data, stats)

	// Get next task (cached with same TTL as stats)
	nextTask, err := b.cache.GetNextTask(b.workDir, ttl, b.taskProvider.GetNextTask)
	if err != nil {
		slog.Debug("failed to get next task", "err", err)
		return
	}
	data.TasksNextTask = nextTask
}

// populateTaskStats populates task statistics into StatusData.
func (b *Builder) populateTaskStats(data *template.StatusData, stats tasks.Stats) {
	data.HasTasks = true
	data.TaskProvider = b.taskProvider.Name()

	// Set task list ID if using shared list (claude provider with env var)
	if b.taskProvider.Name() == "claude" {
		data.TaskListID = os.Getenv("CLAUDE_CODE_TASK_LIST_ID")
	}

	// Raw values only (formatting is done in templates)
	data.TasksTotal = stats.TotalIssues
	data.TasksOpen = stats.OpenIssues
	data.TasksReady = stats.ReadyIssues
	data.TasksInProgress = stats.InProgressIssues
	data.TasksBlocked = stats.BlockedIssues
}
