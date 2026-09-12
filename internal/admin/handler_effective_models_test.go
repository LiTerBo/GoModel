package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/enterpilot/gomodel/internal/authkeys"
	"github.com/enterpilot/gomodel/internal/storage/sqlx/sqlxtest"
	"github.com/enterpilot/gomodel/internal/users"
	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// T12: effective_models answers "what can a request with this credential
// actually call", so it must list the virtual models the credential is
// authorized for by name — the same set GET /v1/models returns for it.
// The admin rows used to walk the concrete catalog only, so a key restricted
// to an alias name reported an empty list.

func newEffectiveModelsHandler(t *testing.T, vms ...virtualmodels.VirtualModel) (*Handler, *authkeys.Service) {
	t.Helper()
	ctx := context.Background()
	registry := newVMModelRegistry(t) // openai/gpt-4o only
	store, err := users.NewSQLStore(ctx, sqlxtest.NewSQLite(t))
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	userService, err := users.NewService(store, registry)
	if err != nil {
		t.Fatalf("users.NewService: %v", err)
	}
	if err := userService.Refresh(ctx); err != nil {
		t.Fatalf("users.Refresh: %v", err)
	}
	vmService := newVMServiceForRegistry(t, registry, true, vms...)
	vmService.SetAccessPolicy(userService)
	now := time.Now().UTC()
	keyService, err := authkeys.NewService(newAuthKeyTestStore(
		authkeys.AuthKey{ID: "alias", Name: "alias", UserPath: "/acme", AllowedModels: []string{"smart"}, SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
		authkeys.AuthKey{ID: "wildcard", Name: "wildcard", UserPath: "/acme", AllowedModels: []string{"openai/"}, SecretHash: "h2", Enabled: true, CreatedAt: now, UpdatedAt: now},
		authkeys.AuthKey{ID: "nomatch", Name: "nomatch", UserPath: "/acme", AllowedModels: []string{"paused", "elsewhere"}, SecretHash: "h3", Enabled: true, CreatedAt: now, UpdatedAt: now},
	))
	if err != nil {
		t.Fatalf("authkeys.NewService: %v", err)
	}
	if err := keyService.Refresh(ctx); err != nil {
		t.Fatalf("authkeys.Refresh: %v", err)
	}
	h := NewHandler(nil, registry, WithUsers(userService), WithAuthKeys(keyService), WithVirtualModels(vmService))
	return h, keyService
}

func aliasRows(t *testing.T, h *Handler) map[string]authKeyResponse {
	t.Helper()
	c, rec := jsonRequest(http.MethodGet, "/admin/auth-keys", "")
	if err := h.ListAuthKeys(c); err != nil {
		t.Fatalf("ListAuthKeys error = %v", err)
	}
	var rows []authKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body.String())
	}
	byID := map[string]authKeyResponse{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	return byID
}

func smartAlias() virtualmodels.VirtualModel {
	return virtualmodels.VirtualModel{
		Source:  "smart",
		Targets: []virtualmodels.Target{{Provider: "openai", Model: "gpt-4o"}},
		Enabled: true,
	}
}

func TestListAuthKeysEffectiveModelsIncludeNamedAlias(t *testing.T) {
	h, _ := newEffectiveModelsHandler(t, smartAlias())

	byID := aliasRows(t, h)
	// The key is authorized for the alias by name; its targets are not listed.
	if r := byID["alias"]; !r.Restricted || !reflect.DeepEqual(r.EffectiveModels, []string{"smart"}) {
		t.Fatalf("alias key = %#v, want effective [smart]", r)
	}
	// A target-level wildcard key keeps its concrete model and also sees the
	// alias it exposes through the leaf rule — exactly what /v1/models returns.
	if r := byID["wildcard"]; !r.Restricted || !reflect.DeepEqual(r.EffectiveModels, []string{"openai/gpt-4o", "smart"}) {
		t.Fatalf("wildcard key = %#v, want effective [openai/gpt-4o smart]", r)
	}
	// Entries naming aliases that cannot serve for this credential stay empty.
	if r := byID["nomatch"]; !r.Restricted || len(r.EffectiveModels) != 0 || r.EffectiveModels == nil {
		t.Fatalf("nomatch key = %#v, want empty non-nil effective list", r)
	}
}

func TestListAuthKeysEffectiveModelsSkipPausedAndScopedAliases(t *testing.T) {
	paused := smartAlias()
	paused.Source = "paused"
	paused.Enabled = false
	elsewhere := smartAlias()
	elsewhere.Source = "elsewhere"
	elsewhere.UserPaths = []string{"/sales"}
	h, _ := newEffectiveModelsHandler(t, smartAlias(), paused, elsewhere)

	byID := aliasRows(t, h)
	// The key names all three; only the serving, in-scope one is reported.
	if r := byID["nomatch"]; !r.Restricted || len(r.EffectiveModels) != 0 {
		t.Fatalf("nomatch key = %#v, want no effective models", r)
	}
	if r := byID["alias"]; !reflect.DeepEqual(r.EffectiveModels, []string{"smart"}) {
		t.Fatalf("alias key = %#v, want effective [smart]", r)
	}
}

func TestUsersTreeEffectiveModelsIncludeNamedAlias(t *testing.T) {
	scoped := smartAlias()
	scoped.Source = "solo"
	scoped.UserPaths = []string{"/acme"}
	h, _ := newEffectiveModelsHandler(t, smartAlias(), scoped)

	c, _ := jsonRequest(http.MethodPut, "/admin/users", `{"user_path":"/acme/eng","allowed_models":["smart"]}`)
	if err := h.UpsertUser(c); err != nil {
		t.Fatalf("UpsertUser error = %v", err)
	}
	c, _ = jsonRequest(http.MethodPut, "/admin/users", `{"user_path":"/sales","allowed_models":["smart"]}`)
	if err := h.UpsertUser(c); err != nil {
		t.Fatalf("UpsertUser(sales) error = %v", err)
	}
	c, rec := jsonRequest(http.MethodPut, "/admin/users", `{"user_path":"/sales/kiosk","allowed_models":["solo"]}`)
	if err := h.UpsertUser(c); err != nil {
		t.Fatalf("UpsertUser(kiosk) error = %v", err)
	}

	nodes := decodeUsers(t, rec)
	if n := nodes["/acme/eng"]; !n.Restricted || !reflect.DeepEqual(n.EffectiveModels, []string{"smart"}) {
		t.Fatalf("/acme/eng = %#v, want effective [smart]", n)
	}
	// The alias carries no user_paths of its own, so any path may address it.
	if n := nodes["/sales"]; !n.Restricted || !reflect.DeepEqual(n.EffectiveModels, []string{"smart"}) {
		t.Fatalf("/sales = %#v, want effective [smart]", n)
	}
	// A scoped alias stays invisible to paths outside its scope.
	if n := nodes["/sales/kiosk"]; !n.Restricted || len(n.EffectiveModels) != 0 || n.EffectiveModels == nil {
		t.Fatalf("/sales/kiosk = %#v, want empty non-nil effective list", n)
	}
}
