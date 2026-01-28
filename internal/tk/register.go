package tk

import "github.com/kostyay/claude-status/internal/tasks"

func init() {
	tasks.RegisterWithPriority(tasks.PriorityTK, func(workDir, _ string) tasks.Provider {
		return NewClient(workDir)
	})
}
