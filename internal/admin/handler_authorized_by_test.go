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

// T13-B: the read-only impact endpoint answers "who is affected if this alias
// points somewhere else" for the alias editor's pre-save preview. It reuses the
// in-memory key and user snapshots and the request-time scope rule, so a
// preview cannot disagree with who can actually address the alias.

// newAuthorizedByHandler builds a handler with explicit keys, so scope
// behaviour can be exercised from both sides (a key inside the alias scope and
// one outside it).
func newAuthorizedByHandler(t *testing.T, keys []authkeys.AuthKey, vms ...virtualmodels.VirtualModel) *Handler {
	t.Helper()
	ctx := context.Background()
	registry := newVMModelRegistry(t)
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
	keyService, err := authkeys.NewService(newAuthKeyTestStore(keys...))
	if err != nil {
		t.Fatalf("authkeys.NewService: %v", err)
	}
	if err := keyService.Refresh(ctx); err != nil {
		t.Fatalf("authkeys.Refresh: %v", err)
	}
	return NewHandler(nil, registry, WithUsers(userService), WithAuthKeys(keyService), WithVirtualModels(vmService))
}

func authorizedBy(t *testing.T, h *Handler, query string) (authorizedByResponse, int) {
	t.Helper()
	c, rec := jsonRequest(http.MethodGet, "/admin/virtual-models/authorized-by"+query, "")
	if err := h.AuthorizedByVirtualModel(c); err != nil {
		t.Fatalf("AuthorizedByVirtualModel error = %v", err)
	}
	if rec.Code != http.StatusOK {
		return authorizedByResponse{}, rec.Code
	}
	var resp authorizedByResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body.String())
	}
	return resp, rec.Code
}

