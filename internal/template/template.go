package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// ANSI color codes
const (
	colorCyan    = "\033[36m"
	colorBlue    = "\033[34m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorMagenta = "\033[35m"
	colorGray    = "\033[90m"
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
)

// ColorMap maps color names to ANSI codes for use with --prefix-color flag.
var ColorMap = map[string]string{
	"cyan":    colorCyan,
	"blue":    colorBlue,
	"green":   colorGreen,
	"yellow":  colorYellow,
	"red":     colorRed,
	"magenta": colorMagenta,
	"gray":    colorGray,
}

// StatusData holds all the data available for template rendering.
// All values are raw; use template functions (fmtTokens, fmtPct, fmtSigned) for formatting.
type StatusData struct {
	Prefix       string // User-provided prefix text
	PrefixColor  string // ANSI color code for prefix (from --prefix-color flag)
	Model        string // Model display name (e.g., "Claude")
	Dir          string // Current directory basename
	GitBranch    string // Current git branch (empty if not in git repo)
	GitStatus    string // Git status like "±3" (empty if clean)
	GitHubStatus string // GitHub build status emoji (empty if unavailable)
	Version      string // Claude Code version

	// Git diff stats (raw values - use fmtSigned for display)
	GitAdditions     int // Line additions count
	GitDeletions     int // Line deletions count
	GitNewFiles      int // New files count
	GitModifiedFiles int // Modified files count
	GitDeletedFiles  int // Deleted files count
	GitUnstagedFiles int // Unstaged files count

	// Token metrics (raw values - use fmtTokens for display)
	// TokensInput, TokensOutput, TokensCached are per-call (last API call only).
	// TokensTotal is session-cumulative (sum across all API calls).
	TokensInput   int64   // Input tokens (per-call)
	TokensOutput  int64   // Output tokens (per-call)
	TokensCached  int64   // Cached tokens (per-call)
	TokensTotal   int64   // Total tokens (session-cumulative)
	ContextLength int64   // Current context length
	ContextPct    float64 // Context percentage (0-100)
	ContextPctUse float64 // Usable context percentage (0-100)

	// Context window metadata
	ContextWindowSize int64 // Max context window size in tokens (200k or 1M)

	// Cost metrics (from Claude Code stdin - use fmtCost/fmtDuration/fmtSigned for display)
	CostUSD          float64 // Session cost in USD
	DurationMS       int64   // Total wall-clock duration in milliseconds
	APIDurationMS    int64   // Total API response time in milliseconds
	CostLinesAdded   int     // Total lines added in session
	CostLinesRemoved int     // Total lines removed in session

	// Task stats (raw values) - populated by claude, kt, or tk
	TaskProvider    string // Provider name: "claude", "kt", or "tk"
	TaskListID      string // Task list ID (from CLAUDE_CODE_TASK_LIST_ID env var, empty if using session)
	TasksTotal      int    // Total issues
	TasksOpen       int    // Open issues
	TasksReady      int    // Ready to work issues
	TasksInProgress int    // In progress issues
	TasksBlocked    int    // Blocked issues
	TasksNextTask   string // Title of next ready task, or empty if none
	HasTasks        bool   // Whether task system is available

	// Workspace extras (from stdin)
	ProjectDir  string   // Directory where Claude Code was launched
	AddedDirs   []string // Extra dirs from /add-dir
	GitWorktree string   // Git worktree name when inside a linked worktree
	RepoHost    string   // Origin remote host (e.g., "github.com")
	RepoOwner   string   // Origin remote owner
	RepoName    string   // Origin remote repo name

	// Session metadata (from stdin)
	SessionName       string // Custom session name from --name or /rename
	OutputStyle       string // Active output style name
	Exceeds200kTokens bool   // Whether last response exceeds 200k tokens
	EffortLevel       string // low|medium|high|xhigh|max
	ThinkingEnabled   bool   // Extended thinking on/off
	VimMode           string // NORMAL|INSERT|VISUAL|VISUAL LINE
	AgentName         string // --agent name when running an agent

	// Rate limits (Pro/Max — fields may be 0 when absent)
	RateLimitFiveHourPct      float64 // 5h window used %
	RateLimitFiveHourResetsAt int64   // 5h window unix epoch reset
	RateLimitSevenDayPct      float64 // 7d window used %
	RateLimitSevenDayResetsAt int64   // 7d window unix epoch reset

	// Pull request (from stdin — present only when an open PR is found)
	PRNumber      int    // Open PR number (0 when absent)
	PRURL         string // PR URL
	PRReviewState string // approved|pending|changes_requested|draft

	// Worktree session (--worktree)
	WorktreeName           string
	WorktreePath           string
	WorktreeBranch         string
	WorktreeOriginalCwd    string
	WorktreeOriginalBranch string
}

// FormatTokens formats a token count in a human-readable way.
// e.g., 1234 -> "1.2k", 1234567 -> "1.2M"
func FormatTokens(count int64) string {
	if count >= 1_000_000 {
		return formatWithSuffix(float64(count)/1_000_000, "M")
	}
	if count >= 1_000 {
		return formatWithSuffix(float64(count)/1_000, "k")
	}
	return fmt.Sprintf("%d", count)
}

func formatWithSuffix(f float64, suffix string) string {
	s := fmt.Sprintf("%.1f", f)
	s = strings.TrimSuffix(s, ".0")
	return s + suffix
}

// funcs is the template function map with color helpers and formatters.
var funcs = template.FuncMap{
	"cyan":    func() string { return colorCyan },
	"blue":    func() string { return colorBlue },
	"green":   func() string { return colorGreen },
	"yellow":  func() string { return colorYellow },
	"red":     func() string { return colorRed },
	"magenta": func() string { return colorMagenta },
	"gray":    func() string { return colorGray },
	"reset":   func() string { return colorReset },
	"bold":    func() string { return colorBold },

	// Context percentage color: green < 50%, yellow 50-80%, red > 80%
	"ctxColor": func(pct float64) string {
		if pct >= 80 {
			return colorRed
		}
		if pct >= 50 {
			return colorYellow
		}
		return colorGreen
	},

	// fmtTokens formats token counts: 10500 -> "10.5k", 1234567 -> "1.2M"
	"fmtTokens": FormatTokens,

	// fmtPct formats a percentage: 45.2 -> "45.2%"
	"fmtPct": func(pct float64) string {
		return fmt.Sprintf("%.1f%%", pct)
	},

	// fmtSigned formats an integer with + prefix for positive: 42 -> "+42", -5 -> "-5"
	"fmtSigned": func(n int) string {
		if n > 0 {
			return fmt.Sprintf("+%d", n)
		}
		return fmt.Sprintf("%d", n)
	},

	// fmtCost formats a USD amount: 0.01234 -> "$0.01", 1.50 -> "$1.50"
	"fmtCost": func(usd float64) string {
		return fmt.Sprintf("$%.2f", usd)
	},

	// link wraps text in an OSC 8 hyperlink so terminals that support it render
	// the text as clickable. Terminals that don't support OSC 8 ignore the
	// sequence and show the plain text.
	"link": func(url, text string) string {
		if url == "" {
			return text
		}
		return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
	},

	// prEmoji maps a PR review state to an emoji indicator.
	"prEmoji": func(state string) string {
		switch state {
		case "approved":
			return "✅"
		case "changes_requested":
			return "❌"
		case "draft":
			return "📝"
		case "pending":
			return "⏳"
		}
		return "🔀"
	},

	// fmtDuration formats milliseconds to human-readable: 125000 -> "2m 5s", 7500000 -> "2h 5m"
	"fmtDuration": func(ms int64) string {
		totalSec := ms / 1000
		hours := totalSec / 3600
		mins := (totalSec % 3600) / 60
		secs := totalSec % 60
		if hours > 0 {
			return fmt.Sprintf("%dh %dm", hours, mins)
		}
		if mins > 0 {
			return fmt.Sprintf("%dm %ds", mins, secs)
		}
		return fmt.Sprintf("%ds", secs)
	},
}

// Engine renders status lines using Go templates.
type Engine struct {
	tmpl *template.Template
}

// NewEngine creates a new template engine with the given template string.
func NewEngine(templateStr string) (*Engine, error) {
	tmpl, err := template.New("status").Funcs(funcs).Parse(templateStr)
	if err != nil {
		return nil, err
	}
	return &Engine{tmpl: tmpl}, nil
}

// Render executes the template with the given data and returns the result.
func (e *Engine) Render(data StatusData) (string, error) {
	var buf bytes.Buffer
	if err := e.tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
