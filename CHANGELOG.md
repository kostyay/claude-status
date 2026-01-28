# Changelog

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
