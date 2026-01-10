package tk

import "github.com/kostyay/claude-status/internal/tasks"

func init() {
	// Register tk with second priority (after kt)
	tasks.Register(func(workDir string) tasks.Provider {
		return NewClient(workDir)
	})
}
