package beads

import "github.com/kostyay/claude-status/internal/tasks"

func init() {
	// Register beads with lowest priority (after kt and tk)
	tasks.Register(func(workDir string) tasks.Provider {
		return NewClient(workDir)
	})
}
