package kt

import "github.com/kostyay/claude-status/internal/tasks"

func init() {
	tasks.RegisterWithPriority(tasks.PriorityKT, func(workDir string) tasks.Provider {
		return NewClient(workDir)
	})
}
