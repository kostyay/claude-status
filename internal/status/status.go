package status

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/kostyay/claude-status/internal/beads"
	"github.com/kostyay/claude-status/internal/cache"
	"github.com/kostyay/claude-status/internal/config"
	"github.com/kostyay/claude-status/internal/git"
	"github.com/kostyay/claude-status/internal/github"
	"github.com/kostyay/claude-status/internal/template"
	"github.com/kostyay/claude-status/internal/tokens"
)

// Input represents the JSON input from stdin.
type Input struct {
	Model          ModelInfo     `json:"model"`
	Workspace      WorkspaceInfo `json:"workspace"`
	Version        string        `json:"version"`
	SessionID      string        `json:"session_id"`
	TranscriptPath string        `json:"transcript_path"`
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
	GetBeadsStats(workDir string, ttl time.Duration, fetchFn func() (beads.Stats, error)) (beads.Stats, error)
	EnsureDir() error
}

// BeadsProvider is an interface for beads operations.
type BeadsProvider interface {
	GetStats() (beads.Stats, error)
	GetNextTask() (string, error)
	HasBeads() bool
}

// Builder constructs StatusData from various sources.
type Builder struct {
	config  *config.Config
	cache   CacheProvider
	git     GitProvider
	gh      GitHubProvider
	beads   BeadsProvider
	workDir string
}

// ErrNilConfig is returned when a nil config is provided to NewBuilder.
var ErrNilConfig = fmt.Errorf("config cannot be nil")

// NewBuilder creates a new status builder.
func NewBuilder(cfg *config.Config, workDir string) (*Builder, error) {
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

	// Initialize beads client
	beadsClient := beads.NewClient(workDir)
	if beadsClient.HasBeads() {
		b.beads = beadsClient
	}

	return b, nil
}

// NewBuilderWithDeps creates a new status builder with injected dependencies.
func NewBuilderWithDeps(cfg *config.Config, cache CacheProvider, git GitProvider, gh GitHubProvider, beads BeadsProvider) *Builder {
	return &Builder{
		config: cfg,
		cache:  cache,
		git:    git,
		gh:     gh,
		beads:  beads,
	}
}

// Build constructs StatusData from the input.
func (b *Builder) Build(input Input) template.StatusData {
	data := template.StatusData{
		Model:   input.Model.DisplayName,
		Dir:     filepath.Base(input.Workspace.CurrentDir),
		Version: input.Version,
	}

	if data.Model == "" {
		data.Model = "Claude"
	}

	// Parse token metrics from transcript
	b.populateTokenMetrics(&data, input)

	// Get beads stats (cached with TTL) - independent of git
	b.fetchBeadsStats(&data)

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

// populateTokenMetrics parses the transcript and populates token metrics.
func (b *Builder) populateTokenMetrics(data *template.StatusData, input Input) {
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

	// Calculate percentages
	ctxPct := metrics.ContextPercentage(ctxCfg)
	ctxPctUsable := metrics.ContextPercentageUsable(ctxCfg)

	// Populate formatted values
	data.TokensInput = tokens.FormatTokens(metrics.InputTokens)
	data.TokensOutput = tokens.FormatTokens(metrics.OutputTokens)
	data.TokensCached = tokens.FormatTokens(metrics.CachedTokens)
	data.TokensTotal = tokens.FormatTokens(metrics.TotalTokens)
	data.ContextLength = tokens.FormatTokens(metrics.ContextLength)
	data.ContextPct = fmt.Sprintf("%.1f%%", ctxPct)
	data.ContextPctUse = fmt.Sprintf("%.1f%%", ctxPctUsable)

	// Populate raw values
	data.TokensInputRaw = metrics.InputTokens
	data.TokensOutputRaw = metrics.OutputTokens
	data.TokensCachedRaw = metrics.CachedTokens
	data.TokensTotalRaw = metrics.TotalTokens
	data.ContextLengthRaw = metrics.ContextLength
	data.ContextPctRaw = ctxPct
	data.ContextPctUseRaw = ctxPctUsable
}

// populateDiffStats populates git diff statistics into StatusData.
func (b *Builder) populateDiffStats(data *template.StatusData, stats git.DiffStats) {
	// Raw values
	data.GitAdditionsRaw = stats.Additions
	data.GitDeletionsRaw = stats.Deletions
	data.GitNewFilesRaw = stats.NewFiles
	data.GitModifiedFilesRaw = stats.ModifiedFiles
	data.GitDeletedFilesRaw = stats.DeletedFiles

	// Formatted values (only if non-zero)
	if stats.Additions > 0 {
		data.GitAdditions = fmt.Sprintf("+%d", stats.Additions)
	}
	if stats.Deletions > 0 {
		data.GitDeletions = fmt.Sprintf("-%d", stats.Deletions)
	}
	if stats.NewFiles > 0 {
		data.GitNewFiles = fmt.Sprintf("✨%d", stats.NewFiles)
	}
	if stats.ModifiedFiles > 0 {
		data.GitModifiedFiles = fmt.Sprintf("📝%d", stats.ModifiedFiles)
	}
	if stats.DeletedFiles > 0 {
		data.GitDeletedFiles = fmt.Sprintf("🗑%d", stats.DeletedFiles)
	}
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

// fetchBeadsStats fetches beads stats and populates the data.
func (b *Builder) fetchBeadsStats(data *template.StatusData) {
	if b.beads == nil {
		return
	}

	ttl := time.Duration(b.config.BeadsTTL) * time.Second
	stats, err := b.cache.GetBeadsStats(b.workDir, ttl, b.beads.GetStats)
	if err != nil {
		slog.Debug("failed to get beads stats", "err", err)
		return
	}

	b.populateBeadsStats(data, stats)

	// Get next task (not cached, fast enough)
	nextTask, err := b.beads.GetNextTask()
	if err != nil {
		slog.Debug("failed to get next task", "err", err)
		return
	}
	data.BeadsNextTask = nextTask
}

// populateBeadsStats populates beads statistics into StatusData.
func (b *Builder) populateBeadsStats(data *template.StatusData, stats beads.Stats) {
	data.HasBeads = true

	// Raw values
	data.BeadsTotalRaw = stats.TotalIssues
	data.BeadsOpenRaw = stats.OpenIssues
	data.BeadsReadyRaw = stats.ReadyIssues
	data.BeadsInProgressRaw = stats.InProgressIssues
	data.BeadsBlockedRaw = stats.BlockedIssues

	// Formatted values (only if non-zero)
	if stats.OpenIssues > 0 {
		data.BeadsOpen = fmt.Sprintf("%d open", stats.OpenIssues)
	}
	if stats.ReadyIssues > 0 {
		data.BeadsReady = fmt.Sprintf("%d ready", stats.ReadyIssues)
	}
	if stats.InProgressIssues > 0 {
		data.BeadsInProgress = fmt.Sprintf("%d wip", stats.InProgressIssues)
	}
	if stats.BlockedIssues > 0 {
		data.BeadsBlocked = fmt.Sprintf("%d blocked", stats.BlockedIssues)
	}
}
