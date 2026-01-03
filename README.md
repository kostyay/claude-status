# claude-status

<p align="center">
  <img src="https://raw.githubusercontent.com/kostyay/claude-status/main/.github/logo.png" alt="claude-status logo" width="200">
</p>

A fast, lightweight status line for [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) written in Go.

![Example status line](https://img.shields.io/badge/Claude-Sonnet_4-cyan) ![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go) ![License](https://img.shields.io/badge/License-MIT-green)

```
work | [Sonnet 4] | 📁 my-project | 🌿 main ±3 +42,-10 ✨2📝1 | ✅ | 📊 56.5%
📋 Tasks: 2 ready, 1 blocked. Next Up: Implement feature X
```

## Features

- **Fast** - Single binary, ~10MB, sub-millisecond startup
- **Smart Caching** - Git info cached based on file modification times; no redundant git calls
- **GitHub CI Status** - Shows build status (✅ ❌ 🔄) for your current branch
- **Git Diff Stats** - Line additions/deletions and file change counts
- **Task Tracking** - Integrates with [beads](https://github.com/steveyegge/beads) for task visibility
- **Multi-Profile** - Use `--prefix` to identify different Claude sessions
- **Customizable** - Full Go template support with ANSI colors
- **Zero Config** - Works out of the box with sensible defaults
- **XDG Compliant** - Config, cache, and data stored in standard locations

## Installation

### From Source

```bash
go install github.com/kostyay/claude-status/cmd/claude-status@latest
```

### Manual Build

```bash
git clone https://github.com/kostyay/claude-status.git
cd claude-status
make build
# Binary is at ./claude-status
```

## Quick Start

The easiest way to configure Claude Code is with the built-in install command:

```bash
./claude-status -install
```

This will:
1. Show a diff of the changes to `~/.claude/settings.json`
2. Ask for confirmation before applying
3. Preserve any existing settings

Alternatively, manually add to your Claude Code settings (`~/.claude/settings.json`):

```json
{
  "statusLine": {
    "type": "command",
    "command": "/path/to/claude-status",
    "padding": 0
  }
}
```

That's it! The status line will appear in your Claude Code sessions.

### Multi-Profile Support

Use `--prefix` to identify different Claude Code sessions (e.g., work vs personal):

```json
{
  "statusLine": {
    "type": "command",
    "command": "/path/to/claude-status --prefix work --prefix-color blue",
    "padding": 0
  }
}
```

Available colors: `cyan` (default), `blue`, `green`, `yellow`, `red`, `magenta`, `gray`

### Custom Config Directory

If you use a custom Claude Code config directory, set `CLAUDE_CONFIG_DIR`:

```bash
CLAUDE_CONFIG_DIR=/custom/path ./claude-status -install
```

## What It Shows

| Component | Description | Example |
|-----------|-------------|---------|
| **Prefix** | Optional profile identifier | `work` |
| **Model** | Current Claude model | `[Sonnet 4]` |
| **Directory** | Current working directory | `📁 my-project` |
| **Git Branch** | Current branch name | `🌿 main` |
| **Git Status** | Uncommitted changes count | `±3` |
| **Git Diff** | Line additions/deletions | `+42,-10` |
| **Git Files** | New/modified/deleted files | `✨2📝1🗑1` |
| **GitHub CI** | Latest workflow run status | `✅` `❌` `🔄` |
| **Context %** | Usable context usage before auto-compact (color-coded) | `📊 56.5%` |
| **Version** | Claude Code version | `v1.0.0` |
| **Tasks** | Task tracking stats (if [beads](https://github.com/anthropics/beads) is configured) | `📋 Tasks: 2 ready, 1 blocked` |

### Token Metrics (Available in Templates)

| Metric | Description | Example |
|--------|-------------|---------|
| **Tokens Input** | Input tokens used | `10.5k` |
| **Tokens Output** | Output tokens generated | `5.2k` |
| **Tokens Cached** | Cached tokens (read + creation) | `35k` |
| **Tokens Total** | Sum of all tokens | `50.7k` |
| **Context Length** | Current context window size | `45.2k` |
| **Context % (Usable)** | Percentage of usable context (80% before auto-compact) - **default** | `56.5%` |
| **Context %** | Percentage of max context used (for custom templates) | `45.2%` |

### GitHub CI Status Icons

| Icon | Meaning |
|------|---------|
| ✅ | Build succeeded |
| ❌ | Build failed |
| 🔄 | Build in progress |
| ⚠️ | Status unknown |

### Task Tracking (beads)

If you use [beads](https://github.com/anthropics/beads) for task tracking, claude-status automatically detects the `.beads/` directory and shows task stats on a second line:

```
📋 Tasks: 2 ready, 1 blocked. Next Up: Implement feature X
```

- **Ready** - Tasks with no blockers that can be started
- **Blocked** - Tasks waiting on dependencies
- **Next Up** - Title of the first ready task

## Configuration

Create `~/.config/claude-status/config.json`:

```json
{
  "template": "{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{end}}{{if .GitHubStatus}} | {{.GitHubStatus}}{{end}}",
  "github_workflow": "ci",
  "github_ttl": 60,
  "logging_enabled": false
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `template` | string | (see below) | Go template for status line |
| `github_workflow` | string | `"build_and_test"` | GitHub Actions workflow name to monitor |
| `github_ttl` | int | `60` | Seconds to cache GitHub status |
| `beads_ttl` | int | `5` | Seconds to cache beads task stats |
| `logging_enabled` | bool | `false` | Enable status line logging |
| `log_path` | string | XDG data dir | Custom log file path |

### Default Template

The default template shows a complete status line with prefix support, git diff stats, and a second line for task tracking (if beads is configured):

```
{{if .Prefix}}{{.PrefixColor}}{{.Prefix}}{{reset}} | {{end}}{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{if or .GitAdditionsRaw .GitDeletionsRaw}} {{green}}{{.GitAdditions}}{{reset}},{{red}}{{.GitDeletions}}{{reset}}{{end}}{{if or .GitNewFilesRaw .GitModifiedFilesRaw .GitDeletedFilesRaw}} {{.GitNewFiles}}{{.GitModifiedFiles}}{{.GitDeletedFiles}}{{end}}{{end}}{{if .GitHubStatus}} | {{.GitHubStatus}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUseRaw}}📊 {{.ContextPctUse}}{{reset}}{{end}}{{if .Version}} | {{gray}}v{{.Version}}{{reset}}{{end}}{{if .BeadsReady}}
{{yellow}}📋 Tasks: {{.BeadsReady}}{{reset}}{{if .BeadsBlocked}}, {{red}}{{.BeadsBlocked}}{{reset}}{{end}}{{if .BeadsNextTask}}. Next Up: {{.BeadsNextTask}}{{end}}{{end}}
```

Features:
- **Prefix** - Optional profile identifier (via `--prefix` flag)
- **Context percentage** - Color-coded (green/yellow/red) based on usage
- **Git diff stats** - Shows additions, deletions, and file changes
- **Task tracking** - Second line with beads task stats (if available)

## Template Reference

### Available Fields

| Field | Type | Description |
|-------|------|-------------|
| `.Prefix` | string | Profile prefix (from `--prefix` flag) |
| `.PrefixColor` | string | ANSI color code for prefix (from `--prefix-color`) |
| `.Model` | string | Model display name (e.g., "Claude", "Sonnet 4") |
| `.Dir` | string | Current directory basename |
| `.GitBranch` | string | Current git branch (empty if not in repo) |
| `.GitStatus` | string | Change indicator like "±3" (empty if clean) |
| `.GitAdditions` | string | Line additions (e.g., "+42", empty if 0) |
| `.GitDeletions` | string | Line deletions (e.g., "-10", empty if 0) |
| `.GitNewFiles` | string | New files count (e.g., "✨2", empty if 0) |
| `.GitModifiedFiles` | string | Modified files count (e.g., "📝1", empty if 0) |
| `.GitDeletedFiles` | string | Deleted files count (e.g., "🗑1", empty if 0) |
| `.GitAdditionsRaw` | int | Raw additions count (for conditionals) |
| `.GitDeletionsRaw` | int | Raw deletions count |
| `.GitNewFilesRaw` | int | Raw new files count |
| `.GitModifiedFilesRaw` | int | Raw modified files count |
| `.GitDeletedFilesRaw` | int | Raw deleted files count |
| `.GitHubStatus` | string | CI status emoji (empty if unavailable) |
| `.Version` | string | Claude Code version |
| `.TokensInput` | string | Input tokens (formatted, e.g., "10.5k") |
| `.TokensOutput` | string | Output tokens (formatted) |
| `.TokensCached` | string | Cached tokens (formatted) |
| `.TokensTotal` | string | Total tokens (formatted) |
| `.ContextLength` | string | Context length (formatted) |
| `.ContextPctUse` | string | Usable context percentage - **used in default template** |
| `.ContextPct` | string | Context percentage of max tokens (for custom templates) |
| `.TokensInputRaw` | int64 | Raw input tokens (for conditionals) |
| `.TokensOutputRaw` | int64 | Raw output tokens |
| `.TokensCachedRaw` | int64 | Raw cached tokens |
| `.TokensTotalRaw` | int64 | Raw total tokens |
| `.ContextLengthRaw` | int64 | Raw context length |
| `.ContextPctUseRaw` | float64 | Raw usable context percentage (for `ctxColor`) - **used in default template** |
| `.ContextPctRaw` | float64 | Raw context percentage of max tokens |
| `.BeadsOpen` | string | Open issues (e.g., "3 open", empty if 0) |
| `.BeadsReady` | string | Ready issues (e.g., "2 ready", empty if 0) |
| `.BeadsInProgress` | string | In-progress issues (e.g., "1 wip", empty if 0) |
| `.BeadsBlocked` | string | Blocked issues (e.g., "1 blocked", empty if 0) |
| `.BeadsNextTask` | string | Title of next ready task (empty if none) |
| `.BeadsTotalRaw` | int | Raw total issues count |
| `.BeadsOpenRaw` | int | Raw open issues count |
| `.BeadsReadyRaw` | int | Raw ready issues count |
| `.BeadsInProgressRaw` | int | Raw in-progress count |
| `.BeadsBlockedRaw` | int | Raw blocked count |
| `.HasBeads` | bool | Whether beads system is available |

### Color Functions

| Function | Description |
|----------|-------------|
| `{{cyan}}` | Cyan color |
| `{{blue}}` | Blue color |
| `{{green}}` | Green color |
| `{{yellow}}` | Yellow color |
| `{{red}}` | Red color |
| `{{magenta}}` | Magenta color |
| `{{gray}}` | Gray color |
| `{{bold}}` | Bold text |
| `{{reset}}` | Reset formatting |
| `{{ctxColor .ContextPctUseRaw}}` | Dynamic color based on usable context: green (<50%), yellow (50-80%), red (>80%) |

### Example Templates

**Minimal:**
```
{{.Model}} {{.Dir}}{{if .GitBranch}} ({{.GitBranch}}){{end}}
```

**With colors, no emojis:**
```
{{cyan}}{{.Model}}{{reset}} {{blue}}{{.Dir}}{{reset}}{{if .GitBranch}} {{green}}{{.GitBranch}}{{reset}}{{if .GitStatus}} {{yellow}}{{.GitStatus}}{{reset}}{{end}}{{end}}
```

**Branch-focused:**
```
{{if .GitBranch}}{{green}}{{.GitBranch}}{{reset}}{{if .GitStatus}} {{red}}{{.GitStatus}}{{reset}}{{end}} {{.GitHubStatus}}{{else}}{{gray}}(no git){{reset}}{{end}}
```

**With full token metrics:**
```
{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{if .GitStatus}} {{.GitStatus}}{{end}}{{reset}}{{end}}{{if .TokensTotal}} | {{gray}}📈 In:{{.TokensInput}} Out:{{.TokensOutput}} Cache:{{.TokensCached}}{{reset}}{{end}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUseRaw}}📊 {{.ContextPctUse}}{{reset}}{{end}}
```

**Context-focused (shows usable context percentage):**
```
{{cyan}}[{{.Model}}]{{reset}} | {{.Dir}}{{if .ContextPctUse}} | {{ctxColor .ContextPctUseRaw}}Ctx: {{.ContextLength}} ({{.ContextPctUse}}){{reset}}{{end}}
```

**With git diff stats:**
```
{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .GitBranch}} | {{green}}🌿 {{.GitBranch}}{{reset}}{{if or .GitAdditionsRaw .GitDeletionsRaw}} {{green}}{{.GitAdditions}}{{reset}},{{red}}{{.GitDeletions}}{{reset}}{{end}}{{end}}
```

**Task-focused (for beads users):**
```
{{cyan}}[{{.Model}}]{{reset}} | {{blue}}📁 {{.Dir}}{{reset}}{{if .BeadsReady}} | {{yellow}}📋 {{.BeadsReady}}{{reset}}{{if .BeadsBlocked}}, {{red}}{{.BeadsBlocked}}{{reset}}{{end}}{{end}}
```

## GitHub Integration

claude-status shows CI/CD build status for GitHub repositories. It requires the [GitHub CLI](https://cli.github.com/) (`gh`) to be installed and authenticated.

### Setup

1. Install GitHub CLI: https://cli.github.com/
2. Authenticate: `gh auth login`
3. Set workflow name in config (defaults to `build_and_test`)

### How It Works

- Detects GitHub repos from git remote URL
- Fetches latest workflow run status via GitHub API
- Caches results based on TTL and git ref changes
- Uses `gh auth token` for authentication (no token config needed)

### Supported Workflow Matching

The `github_workflow` config matches by:
- Workflow name (e.g., `"CI"`)
- Workflow filename without extension (e.g., `"ci"` matches `ci.yml`)

## Caching

claude-status uses smart caching to minimize git and API calls:

| Data | Invalidation Strategy |
|------|----------------------|
| Git branch | `.git/HEAD` file modification time |
| Git status | `.git/index` file modification time |
| GitHub status | TTL-based (default 60s) + ref file mtime |

Cache location: `~/.cache/claude-status/cache.json`

## File Locations

Following [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html):

| Purpose | Path |
|---------|------|
| Config | `~/.config/claude-status/config.json` |
| Cache | `~/.cache/claude-status/cache.json` |
| Logs | `~/.local/share/claude-status/status_line.json` |

## Development

### Prerequisites

- Go 1.25+
- golangci-lint (for linting)

### Commands

```bash
make lint    # Run linter
make test    # Run tests with coverage
make build   # Build binary
make clean   # Remove build artifacts
make all     # lint + test + build
```

### Running Tests

```bash
go test -v ./...
```

### Project Structure

```
├── cmd/claude-status/    # Main entry point
├── internal/
│   ├── beads/            # Beads task tracking integration
│   ├── cache/            # File-based caching
│   ├── config/           # Configuration loading
│   ├── git/              # Git operations
│   ├── github/           # GitHub API client
│   ├── install/          # -install command logic
│   ├── status/           # Status data builder
│   ├── template/         # Template rendering
│   └── tokens/           # Token metrics parsing
├── testdata/             # Test fixtures
└── integration_test.go   # Integration tests
```

## Troubleshooting

### Status line not appearing

1. Check Claude Code settings has correct path
2. Verify binary is executable: `chmod +x claude-status`
3. Test manually: `echo '{"model":{"display_name":"Test"},"workspace":{"current_dir":"/tmp"}}' | claude-status`

### GitHub status not showing

1. Ensure `gh` CLI is installed and authenticated
2. Check workflow name matches your `.github/workflows/*.yml`
3. Verify you're in a GitHub repository

### Errors logged to stderr

claude-status logs errors to stderr but still outputs a fallback status line. Check stderr for debugging:

```bash
echo '{}' | claude-status 2>&1
```

## Comparison with ccstatusline

| Feature | claude-status | ccstatusline |
|---------|--------------|--------------|
| Language | Go | TypeScript/Node.js |
| Binary size | ~10MB | Requires Node.js runtime |
| Startup time | <1ms | ~100ms |
| Configuration | JSON file | Interactive TUI |
| Widgets | Template-based | Widget-based |
| Token metrics | ✅ Full support | ✅ Full support |
| Context tracking | ✅ Model-aware (1M/200k) | ✅ Model-aware |
| Git diff stats | ✅ Additions/deletions/files | ❌ |
| Task tracking | ✅ beads integration | ❌ |
| Multi-profile | ✅ `--prefix` flag | ❌ |
| Powerline fonts | Via template | Built-in support |

claude-status prioritizes speed and simplicity. If you need more widgets and interactive configuration, check out [ccstatusline](https://github.com/sirmalloc/ccstatusline).

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Run `make all` to ensure tests pass
4. Submit a pull request

## Credits

Inspired by [ccstatusline](https://github.com/sirmalloc/ccstatusline) by sirmalloc.
