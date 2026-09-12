package providers

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/modeldata"
)

// overrideSourceRegistry builds a registry whose provider reports one model with
// the given metadata, plus an empty model list so the config metadata overrides
// are the only enrichment input.
func overrideSourceRegistry(t *testing.T, reported *core.ModelMetadata) *ModelRegistry {
	t.Helper()

	registry := NewModelRegistry()
	registry.RegisterProviderWithNameAndType(&registryMockProvider{
		name: "provider-nippur",
		modelsResponse: &core.ModelsResponse{
			Object: "list",
			Data: []core.Model{{
				ID: "GLM-4.7-Flash", Object: "model", OwnedBy: "ollama", Metadata: reported,
			}},
		},
	}, "nippur", "ollama")

	raw := []byte(`{"version":1,"updated_at":"2025-01-01T00:00:00Z","providers":{},"models":{},"provider_models":{}}`)
	list, err := modeldata.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	registry.SetModelList(list, raw)
	return registry
}

func publishedMetadata(t *testing.T, registry *ModelRegistry, modelID string) *core.ModelMetadata {
	t.Helper()

	info := registry.GetModel(modelID)
	if info == nil || info.Model.Metadata == nil {
		t.Fatalf("%s has no published metadata", modelID)
	}
	return info.Model.Metadata
}

// TestConfigMetadataOverrides_StampCapabilitySources covers the capability side
// of what the pricing assertions in registry_metadata_override_test.go already
// cover: a capability declared in config.yaml is an operator declaration, so the
// published model should say so. core.CapSrcConfig was defined for this and
// nothing wrote it, while the pricing branch records its own config source in
// the same function - the asymmetry is why this is a gap and not a design.
func TestConfigMetadataOverrides_StampCapabilitySources(t *testing.T) {
	registry := overrideSourceRegistry(t, &core.ModelMetadata{
		Capabilities: map[string]bool{"function_calling": true},
	})
	registry.SetProviderMetadataOverrides("nippur", map[string]*core.ModelMetadata{
		"GLM-4.7-Flash": {Capabilities: map[string]bool{"vision": true}},
	})

	if err := registry.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	meta := publishedMetadata(t, registry, "nippur/GLM-4.7-Flash")
	if !meta.Capabilities["vision"] {
		t.Errorf("vision = false, want the config-declared true")
	}
	if got := meta.CapabilitySources["vision"]; got != core.CapSrcConfig {
		t.Errorf("config-declared vision source = %q, want %q", got, core.CapSrcConfig)
	}
	// A capability the config did not touch keeps the origin it already had.
	if got := meta.CapabilitySources["function_calling"]; got != core.CapSrcDiscovered {
		t.Errorf("provider-reported function_calling source = %q, want %q", got, core.CapSrcDiscovered)
	}
}

// TestConfigMetadataOverrides_KeepSourcedKeysAsDeclared pins the shared-channel
// invariant: operator confirmations ride this same override map and arrive with
// their own source. Stamping config over them would relabel a verification as a
// plain declaration and lose the distinction the dashboard renders.
func TestConfigMetadataOverrides_KeepSourcedKeysAsDeclared(t *testing.T) {
	registry := overrideSourceRegistry(t, &core.ModelMetadata{
		Capabilities: map[string]bool{"vision": true},
	})
	registry.SetProviderMetadataOverrides("nippur", map[string]*core.ModelMetadata{
		"GLM-4.7-Flash": {
			Capabilities:      map[string]bool{"vision": false},
			CapabilitySources: map[string]string{"vision": core.CapSrcTest},
		},
	})

	if err := registry.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	meta := publishedMetadata(t, registry, "nippur/GLM-4.7-Flash")
	if meta.Capabilities["vision"] {
		t.Errorf("vision = true, want the confirmed false")
	}
	if got := meta.CapabilitySources["vision"]; got != core.CapSrcTest {
		t.Errorf("vision source = %q, want %q", got, core.CapSrcTest)
	}
}
