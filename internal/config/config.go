package config

import (
	"encoding/json"
	"log/slog"
	"os"
)

// DefaultTemplate is the default Go template for the status line.
// All values are raw numbers; use fmtTokens, fmtPct, fmtSigned for formatting.
// Prefix color is set via --prefix-color flag (defaults to cyan if prefix is set).
const DefaultTemplate = `{{if .Prefix}}{{.PrefixColor}}{{.Prefix}}{{reset}} | {{end}}{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{if or .GitAdditions .GitDeletions}} {{green}}{{fmtSigned .GitAdditions}}{{reset}},{{red}}-{{.GitDeletions}}{{reset}}{{end}}{{if or .GitNewFiles .GitModifiedFiles .GitDeletedFiles .GitUnstagedFiles}}{{if .GitNewFiles}} ✨{{.GitNewFiles}}{{end}}{{if .GitModifiedFiles}} 📝{{.GitModifiedFiles}}{{end}}{{if .GitDeletedFiles}} 🗑{{.GitDeletedFiles}}{{end}}{{if .GitUnstagedFiles}} ⚡{{.GitUnstagedFiles}}{{end}}{{end}}{{end}}{{if .GitHubStatus}} | {{.GitHubStatus}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUse}}📊 {{fmtPct .ContextPctUse}}{{reset}}{{end}}{{if .Version}} | {{gray}}v{{.Version}}{{reset}}{{end}}{{if .TasksReady}}
{{yellow}}📋 {{.TaskProvider}}: {{.TasksReady}} ready{{reset}}{{if .TasksBlocked}}, {{red}}{{.TasksBlocked}} blocked{{reset}}{{end}}{{if .TasksNextTask}}. Next Up: {{.TasksNextTask}}{{end}}{{end}}`

// TemplateWithTokens is an example template that shows all token metrics.
// Usage: set "template" in config.json to this value.
const TemplateWithTokens = `{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{end}}{{if .TokensTotal}} | {{gray}}📈 In:{{fmtTokens .TokensInput}} Out:{{fmtTokens .TokensOutput}} Cache:{{fmtTokens .TokensCached}}{{reset}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUse}}📊 {{fmtPct .ContextPctUse}}{{reset}}{{end}}`

// TemplateWithTasks is an example template that shows task stats (beads/tk/kt).
// Usage: set "template" in config.json to this value.
const TemplateWithTasks = `{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUse}}📊 {{fmtPct .ContextPctUse}}{{reset}}{{end}}{{if .TasksReady}} | {{yellow}}📋 {{.TaskProvider}}: {{.TasksReady}} ready{{reset}}{{if .TasksBlocked}}, {{red}}{{.TasksBlocked}} blocked{{reset}}{{end}}{{if .TasksNextTask}}. Next Up: {{.TasksNextTask}}{{end}}{{end}}`

// Config holds the configuration for claude-status.
type Config struct {
	// Template is the Go template string for rendering the status line.
	Template string `json:"template"`

	// GitHubWorkflow is the name of the GitHub workflow to check.
	GitHubWorkflow string `json:"github_workflow"`

	// GitHubTTL is the time-to-live in seconds for cached GitHub build status.
	GitHubTTL int `json:"github_ttl"`

	// TasksTTL is the time-to-live in seconds for cached task stats.
	TasksTTL int `json:"tasks_ttl"`

	// LoggingEnabled enables logging of status line events.
	LoggingEnabled bool `json:"logging_enabled"`

	// LogPath is an optional override for the log file path.
	LogPath string `json:"log_path"`
}

// Default returns a Config with sensible default values.
func Default() Config {
	return Config{
		Template:       DefaultTemplate,
		GitHubWorkflow: "build_and_test",
		GitHubTTL:      60,
		TasksTTL:       5,
		LoggingEnabled: false,
		LogPath:        "",
	}
}

// Load reads the config file and returns a merged Config.
// Missing fields use default values. If the file doesn't exist or
// is invalid, default values are returned.
func Load() Config {
	return LoadFrom(ConfigPath())
}

// LoadFrom reads config from a specific path.
func LoadFrom(path string) Config {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist is normal, don't log
		if !os.IsNotExist(err) {
			slog.Error("failed to read config", "err", err)
		}
		return cfg
	}

	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		slog.Error("invalid config file", "path", path, "err", err)
		return cfg
	}

	// Merge: only override defaults for non-zero values
	if fileCfg.Template != "" {
		cfg.Template = fileCfg.Template
	}
	if fileCfg.GitHubWorkflow != "" {
		cfg.GitHubWorkflow = fileCfg.GitHubWorkflow
	}
	if fileCfg.GitHubTTL > 0 {
		cfg.GitHubTTL = fileCfg.GitHubTTL
	}
	if fileCfg.TasksTTL > 0 {
		cfg.TasksTTL = fileCfg.TasksTTL
	}
	// LoggingEnabled is a bool, so we check if it was explicitly set
	// by seeing if the JSON had the field (we need to re-parse for this)
	var rawCfg map[string]json.RawMessage
	if json.Unmarshal(data, &rawCfg) == nil {
		if _, ok := rawCfg["logging_enabled"]; ok {
			cfg.LoggingEnabled = fileCfg.LoggingEnabled
		}
	}
	if fileCfg.LogPath != "" {
		cfg.LogPath = fileCfg.LogPath
	}

	return cfg
}
