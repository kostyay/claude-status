package kt

import "github.com/kostyay/claude-status/internal/tasks"

func init() {
	// Register kt with highest priority (registered first, checked first)
	tasks.Register(func(workDir string) tasks.Provider {
		return NewClient(workDir)
	})
}
