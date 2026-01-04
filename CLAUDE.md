# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Use 'bd' for task tracking.

*Never* commit to main/master directly, always create a side branch. Code is merged via pull request.

## Build Commands

```bash
make all        # Full pipeline: lint → test → build
make lint       # Run golangci-lint
make test       # Run tests with race detection and coverage
make build      # Build binary to ./claude-status
```

Run a single test:
```bash
go test -v -run TestName ./internal/package/
```

## Architecture

**Data flow:** stdin (JSON) → config.Load() → status.Builder → template.Engine → stdout

The binary reads JSON from stdin containing model, workspace, version, and transcript path. It gathers status data and renders a formatted status line.

### Key Packages

- **cmd/claude-status** - Entry point, stdin parsing, error handling with fallback
- **internal/status** - Builder that orchestrates all status gathering
- **internal/git** - Git operations with 2-second timeout (branch, status, remote URL)
- **internal/github** - GitHub API client, uses `gh auth token` for authentication
- **internal/cache** - File-based JSON cache with mtime-based invalidation
- **internal/tokens** - Parses transcript JSONL for token metrics
- **internal/template** - Go template engine with ANSI color functions
- **internal/config** - XDG-compliant config loading and defaults

### Design Patterns

- Dependency injection via interfaces (Commander, HTTPClient, TokenGetter, Clock) for testing
- Cache invalidation keyed on file mtimes: `.git/HEAD` for branch, `.git/index` for status
- Model-aware context limits: 1M tokens for Sonnet 4.5 [1m], 200k for others

### File Locations (XDG)

- Config: `~/.config/claude-status/config.json`
- Cache: `~/.cache/claude-status/cache.json`
- Logs: `~/.local/share/claude-status/status_line.json`
