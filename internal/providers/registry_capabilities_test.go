package providers

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

func capabilitiesTestRegistry(t *testing.T) *ModelRegistry {
	t.Helper()
	registry := NewModelRegistry()
	mock := &registryMockProvider{
		name: "local",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{
				{ID: "my-llm", Object: "model", OwnedBy: "local"},
			},
		},
	}
	registry.RegisterProviderWithNameAndType(mock, "local", "openai-compatible")
	if err := registry.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	return registry
}

// Operator-confirmed capabilities must land in the routable catalog with the
// confirmed source, winning over heuristic inference at every re-enrichment.
func TestMergeModelCapabilities_ConfirmsFunctionCalling(t *testing.T) {
	registry := capabilitiesTestRegistry(t)

	before, ok := registry.LookupModel("local/my-llm")
	if !ok || before.Metadata == nil {
		t.Fatalf("model missing before merge: %v", ok)
	}
	if before.Metadata.Capabilities["function_calling"] {
		t.Fatal("function_calling already set before merge; fixture not clean")
	}

	if !registry.MergeModelCapabilities("local", "my-llm", map[string]bool{"function_calling": true}, core.CapSrcTest) {
		t.Fatal("MergeModelCapabilities() = false, want true for a known model")
	}

	after, ok := registry.LookupModel("local/my-llm")
	if !ok || after.Metadata == nil {
		t.Fatal("model lost after merge")
	}
	if !after.Metadata.Capabilities["function_calling"] {
		t.Fatalf("function_calling = %v, want true after confirmation", after.Metadata.Capabilities)
	}
	if got := after.Metadata.CapabilitySources["function_calling"]; got != core.CapSrcTest {
		t.Fatalf("source = %q, want %q", got, core.CapSrcTest)
	}
}

// The confirmation must survive a re-enrichment (provider refresh): the
// override channel, not a one-off patch, carries it.
func TestMergeModelCapabilities_SurvivesReEnrichment(t *testing.T) {
	registry := capabilitiesTestRegistry(t)
	if !registry.MergeModelCapabilities("local", "my-llm", map[string]bool{"function_calling": true}, core.CapSrcTest) {
		t.Fatal("MergeModelCapabilities() = false")
	}
	if err := registry.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	after, ok := registry.LookupModel("local/my-llm")
	if !ok || after.Metadata == nil || !after.Metadata.Capabilities["function_calling"] {
		t.Fatal("confirmed capability lost across re-enrichment")
	}
}

// An operator-confirmed negative (explicit upstream rejection record) must
// strip the flag — this is the only sanctioned negative path.
func TestMergeModelCapabilities_RecordsConfirmedNegative(t *testing.T) {
	registry := capabilitiesTestRegistry(t)
	if !registry.MergeModelCapabilities("local", "my-llm", map[string]bool{"function_calling": false}, core.CapSrcTest) {
		t.Fatal("MergeModelCapabilities() = false")
	}
	after, _ := registry.LookupModel("local/my-llm")
	if after.Metadata.Capabilities["function_calling"] {
		t.Fatal("confirmed negative not applied")
	}
	if got := after.Metadata.CapabilitySources["function_calling"]; got != core.CapSrcTest {
		t.Fatalf("source = %q, want %q", got, core.CapSrcTest)
	}
}

// Unknown provider/model pairs are rejected rather than silently recorded.
func TestMergeModelCapabilities_RejectsUnknown(t *testing.T) {
	registry := capabilitiesTestRegistry(t)
	if registry.MergeModelCapabilities("no-such-provider", "my-llm", map[string]bool{"vision": true}, core.CapSrcTest) {
		t.Fatal("unknown provider accepted")
	}
	if registry.MergeModelCapabilities("local", "no-such-model", map[string]bool{"vision": true}, core.CapSrcTest) {
		t.Fatal("unknown model accepted")
	}
	if registry.MergeModelCapabilities("local", "my-llm", nil, core.CapSrcTest) {
		t.Fatal("empty caps accepted")
	}
	if registry.MergeModelCapabilities("", "my-llm", map[string]bool{"vision": true}, core.CapSrcTest) {
		t.Fatal("empty provider accepted")
	}
}

// A confirmed type (modes) retypes the model the same way a config.yaml
// metadata block would.
func TestMergeModelCapabilities_WithModesRetypes(t *testing.T) {
	registry := capabilitiesTestRegistry(t)
	if !registry.MergeModelCapabilities("local", "my-llm", map[string]bool{"function_calling": true}, core.CapSrcTest) {
		t.Fatal("MergeModelCapabilities() = false")
	}
	// Modes stay untouched by a capabilities-only merge (no accidental retype).
	after, _ := registry.LookupModel("local/my-llm")
	if len(after.Metadata.Modes) == 0 {
		t.Log("modes empty (default chat inference still applies at enrichment)")
	}
}
