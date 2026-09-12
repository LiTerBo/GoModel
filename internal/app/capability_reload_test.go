package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/providers"
	"github.com/enterpilot/gomodel/internal/storage"
)

// capabilityGeneration is one application generation for the reload test: the
// bootstrap the phases hang off, the registry that generation built, and the
// storage backend it shares with its siblings.
type capabilityGeneration struct {
	bootstrap *bootstrap
	registry  *providers.ModelRegistry
}

// openCapabilityGeneration builds one generation the way App construction does:
// open the configured storage, build and initialize the registry, then run the
// capability phase against both.
func openCapabilityGeneration(t *testing.T, ctx context.Context, dbPath string) *capabilityGeneration {
	t.Helper()

	backend, err := storage.NewSQLite(storage.SQLiteConfig{Path: dbPath})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })

	registry := providers.NewModelRegistry()
	registry.RegisterProviderWithNameAndType(&runtimeRefreshMockProvider{
		models: &core.ModelsResponse{
			Object: "list",
			Data:   []core.Model{{ID: "my-llm", Object: "model", OwnedBy: "local"}},
		},
	}, "local", "openai-compatible")
	if err := registry.Initialize(ctx); err != nil {
		t.Fatalf("registry Initialize: %v", err)
	}

	b := &bootstrap{ctx: ctx, app: &App{
		storage:   backend,
		providers: &providers.InitResult{Registry: registry},
	}}
	if err := b.initCapabilities(); err != nil {
		t.Fatalf("initCapabilities: %v", err)
	}
	t.Cleanup(func() { _ = b.app.capabilities.Close() })

	return &capabilityGeneration{bootstrap: b, registry: registry}
}

// assertReplayedCapability fails unless the registry carries the operator's
// confirmed verdict, applied by replay rather than by a fresh confirmation.
func assertReplayedCapability(t *testing.T, registry *providers.ModelRegistry) {
	t.Helper()

	model, ok := registry.LookupModel("local/my-llm")
	if !ok {
		t.Fatal("local/my-llm missing from registry")
	}
	if model.Metadata == nil || !model.Metadata.Capabilities["function_calling"] {
		t.Fatalf("confirmed capability not present: %+v", model.Metadata)
	}
	if got := model.Metadata.CapabilitySources["function_calling"]; got != core.CapSrcTest {
		t.Fatalf("capability source = %q, want %q", got, core.CapSrcTest)
	}
}

// TestCapabilityConfirmationsSurviveRebuild pins the reload half of the
// confirmation lifecycle, not just the restart half. run.Run rebuilds a whole
// application for every configuration generation it serves, App construction
// runs bootstrap.phases() again, and initCapabilities replays what the previous
// generation persisted into the registry the new generation just built. So a
// reload must come up with confirmed verdicts applied, with no new Confirm
// call — that replay is the reason the phase exists, and it happens on every
// build rather than only on process start.
func TestCapabilityConfirmationsSurviveRebuild(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "capability.db")

	// First generation: the operator confirms a verdict through the service
	// this generation built.
	first := openCapabilityGeneration(t, ctx, dbPath)
	if err := first.bootstrap.app.capabilities.Service.Confirm(
		ctx, "local", "my-llm", map[string]bool{"function_calling": true}, core.CapSrcTest,
	); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	assertReplayedCapability(t, first.registry)

	// Second generation: a reload. Same database file, a registry that never
	// saw the confirmation.
	second := openCapabilityGeneration(t, ctx, dbPath)
	assertReplayedCapability(t, second.registry)
}
