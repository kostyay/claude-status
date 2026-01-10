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
	TokensInput   int64   // Input tokens
	TokensOutput  int64   // Output tokens
	TokensCached  int64   // Cached tokens
	TokensTotal   int64   // Total tokens
	ContextLength int64   // Current context length
	ContextPct    float64 // Context percentage (0-100)
	ContextPctUse float64 // Usable context percentage (0-100)

	// Task stats (raw values) - populated by kt, tk, or beads
	TaskProvider    string // Provider name: "kt", "tk", or "beads"
	TasksTotal      int    // Total issues
	TasksOpen       int    // Open issues
	TasksReady      int    // Ready to work issues
	TasksInProgress int    // In progress issues
	TasksBlocked    int    // Blocked issues
	TasksNextTask   string // Title of next ready task, or empty if none
	HasTasks        bool   // Whether task system is available
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
