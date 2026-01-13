package beads

import "github.com/kostyay/claude-status/internal/tasks"

func init() {
	tasks.RegisterWithPriority(tasks.PriorityBeads, func(workDir string) tasks.Provider {
		return NewClient(workDir)
	})
}
