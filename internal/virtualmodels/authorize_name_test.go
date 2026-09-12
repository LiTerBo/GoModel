package virtualmodels

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/storage/sqlx/sqlxtest"
	"github.com/enterpilot/gomodel/internal/users"
)

// newUsersPolicy wires the real subject-side policy (auth-key and user-path
// allowlists) into the virtual-models service, exactly as the gateway does.
func newUsersPolicy(t *testing.T) *users.Service {
	t.Helper()
	store, err := users.NewSQLStore(context.Background(), sqlxtest.NewSQLite(t))
	if err != nil {
		t.Fatalf("users.NewSQLStore: %v", err)
	}
	policy, err := users.NewService(store, testCatalog())
	if err != nil {
		t.Fatalf("users.NewService: %v", err)
	}
	if err := policy.Refresh(context.Background()); err != nil {
		t.Fatalf("users Refresh: %v", err)
	}
	return policy
}

func exposedIDs(models []core.Model) map[string]bool {
	out := make(map[string]bool, len(models))
	for _, model := range models {
		out[model.ID] = true
	}
	return out
}

// A key whose allowlist names an alias sees that alias — and only it — even
// though its targets are concrete models the key is not allowed to name.
func TestExposedModels_AuthorizedNameAliasOnly(t *testing.T) {
	t.Parallel()
	svc, err := NewService(newSQLVMStore(t), testCatalog(), true)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.SetAccessPolicy(newUsersPolicy(t))

	ctx := context.Background()
	// "fast" and "sibling" share one concrete target; "smart" chains to "fast".
	for _, row := range []VirtualModel{
		{Source: "fast", Targets: []Target{{Provider: "openai", Model: "gpt-4o"}}, Enabled: true},
		{Source: "sibling", Targets: []Target{{Provider: "openai", Model: "gpt-4o"}}, Enabled: true},
		{Source: "smart", Targets: []Target{{Model: "fast"}}, Enabled: true},
	} {
		if err := svc.Upsert(ctx, row); err != nil {
			t.Fatalf("Upsert(%s): %v", row.Source, err)
		}
	}

	keyCtx := core.WithCredentialAllowedModels(ctx, []string{"smart"})
	allow := func(selector core.ModelSelector) bool { return svc.AllowsModel(keyCtx, selector) }
	// The name predicate is the authorizer probed with a name-only selector,
	// exactly as GET /v1/models does it.
	allowName := func(name string) bool {
		return svc.AllowsModel(keyCtx, core.ModelSelector{Model: name})
	}

	got := exposedIDs(svc.ExposedModelsForUserPathNamed("/", allow, allowName))
	if len(got) != 1 || !got["smart"] {
		t.Fatalf("exposed = %v, want exactly {smart}", got)
	}

	// Name-authorized exposure is opt-in: the legacy filtered list still hides
	// the alias when its concrete target is filtered out, so a target-shaped
	// predicate is never fooled into exposing an alias.
	if legacyFiltered := exposedIDs(svc.ExposedModelsFiltered(func(sel core.ModelSelector) bool {
		return sel.Provider != "openai"
	})); len(legacyFiltered) != 0 {
		t.Fatalf("ExposedModelsFiltered = %v, want none (target filtered out)", legacyFiltered)
	}

	// The concrete model behind the alias stays invisible to the same key.
	concrete := []core.Model{{ID: "openai/gpt-4o", Object: "model", OwnedBy: "openai"}}
	if kept := svc.FilterPublicModels(keyCtx, concrete); len(kept) != 0 {
		t.Fatalf("FilterPublicModels = %#v, want none for a name-only allowlist", kept)
	}

	// The alias answers as itself: no target provenance, alias creation time.
	exposed := svc.ExposedModelsForUserPathNamed("/", allow, allowName)
	if exposed[0].OwnedBy != "" {
		t.Fatalf("exposed OwnedBy = %q, want empty (no target provenance)", exposed[0].OwnedBy)
	}
	if exposed[0].Created == 0 {
		t.Fatal("exposed Created = 0, want the alias creation time")
	}

	// Target-level allowlists keep working exactly as before.
	legacyCtx := core.WithCredentialAllowedModels(ctx, []string{"openai/gpt-4o"})
	legacy := exposedIDs(svc.ExposedModelsForUserPath("/", func(selector core.ModelSelector) bool {
		return svc.AllowsModel(legacyCtx, selector)
	}))
	if !legacy["fast"] || !legacy["sibling"] || !legacy["smart"] {
		t.Fatalf("legacy exposed = %v, want fast, sibling and smart (unchanged)", legacy)
	}
}

// A key restricted to one alias must not see a sibling alias that shares the
// same concrete target, while an unrelated caller (no key) still sees both.
func TestExposedModels_NoSiblingLeakByName(t *testing.T) {
	t.Parallel()
	svc, err := NewService(newSQLVMStore(t), testCatalog(), true)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.SetAccessPolicy(newUsersPolicy(t))

	ctx := context.Background()
	for _, source := range []string{"smart", "other"} {
		row := VirtualModel{Source: source, Targets: []Target{{Provider: "openai", Model: "gpt-4o"}}, Enabled: true}
		if err := svc.Upsert(ctx, row); err != nil {
			t.Fatalf("Upsert(%s): %v", source, err)
		}
	}

	keyCtx := core.WithCredentialAllowedModels(ctx, []string{"smart"})
	got := exposedIDs(svc.ExposedModelsForUserPathNamed("/", func(selector core.ModelSelector) bool {
		return svc.AllowsModel(keyCtx, selector)
	}, func(name string) bool {
		return svc.AllowsModel(keyCtx, core.ModelSelector{Model: name})
	}))
	if !got["smart"] || got["other"] {
		t.Fatalf("exposed = %v, want {smart} only", got)
	}

	unrestricted := exposedIDs(svc.ExposedModelsForUserPath("/", func(selector core.ModelSelector) bool {
		return svc.AllowsModel(ctx, selector)
	}))
	if !unrestricted["smart"] || !unrestricted["other"] {
		t.Fatalf("unrestricted exposed = %v, want both", unrestricted)
	}
}
