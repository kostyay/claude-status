# Changelog

## remove-beads

Removed the beads task provider and all `bd` CLI integration. The supported
task trackers are now Claude Code tasks, kt, and tk.

## feat/debug-logging-remove-task-cache

Debug logging for workspace diagnosis and task caching removal (#16).
Stdin parsing now logs workDir, sessionID, and model to stderr for
troubleshooting. Task results are fetched directly from providers instead
of going through the cache layer, simplifying the data path. Context
window documentation updated to reflect Opus 4.6 support alongside
Sonnet 4.5 and Sonnet 4 for the [1m] beta.

## Unreleased

### Added
- Claude Code task provider - reads tasks from `~/.claude/tasks/{task_list_id}/`
- `CLAUDE_CODE_TASK_LIST_ID` env var - shared task list across sessions
- `CLAUDE_PROFILE` env var support - uses `~/.claude-{profile}/tasks/` when set
- Claude tasks have highest priority (5) over kt (10), tk (20), beads (30)
- `{{.TaskListID}}` template variable - shows task list ID when using shared list

### Changed
- Task provider registry now passes sessionID to factories
- `NewBuilder` signature: added sessionID parameter

### Removed
- bd git hooks reference from CLAUDE.md
