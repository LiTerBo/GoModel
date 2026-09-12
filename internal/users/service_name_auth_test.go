package users

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

func TestService_AllowsModelByRequestedName(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	gpt := core.ModelSelector{Provider: "openai", Model: "gpt-4o"}

	named := func(ctx context.Context, name string) context.Context {
		return core.WithRequestedModelName(ctx, name)
	}

	// A credential restricted to an alias admits the request by name, whatever
	// the alias currently resolves to.
	keyCtx := requestCtx("", "smart")
	if !svc.AllowsModel(named(keyCtx, "smart"), gpt) {
		t.Fatal("AllowsModel(name=smart, resolved=openai/gpt-4o) = false, want true")
	}
	// A sibling alias is not admitted by the same allowlist.
	if svc.AllowsModel(named(keyCtx, "lite"), gpt) {
		t.Fatal("AllowsModel(name=lite) = true, want false (allowlist names smart only)")
	}
	// Without a requested name nothing changes: the selector path still denies.
	if svc.AllowsModel(keyCtx, gpt) {
		t.Fatal("AllowsModel(no name) = true, want false (name-only allowlist)")
	}

	// Layers still intersect. The key admits by name ...
	pathCtx := requestCtx("/agents/team1", "smart")
	if !svc.AllowsModel(named(pathCtx, "smart"), gpt) {
		t.Fatal("AllowsModel(key+no path policy) = false, want true")
	}
	// ... but a user-path policy naming something else must still deny.
	if _, err := svc.Upsert(context.Background(), User{UserPath: "/agents", AllowedModels: []string{"lite"}}); err != nil {
		t.Fatalf("Upsert(/agents): %v", err)
	}
	if svc.AllowsModel(named(pathCtx, "smart"), gpt) {
		t.Fatal("AllowsModel(name=smart, path policy=lite) = true, want false")
	}
	// ... and a key naming something else denies even when the path admits.
	keyOnly := requestCtx("/agents/team1", "lite")
	if svc.AllowsModel(named(keyOnly, "smart"), gpt) {
		t.Fatal("AllowsModel(key=lite, path=lite, name=smart) = true, want false")
	}
	// Both layers admitting the same name passes.
	both := requestCtx("/agents/team1", "lite")
	if !svc.AllowsModel(named(both, "lite"), gpt) {
		t.Fatal("AllowsModel(key+path = lite, name = lite) = false, want true")
	}
}
