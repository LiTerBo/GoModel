package capability

import (
	"context"
	"io"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/providers"
	"github.com/enterpilot/gomodel/internal/storage/sqlx"
	"github.com/enterpilot/gomodel/internal/storage/sqlx/sqlxtest"
)

// localMockProvider is a minimal core.Provider for registry seeding: it
// answers ListModels with one model and rejects everything else. Mirrors the
// mock shape the providers package's own registry tests use.
type localMockProvider struct{}

func (m *localMockProvider) ChatCompletion(_ context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
	return nil, core.NewInvalidRequestError("not supported", nil)
}

func (m *localMockProvider) StreamChatCompletion(_ context.Context, _ *core.ChatRequest) (io.ReadCloser, error) {
	return nil, core.NewInvalidRequestError("not supported", nil)
}

func (m *localMockProvider) ListModels(_ context.Context) (*core.ModelsResponse, error) {
	return &core.ModelsResponse{
		Object: "list",
		Data:   []core.Model{{ID: "my-llm", Object: "model", OwnedBy: "local"}},
	}, nil
}

func (m *localMockProvider) Responses(_ context.Context, _ *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return nil, core.NewInvalidRequestError("not supported", nil)
}

func (m *localMockProvider) StreamResponses(_ context.Context, _ *core.ResponsesRequest) (io.ReadCloser, error) {
	return nil, core.NewInvalidRequestError("not supported", nil)
}

func (m *localMockProvider) Embeddings(_ context.Context, _ *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return nil, core.NewInvalidRequestError("not supported", nil)
}

// TestRestartReplayEndToEnd drives the production write/read path — a real
// SQLStore over a real database and a real *providers.ModelRegistry — through
// a confirm-then-restart cycle: confirm on the first "process", close it,
// build a brand-new store+service against the same database (the replay half
// of the factory's New), and verify the registry recovered the confirmation
// with its source intact.
//
// The backend-dispatch half of the factory (storage.ResolveSQLBackend) is
// covered by the dialect matrix in the store suite above; this test pins the
// persistence semantics, which are backend-independent by construction.
func TestRestartReplayEndToEnd(t *testing.T) {
	sqlxtest.Run(t, func(t *testing.T, db sqlx.DB) {
		ctx := context.Background()

		registry := providers.NewModelRegistry()
		registry.RegisterProviderWithNameAndType(&localMockProvider{}, "local", "openai-compatible")
		if err := registry.Initialize(ctx); err != nil {
			t.Fatalf("registry Initialize: %v", err)
		}

		// First "process": confirm function_calling through the full service.
		store, err := NewSQLStore(ctx, db)
		if err != nil {
			t.Fatalf("first NewSQLStore: %v", err)
		}
		first := NewService(store, registry)
		if err := first.Confirm(ctx, "local", "my-llm", map[string]bool{"function_calling": true}, core.CapSrcTest); err != nil {
			t.Fatalf("first Confirm: %v", err)
		}

		// Second "process": a fresh store+service over the same database and
		// a registry that never saw the confirmation. Refresh (the startup
		// load) must restore it without any new Confirm call.
		replayStore, err := NewSQLStore(ctx, db)
		if err != nil {
			t.Fatalf("second NewSQLStore: %v", err)
		}
		defer replayStore.Close()
		replay := NewService(replayStore, registry)
		if err := replay.Refresh(ctx); err != nil {
			t.Fatalf("Refresh: %v", err)
		}

		model, ok := registry.LookupModel("local/my-llm")
		if !ok {
			t.Fatal("model missing from registry after replay")
		}
		if model.Metadata == nil || !model.Metadata.Capabilities["function_calling"] {
			t.Fatalf("capability not replayed after restart: %+v", model.Metadata)
		}
		if got := model.Metadata.CapabilitySources["function_calling"]; got != core.CapSrcTest {
			t.Fatalf("capability source = %q, want %q", got, core.CapSrcTest)
		}
	})
}
