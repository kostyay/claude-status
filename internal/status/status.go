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
	"github.com/kostyay/claude-status/internal/pricing"
	"github.com/kostyay/claude-status/internal/tasks"
	"github.com/kostyay/claude-status/internal/template"
	"github.com/kostyay/claude-status/internal/tokens"

	// Task providers (priority controlled by RegisterWithPriority, not import order)
	_ "github.com/kostyay/claude-status/internal/claudetasks"
	_ "github.com/kostyay/claude-status/internal/kt"
	_ "github.com/kostyay/claude-status/internal/tk"
)

// Input represents the JSON input from stdin.
// See https://code.claude.com/docs/en/statusline for the full schema.
type Input struct {
	Model             ModelInfo          `json:"model"`
	Workspace         WorkspaceInfo      `json:"workspace"`
	Version           string             `json:"version"`
	SessionID         string             `json:"session_id"`
	SessionName       string             `json:"session_name"`
	TranscriptPath    string             `json:"transcript_path"`
	ContextWindow     *ContextWindowInfo `json:"context_window"`
	Cost              *CostInfo          `json:"cost"`
	Exceeds200kTokens bool               `json:"exceeds_200k_tokens"`
	OutputStyle       *OutputStyleInfo   `json:"output_style"`
	Effort            *EffortInfo        `json:"effort"`
	Thinking          *ThinkingInfo      `json:"thinking"`
	RateLimits        *RateLimitsInfo    `json:"rate_limits"`
	Vim               *VimInfo           `json:"vim"`
	Agent             *AgentInfo         `json:"agent"`
	PR                *PRInfo            `json:"pr"`
	Worktree          *WorktreeInfo      `json:"worktree"`
}

// ModelInfo contains information about the model.
type ModelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// WorkspaceInfo contains workspace information.
type WorkspaceInfo struct {
	CurrentDir  string    `json:"current_dir"`
	ProjectDir  string    `json:"project_dir"`
	AddedDirs   []string  `json:"added_dirs"`
	GitWorktree string    `json:"git_worktree"`
	Repo        *RepoInfo `json:"repo"`
}

// RepoInfo contains repository identity parsed from the origin remote.
type RepoInfo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// OutputStyleInfo contains the active output style.
type OutputStyleInfo struct {
	Name string `json:"name"`
}

// EffortInfo contains the reasoning effort level.
type EffortInfo struct {
	Level string `json:"level"`
}

// ThinkingInfo contains extended thinking state.
type ThinkingInfo struct {
	Enabled bool `json:"enabled"`
}

// RateLimitsInfo contains Pro/Max rate-limit usage windows.
type RateLimitsInfo struct {
	FiveHour *RateLimitWindow `json:"five_hour"`
	SevenDay *RateLimitWindow `json:"seven_day"`
}

// RateLimitWindow describes a single rate-limit window.
type RateLimitWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

// VimInfo contains vim mode state.
type VimInfo struct {
	Mode string `json:"mode"`
}

// AgentInfo contains the active agent name.
type AgentInfo struct {
	Name string `json:"name"`
}

// PRInfo contains the open PR for the current branch.
type PRInfo struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	ReviewState string `json:"review_state"`
}

// WorktreeInfo contains --worktree session metadata.
type WorktreeInfo struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	OriginalCwd    string `json:"original_cwd"`
	OriginalBranch string `json:"original_branch"`
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
	pricing      pricing.Provider
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
		pricing: pricing.NewLazy(config.CacheDir()),
	}

	// Try to initialize git client (may fail if not in git repo)
	if gitClient, err := git.NewClient(workDir); err == nil {
		b.git = gitClient
	} else {
		slog.Debug("git client initialization skipped", "workDir", workDir, "err", err)
	}

	// Initialize task tracker via registry (priority: claude > kt > tk)
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

	populateFromInput(&data, input)

	// Parse token metrics from transcript
	metrics := b.populateTokenMetrics(&data, input)

	// Populate cost metrics from Claude Code stdin, or fall back to models.dev pricing.
	if input.Cost != nil {
		data.CostUSD = input.Cost.TotalCostUSD
		data.DurationMS = input.Cost.TotalDurationMS
		data.APIDurationMS = input.Cost.TotalAPIDurationMS
		data.CostLinesAdded = input.Cost.TotalLinesAdded
		data.CostLinesRemoved = input.Cost.TotalLinesRemoved
	} else if b.pricing != nil && metrics.TotalTokens > 0 {
		if price, ok := b.pricing.Lookup(input.Model.ID); ok {
			data.CostUSD = pricing.Cost(metrics, price)
		}
	}

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