// mustUpsertUser writes a policy and fails unless the admin endpoint accepted
// it. Admin handlers report failures by writing a response, so checking the
// returned error alone would silently accept a rejected policy.
func mustUpsertUser(t *testing.T, h *Handler, body string) {
	t.Helper()
	c, rec := jsonRequest(http.MethodPut, "/admin/users", body)
	if err := h.UpsertUser(c); err != nil {
		t.Fatalf("UpsertUser(%s) error = %v", body, err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("UpsertUser(%s) status = %d, want 200 body=%s", body, rec.Code, rec.Body.String())
	}
}

func TestAuthorizedByVirtualModelReportsFollowersAndPotentialGrantees(t *testing.T) {
	h, _ := newEffectiveModelsHandler(t, smartAlias())
	// A user path that names the alias follows it just like a credential does.
	mustUpsertUser(t, h, `{"user_path":"/acme/eng","allowed_models":["smart"]}`)

	resp, code := authorizedBy(t, h, "?source=smart")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	want := []authorizedByGrant{
		{Kind: "credential", ID: "alias", Label: "alias", Path: "/acme", Change: ImpactFollow, MatchedBy: "name", Matched: []string{"smart"}},
		{Kind: "user_path", Label: "/acme/eng", Path: "/acme/eng", Change: ImpactFollow, MatchedBy: "name", Matched: []string{"smart"}},
		{Kind: "credential", ID: "wildcard", Label: "wildcard", Path: "/acme", Change: ImpactPotential, MatchedBy: "selector", Matched: []string{"openai/"}},
	}
	if !reflect.DeepEqual(resp.Grants, want) {
		t.Fatalf("grants = %#v, want %#v", resp.Grants, want)
	}
	if resp.Summary.Follow != 2 || resp.Summary.Potential != 1 || resp.Summary.Unrestricted != 0 {
		t.Fatalf("summary = %#v, want follow=2 potential=1 unrestricted=0", resp.Summary)
	}
	// Without new_targets the inventory reports the current target set on both
	// sides, so the same call doubles as "who follows this alias right now".
	if !reflect.DeepEqual(resp.OldTargets, []string{"openai/gpt-4o"}) || !reflect.DeepEqual(resp.NewTargets, resp.OldTargets) {
		t.Fatalf("targets = %v -> %v, want [openai/gpt-4o] both sides", resp.OldTargets, resp.NewTargets)
	}
}

func TestAuthorizedByVirtualModelPreviewUsesProposedTargets(t *testing.T) {
	h, _ := newEffectiveModelsHandler(t, smartAlias())
	// This path is restricted to a model the alias does not serve yet.
	mustUpsertUser(t, h, `{"user_path":"/sales","allowed_models":["openai/gpt-4o-mini"]}`)

	// Nothing points at that model yet, so the path is not affected at all.
	baseline, _ := authorizedBy(t, h, "?source=smart")
	for _, grant := range baseline.Grants {
		if grant.Path == "/sales" {
			t.Fatalf("baseline grants = %#v, want no /sales grant", baseline.Grants)
		}
	}
	if baseline.Summary.Follow != 1 || baseline.Summary.Potential != 1 {
		t.Fatalf("baseline summary = %#v, want follow=1 potential=1", baseline.Summary)
	}

	// Asking about a target the alias does not serve yet reports the path as
	// potentially affected: this is the preview of the save about to happen.
	resp, code := authorizedBy(t, h, "?source=smart&new_targets=openai/gpt-4o-mini")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !reflect.DeepEqual(resp.NewTargets, []string{"openai/gpt-4o-mini"}) {
		t.Fatalf("new_targets = %v, want [openai/gpt-4o-mini]", resp.NewTargets)
	}
	var found bool
	for _, grant := range resp.Grants {
		if grant.Path == "/sales" {
			found = true
			want := authorizedByGrant{Kind: "user_path", Label: "/sales", Path: "/sales", Change: ImpactPotential, MatchedBy: "selector", Matched: []string{"openai/gpt-4o-mini"}}
			if !reflect.DeepEqual(grant, want) {
				t.Fatalf("sales grant = %#v, want %#v", grant, want)
			}
		}
	}
	if !found {
		t.Fatalf("grants = %#v, want a /sales grant matched by the proposed target", resp.Grants)
	}
	// The credential that admitted the old target stays potentially affected by
	// its removal, so both sides of the change are reported.
	if resp.Summary.Follow != 1 || resp.Summary.Potential != 2 {
		t.Fatalf("summary = %#v, want follow=1 potential=2", resp.Summary)
	}
}

func TestAuthorizedByVirtualModelAppliesScopeToGrantees(t *testing.T) {
	now := time.Now().UTC()
	entries := []string{"solo", "smart"}
	keys := []authkeys.AuthKey{
		{ID: "in", Name: "in", UserPath: "/acme", AllowedModels: entries, SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "out", Name: "out", UserPath: "/sales", AllowedModels: entries, SecretHash: "h2", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "unscoped", Name: "unscoped", AllowedModels: entries, SecretHash: "h3", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	scoped := smartAlias()
	scoped.Source = "solo"
	scoped.UserPaths = []string{"/acme"}
	h := newAuthorizedByHandler(t, keys, smartAlias(), scoped)

	for _, path := range []string{"/acme/eng", "/sales"} {
		mustUpsertUser(t, h, `{"user_path":"`+path+`","allowed_models":["solo","smart"]}`)
	}

	// A scoped alias only reaches callers inside its scope: the /sales key, the
	// pathless key and the /sales policy are all outside it.
	scopedResp, _ := authorizedBy(t, h, "?source=solo")
	wantScoped := []authorizedByGrant{
		{Kind: "credential", ID: "in", Label: "in", Path: "/acme", Change: ImpactFollow, MatchedBy: "name", Matched: []string{"solo"}},
		{Kind: "user_path", Label: "/acme/eng", Path: "/acme/eng", Change: ImpactFollow, MatchedBy: "name", Matched: []string{"solo"}},
	}
	if !reflect.DeepEqual(scopedResp.Grants, wantScoped) {
		t.Fatalf("scoped grants = %#v, want %#v", scopedResp.Grants, wantScoped)
	}

	// Reverse control: every holder also names the unscoped alias, so the
	// exclusions above come from the scope and not from a dropped grant.
	unscopedResp, _ := authorizedBy(t, h, "?source=smart")
	if len(unscopedResp.Grants) != 5 || unscopedResp.Summary.Follow != 5 {
		t.Fatalf("unscoped grants = %#v (summary %#v), want all five followers", unscopedResp.Grants, unscopedResp.Summary)
	}
}

func TestAuthorizedByVirtualModelRejectsInvalidRequests(t *testing.T) {
	h, _ := newEffectiveModelsHandler(t, smartAlias())

	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "missing source", query: "", want: http.StatusBadRequest},
		{name: "blank source", query: "?source=%20", want: http.StatusBadRequest},
		{name: "unknown alias", query: "?source=ghost", want: http.StatusNotFound},
		{name: "empty target entry", query: "?source=smart&new_targets=openai/gpt-4o,,", want: http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := jsonRequest(http.MethodGet, "/admin/virtual-models/authorized-by"+tc.query, "")
			if err := h.AuthorizedByVirtualModel(c); err != nil {
				t.Fatalf("handler returned error = %v, want a written response", err)
			}
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestAuthorizedByVirtualModelUnavailableWithoutService(t *testing.T) {
	registry := newVMModelRegistry(t)
	h := NewHandler(nil, registry)

	c, rec := jsonRequest(http.MethodGet, "/admin/virtual-models/authorized-by?source=smart", "")
	if err := h.AuthorizedByVirtualModel(c); err != nil {
		t.Fatalf("handler returned error = %v, want a written response", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
