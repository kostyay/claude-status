package config

import (
	"encoding/json"
	"log/slog"
	"os"
)

// DefaultTemplate is the default Go template for the status line.
// It replicates the Python version's output with emojis and adds git changes.
// Uses ContextPctUse (usable context before auto-compact) to match Claude's display.
const DefaultTemplate = `{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{if or .GitAdditionsRaw .GitDeletionsRaw}} {{green}}{{.GitAdditions}}{{reset}},{{red}}{{.GitDeletions}}{{reset}}{{end}}{{if or .GitNewFilesRaw .GitModifiedFilesRaw .GitDeletedFilesRaw}} {{.GitNewFiles}}{{.GitModifiedFiles}}{{.GitDeletedFiles}}{{end}}{{end}}{{if .GitHubStatus}} | {{.GitHubStatus}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUseRaw}}📊 {{.ContextPctUse}}{{reset}}{{end}}{{if .HasBeads}} | {{yellow}}📋 {{.BeadsReady}}{{reset}}{{if .BeadsBlocked}} {{red}}{{.BeadsBlocked}}{{reset}}{{end}}{{end}}{{if .Version}} | {{gray}}v{{.Version}}{{reset}}{{end}}`

// TemplateWithTokens is an example template that shows all token metrics.
// Usage: set "template" in config.json to this value.
const TemplateWithTokens = `{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{end}}{{if .TokensTotal}} | {{gray}}📈 In:{{.TokensInput}} Out:{{.TokensOutput}} Cache:{{.TokensCached}}{{reset}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUseRaw}}📊 {{.ContextPctUse}}{{reset}}{{end}}`

// TemplateWithBeads is an example template that shows beads task stats.
// Usage: set "template" in config.json to this value.
const TemplateWithBeads = `{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUseRaw}}📊 {{.ContextPctUse}}{{reset}}{{end}}{{if .HasBeads}} | {{yellow}}📋 {{.BeadsReady}}{{reset}}{{if .BeadsBlocked}} {{red}}{{.BeadsBlocked}}{{reset}}{{end}}{{end}}`

// Config holds the configuration for claude-status.
type Config struct {
	// Template is the Go template string for rendering the status line.
	Template string `json:"template"`

	// GitHubWorkflow is the name of the GitHub workflow to check.
	GitHubWorkflow string `json:"github_workflow"`

	// GitHubTTL is the time-to-live in seconds for cached GitHub build status.
	GitHubTTL int `json:"github_ttl"`

	// BeadsTTL is the time-to-live in seconds for cached beads stats.
	BeadsTTL int `json:"beads_ttl"`

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
		BeadsTTL:       60,
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
	if fileCfg.BeadsTTL > 0 {
		cfg.BeadsTTL = fileCfg.BeadsTTL
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
