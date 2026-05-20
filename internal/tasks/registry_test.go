package tasks

import (
	"testing"
)

func TestRegisterWithPriority_Order(t *testing.T) {
	// Save and restore original registry
	origRegistry := registry
	registry = nil
	defer func() { registry = origRegistry }()

	// Register in reverse priority order
	RegisterWithPriority(30, func(workDir, sessionID string) Provider { return nil })
	RegisterWithPriority(10, func(workDir, sessionID string) Provider { return nil })
	RegisterWithPriority(20, func(workDir, sessionID string) Provider { return nil })

	// Verify they're sorted by priority
	if len(registry) != 3 {
		t.Fatalf("expected 3 registered providers, got %d", len(registry))
	}

	expected := []int{10, 20, 30}
	for i, rp := range registry {
		if rp.priority != expected[i] {
			t.Errorf("registry[%d].priority = %d, want %d", i, rp.priority, expected[i])
		}
	}
}

type mockProvider struct {
	name      string
	available bool
}

func (m *mockProvider) Name() string                 { return m.name }
func (m *mockProvider) Available() bool              { return m.available }
func (m *mockProvider) GetStats() (Stats, error)     { return Stats{}, nil }
func (m *mockProvider) GetNextTask() (string, error) { return "", nil }

func TestSelectProvider_Priority(t *testing.T) {
	// Save and restore original registry
	origRegistry := registry
	registry = nil
	defer func() { registry = origRegistry }()

	// Register providers: kt (available), tk (available)
	// kt should win due to higher priority
	RegisterWithPriority(PriorityKT, func(workDir, sessionID string) Provider {
		return &mockProvider{name: "kt", available: true}
	})
	RegisterWithPriority(PriorityTK, func(workDir, sessionID string) Provider {
		return &mockProvider{name: "tk", available: true}
	})

	provider := SelectProvider("/test", "")
	if provider == nil {
		t.Fatal("SelectProvider returned nil")
	}
	if provider.Name() != "kt" {
		t.Errorf("SelectProvider() = %q, want %q", provider.Name(), "kt")
	}
}

func TestSelectProvider_Fallback(t *testing.T) {
	// Save and restore original registry
	origRegistry := registry
	registry = nil
	defer func() { registry = origRegistry }()

	// Register providers: kt (unavailable), tk (available)
	// tk should be selected as fallback
	RegisterWithPriority(PriorityKT, func(workDir, sessionID string) Provider {
		return &mockProvider{name: "kt", available: false}
	})
	RegisterWithPriority(PriorityTK, func(workDir, sessionID string) Provider {
		return &mockProvider{name: "tk", available: true}
	})

	provider := SelectProvider("/test", "")
	if provider == nil {
		t.Fatal("SelectProvider returned nil")
	}
	if provider.Name() != "tk" {
		t.Errorf("SelectProvider() = %q, want %q", provider.Name(), "tk")
	}
}

func TestSelectProvider_None(t *testing.T) {
	// Save and restore original registry
	origRegistry := registry
	registry = nil
	defer func() { registry = origRegistry }()

	// Register providers: all unavailable
	RegisterWithPriority(PriorityKT, func(workDir, sessionID string) Provider {
		return &mockProvider{name: "kt", available: false}
	})

	provider := SelectProvider("/test", "")
	if provider != nil {
		t.Errorf("SelectProvider() = %v, want nil", provider)
	}
}
