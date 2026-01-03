package template

import (
	"bytes"
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
type StatusData struct {
	Prefix      string // User-provided prefix text
	PrefixColor string // ANSI color code for prefix (from --prefix-color flag)
	Model        string // Model display name (e.g., "Claude")
	Dir          string // Current directory basename
	GitBranch    string // Current git branch (empty if not in git repo)
	GitStatus    string // Git status like "±3" (empty if clean)
	GitHubStatus string // GitHub build status emoji (empty if unavailable)
	Version      string // Claude Code version

	// Git diff stats (formatted)
	GitAdditions     string // "+42" or empty if 0
	GitDeletions     string // "-10" or empty if 0
	GitNewFiles      string // "✨2" or empty if 0
	GitModifiedFiles string // "📝1" or empty if 0
	GitDeletedFiles  string // "🗑1" or empty if 0

	// Git diff stats (raw values for conditionals)
	GitAdditionsRaw     int // Raw additions count
	GitDeletionsRaw     int // Raw deletions count
	GitNewFilesRaw      int // Raw new files count
	GitModifiedFilesRaw int // Raw modified files count
	GitDeletedFilesRaw  int // Raw deleted files count

	// Token metrics
	TokensInput   string // Input tokens (formatted, e.g., "10.5k")
	TokensOutput  string // Output tokens (formatted)
	TokensCached  string // Cached tokens (formatted)
	TokensTotal   string // Total tokens (formatted)
	ContextLength string // Current context length (formatted)
	ContextPct    string // Context percentage (e.g., "45.2%")
	ContextPctUse string // Usable context percentage (e.g., "56.5%")

	// Raw values for conditional logic
	TokensInputRaw   int64   // Raw input tokens
	TokensOutputRaw  int64   // Raw output tokens
	TokensCachedRaw  int64   // Raw cached tokens
	TokensTotalRaw   int64   // Raw total tokens
	ContextLengthRaw int64   // Raw context length
	ContextPctRaw    float64 // Raw context percentage
	ContextPctUseRaw float64 // Raw usable context percentage

	// Beads stats (formatted)
	BeadsOpen       string // "3 open" or empty if 0
	BeadsReady      string // "2 ready" or empty if 0
	BeadsInProgress string // "1 wip" or empty if 0
	BeadsBlocked    string // "1 blocked" or empty if 0
	BeadsNextTask   string // Title of next ready task, or empty if none

	// Beads stats (raw values for conditionals)
	BeadsTotalRaw      int // Total issues
	BeadsOpenRaw       int // Open issues
	BeadsReadyRaw      int // Ready to work issues
	BeadsInProgressRaw int // In progress issues
	BeadsBlockedRaw    int // Blocked issues
	HasBeads           bool // Whether beads system is available
}

// funcs is the template function map with color helpers.
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
