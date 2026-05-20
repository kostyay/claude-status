package tasks

import (
	"log/slog"
	"sort"
)

// ProviderFactory creates a Provider for a given working directory and session.
type ProviderFactory func(workDir, sessionID string) Provider

// registeredProvider holds a factory with its priority.
type registeredProvider struct {
	factory  ProviderFactory
	priority int
}

// registry holds provider factories ordered by priority (lower = higher priority).
var registry []registeredProvider

// RegisterWithPriority adds a provider factory with a specific priority.
// Lower priority values are checked first. Use constants:
// PriorityClaude=5, PriorityKT=10, PriorityTK=20
func RegisterWithPriority(priority int, factory ProviderFactory) {
	registry = append(registry, registeredProvider{factory: factory, priority: priority})
	// Keep sorted by priority
	sort.Slice(registry, func(i, j int) bool {
		return registry[i].priority < registry[j].priority
	})
}

// Priority constants for task providers.
const (
	PriorityClaude = 5  // claude tasks has highest priority
	PriorityKT     = 10 // kt has second priority
	PriorityTK     = 20 // tk has third priority
)

// SelectProvider returns the first available provider for the working directory and session.
// Returns nil if no provider is available.
func SelectProvider(workDir, sessionID string) Provider {
	for _, rp := range registry {
		provider := rp.factory(workDir, sessionID)
		if provider.Available() {
			slog.Debug("using task tracker", "provider", provider.Name(), "workDir", workDir, "sessionID", sessionID)
			return provider
		}
	}
	slog.Debug("no task tracker found", "workDir", workDir, "sessionID", sessionID)
	return nil
}
