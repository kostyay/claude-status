package tasks

import (
	"log/slog"
)

// ProviderFactory creates a Provider for a given working directory.
type ProviderFactory func(workDir string) Provider

// registry holds the ordered list of provider factories.
// Priority order: kt > tk > beads (first available wins).
var registry []ProviderFactory

// Register adds a provider factory to the registry.
// Providers are checked in registration order.
func Register(factory ProviderFactory) {
	registry = append(registry, factory)
}

// SelectProvider returns the first available provider for the working directory.
// Returns nil if no provider is available.
func SelectProvider(workDir string) Provider {
	for _, factory := range registry {
		provider := factory(workDir)
		if provider.Available() {
			slog.Debug("using task tracker", "provider", provider.Name(), "workDir", workDir)
			return provider
		}
	}
	slog.Debug("no task tracker found", "workDir", workDir)
	return nil
}