// populateFromInput copies optional stdin fields into the status template data.
func populateFromInput(data *template.StatusData, input Input) {
	data.ProjectDir = input.Workspace.ProjectDir
	data.AddedDirs = input.Workspace.AddedDirs
	data.GitWorktree = input.Workspace.GitWorktree
	if r := input.Workspace.Repo; r != nil {
		data.RepoHost = r.Host
		data.RepoOwner = r.Owner
		data.RepoName = r.Name
	}
	data.SessionName = input.SessionName
	data.Exceeds200kTokens = input.Exceeds200kTokens
	if input.OutputStyle != nil {
		data.OutputStyle = input.OutputStyle.Name
	}
	if input.Effort != nil {
		data.EffortLevel = input.Effort.Level
	}
	if input.Thinking != nil {
		data.ThinkingEnabled = input.Thinking.Enabled
	}
	if input.Vim != nil {
		data.VimMode = input.Vim.Mode
	}
	if input.Agent != nil {
		data.AgentName = input.Agent.Name
	}
	if rl := input.RateLimits; rl != nil {
		if w := rl.FiveHour; w != nil {
			data.RateLimitFiveHourPct = w.UsedPercentage
			data.RateLimitFiveHourResetsAt = w.ResetsAt
		}
		if w := rl.SevenDay; w != nil {
			data.RateLimitSevenDayPct = w.UsedPercentage
			data.RateLimitSevenDayResetsAt = w.ResetsAt
		}
	}
	if pr := input.PR; pr != nil {
		data.PRNumber = pr.Number
		data.PRURL = pr.URL
		data.PRReviewState = pr.ReviewState
	}
	if wt := input.Worktree; wt != nil {
		data.WorktreeName = wt.Name
		data.WorktreePath = wt.Path
		data.WorktreeBranch = wt.Branch
		data.WorktreeOriginalCwd = wt.OriginalCwd
		data.WorktreeOriginalBranch = wt.OriginalBranch
	}
}

// populateTokenMetrics populates token metrics from context_window (preferred)
// or transcript (fallback) and returns the metrics used for cost calculations.
func (b *Builder) populateTokenMetrics(data *template.StatusData, input Input) tokens.Metrics {
	// Prefer context_window data from Claude Code stdin, else parse the transcript.
	if input.ContextWindow != nil && input.ContextWindow.UsedPercentage != nil {
		m := b.populateFromContextWindow(data, input)
		data.ContextWindowSize = b.contextConfig(input, m.ContextLength).MaxTokens
		return m
	}
	return b.populateFromTranscript(data, input)
}

// populateFromContextWindow uses pre-calculated context data from Claude Code.
func (b *Builder) populateFromContextWindow(data *template.StatusData, input Input) tokens.Metrics {
	cw := input.ContextWindow

	data.ContextPct = *cw.UsedPercentage

	// Scale to usable context (auto-compact threshold)
	data.ContextPctUse = min(*cw.UsedPercentage/tokens.AutoCompactThreshold, 100)

	var m tokens.Metrics
	// Populate token metrics from current_usage if available
	if cw.CurrentUsage != nil {
		u := cw.CurrentUsage
		data.TokensInput = u.InputTokens
		data.TokensOutput = u.OutputTokens
		data.TokensCached = u.CacheReadInputTokens + u.CacheCreationInputTokens
		data.ContextLength = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens

		m.InputTokens = u.InputTokens
		m.OutputTokens = u.OutputTokens
		m.CacheReadTokens = u.CacheReadInputTokens
		m.CacheCreationTokens = u.CacheCreationInputTokens
		m.CachedTokens = u.CacheReadInputTokens + u.CacheCreationInputTokens
		m.ContextLength = data.ContextLength
	}

	// Use cumulative totals for total tokens (session-cumulative, not per-call)
	data.TokensTotal = cw.TotalInputTokens + cw.TotalOutputTokens
	m.TotalTokens = m.InputTokens + m.OutputTokens + m.CachedTokens
	return m
}

// contextConfig resolves the session's context window, most authoritative first:
// the size Claude Code reports, then the "[1m]" model-ID suffix. Older Claude Code
// supplies neither, so as a last resort trust models.dev's limit — but only once
// observed usage proves the assumed window too small, since models.dev reports
// what a model supports, not how this session was configured.
func (b *Builder) contextConfig(input Input, contextLength int64) tokens.ContextConfig {
	if cw := input.ContextWindow; cw != nil && cw.ContextWindowSize > 0 {
		return tokens.NewContextConfig(cw.ContextWindowSize)
	}
	cfg := tokens.GetContextConfig(input.Model.ID)
	if contextLength <= cfg.MaxTokens || b.pricing == nil {
		return cfg
	}
	if p, ok := b.pricing.Lookup(input.Model.ID); ok && p.ContextLimit > cfg.MaxTokens {
		return tokens.NewContextConfig(p.ContextLimit)
	}
	return cfg
}

// populateFromTranscript parses the transcript JSONL file for token metrics.
func (b *Builder) populateFromTranscript(data *template.StatusData, input Input) tokens.Metrics {
	var metrics tokens.Metrics
	if input.TranscriptPath != "" {
		var err error
		metrics, err = tokens.ParseTranscript(input.TranscriptPath)
		if err != nil {
			slog.Debug("failed to parse transcript", "path", input.TranscriptPath, "err", err)
			metrics = tokens.Metrics{}
		}
	}

	ctxCfg := b.contextConfig(input, metrics.ContextLength)
	data.ContextWindowSize = ctxCfg.MaxTokens

	// Populate raw values (formatting is done in templates via fmtTokens/fmtPct)
	data.TokensInput = metrics.InputTokens
	data.TokensOutput = metrics.OutputTokens
	data.TokensCached = metrics.CachedTokens
	data.TokensTotal = metrics.TotalTokens
	data.ContextLength = metrics.ContextLength
	data.ContextPct = metrics.ContextPercentage(ctxCfg)
	data.ContextPctUse = metrics.ContextPercentageUsable(ctxCfg)
	return metrics
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

// SetPricingProvider overrides the pricing provider used for cost fallback.
// Primarily intended for tests; production code uses the lazy provider set in
// NewBuilder.
func (b *Builder) SetPricingProvider(p pricing.Provider) {
	b.pricing = p
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
