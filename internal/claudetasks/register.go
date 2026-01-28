package claudetasks

import "github.com/kostyay/claude-status/internal/tasks"

func init() {
	tasks.RegisterWithPriority(tasks.PriorityClaude, func(_, sessionID string) tasks.Provider {
		return NewClient(sessionID)
	})
}
