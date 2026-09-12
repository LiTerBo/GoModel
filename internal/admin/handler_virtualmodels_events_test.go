package admin

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/authkeys"
	"github.com/enterpilot/gomodel/internal/storage/sqlx/sqlxtest"
	"github.com/enterpilot/gomodel/internal/users"
	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// T13-E: every management action that changes what a virtual model resolves to
// (or that removes it) reaches the audit trail, so an operator can reconstruct
// later who was affected without asking the policies that were in place then.

// newEventFixture builds a handler whose virtual-model catalog serves both
// gpt-4o and gpt-4o-mini, so a retarget between them is a valid write.
func newEventFixture(t *testing.T, keys []authkeys.AuthKey, vms ...virtualmodels.VirtualModel) *Handler {
	t.Helper()
	ctx := context.Background()
	registry := newVMModelRegistry(t) // provider openai for allowlist validation
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
	catalog := newVMTestCatalog()
	catalog.add("openai/gpt-4o", "openai")
	catalog.add("openai/gpt-4o-mini", "openai")
	vmService := newVMService(t, catalog, newVMTestStore(vms...), true)
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

func captureVirtualModelEvents(t *testing.T, h *Handler) *[]auditlog.VirtualModelChange {
	t.Helper()
	events := &[]auditlog.VirtualModelChange{}
	WithVirtualModelEvents(func(change auditlog.VirtualModelChange) {
		*events = append(*events, change)
	})(h)
	return events
}

func eventFixtureKeys() []authkeys.AuthKey {
	now := time.Now().UTC()
	return []authkeys.AuthKey{
		{ID: "by-name", Name: "by-name", UserPath: "/acme", AllowedModels: []string{"smart"}, SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "by-target", Name: "by-target", UserPath: "/acme", AllowedModels: []string{"openai/"}, SecretHash: "h2", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
}

func TestWithVirtualModelEventsWiresTheSink(t *testing.T) {
	h := newVMHandler(t)
	if h.virtualModelEvents != nil {
		t.Fatalf("virtualModelEvents is installed by default, want nil")
	}
	called := false
	WithVirtualModelEvents(func(auditlog.VirtualModelChange) { called = true })(h)
	if h.virtualModelEvents == nil {
		t.Fatalf("virtualModelEvents = nil, want the installed sink")
	}
	h.virtualModelEvents(auditlog.VirtualModelChange{})
	if !called {
		t.Fatalf("sink was not called")
	}
}

func TestVirtualModelEventsOnRetarget(t *testing.T) {
	locked := smartAlias()
	locked.Locked = true
	h := newEventFixture(t, eventFixtureKeys(), locked)
	mustUpsertUser(t, h, `{"user_path":"/team","allowed_models":["smart"]}`)
	events := captureVirtualModelEvents(t, h)

	code := upsertVirtualModel(t, h, `{"source":"smart","targets":[{"provider":"openai","model":"gpt-4o-mini"}],"unlock":true}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %#v, want one retarget event", *events)
	}
	got := (*events)[0]
	if got.Action != auditlog.VirtualModelActionRetarget || got.Source != "smart" {
		t.Fatalf("event = %#v, want a retarget of smart", got)
	}
	if !reflect.DeepEqual(got.OldTargets, []string{"openai/gpt-4o"}) || !reflect.DeepEqual(got.NewTargets, []string{"openai/gpt-4o-mini"}) {
		t.Fatalf("targets = %v -> %v, want the before/after pair", got.OldTargets, got.NewTargets)
	}
	if !got.Locked {
		t.Fatalf("Locked = false, want the row to still be locked")
	}
	// Reachability is recorded with the change: the name-authorized key, the
	// openai/ wildcard key, and the /team policy.
	if got.Credentials != 2 || got.UserPaths != 1 {
		t.Fatalf("counts = %d credentials / %d user paths, want 2/1", got.Credentials, got.UserPaths)
	}
	if got.Method != http.MethodPut || got.Path != "/admin/virtual-models" || got.Timestamp.IsZero() {
		t.Fatalf("event metadata = %#v, want method/path/timestamp", got)
	}
}

func TestVirtualModelEventsOnLockToggles(t *testing.T) {
	h := newEventFixture(t, eventFixtureKeys(), smartAlias())
	events := captureVirtualModelEvents(t, h)

	if code := upsertVirtualModel(t, h, `{"source":"smart","targets":[{"provider":"openai","model":"gpt-4o"}],"locked":true}`); code != http.StatusOK {
		t.Fatalf("lock status = %d, want 200", code)
	}
	if code := upsertVirtualModel(t, h, `{"source":"smart","targets":[{"provider":"openai","model":"gpt-4o"}],"locked":false}`); code != http.StatusOK {
		t.Fatalf("unlock status = %d, want 200", code)
	}
	// An edit that only touches the description is not a management action.
	if code := upsertVirtualModel(t, h, `{"source":"smart","targets":[{"provider":"openai","model":"gpt-4o"}],"description":"note"}`); code != http.StatusOK {
		t.Fatalf("note status = %d, want 200", code)
	}

	want := []string{auditlog.VirtualModelActionLock, auditlog.VirtualModelActionUnlock}
	got := make([]string, 0, len(*events))
	for _, event := range *events {
		got = append(got, event.Action)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
	for _, event := range *events {
		// A lock toggle leaves the pointing alone, so both target lists are the
		// stored one; the reachability of the row still rides along, which is
		// what makes a later "who depended on this?" answer possible.
		if event.Source != "smart" || event.Locked != (event.Action == auditlog.VirtualModelActionLock) {
			t.Fatalf("event = %#v, want a smart toggle with the matching lock state", event)
		}
		if !reflect.DeepEqual(event.OldTargets, []string{"openai/gpt-4o"}) || !reflect.DeepEqual(event.NewTargets, []string{"openai/gpt-4o"}) {
			t.Fatalf("targets = %v -> %v, want the unchanged pair", event.OldTargets, event.NewTargets)
		}
		if event.Credentials != 2 {
			t.Fatalf("Credentials = %d, want the reachability of the row (2)", event.Credentials)
		}
	}
}

func TestVirtualModelEventsOnForcedDelete(t *testing.T) {
	h := newEventFixture(t, eventFixtureKeys(), smartAlias())
	mustUpsertUser(t, h, `{"user_path":"/team","allowed_models":["smart"]}`)
	events := captureVirtualModelEvents(t, h)

	if code, _ := deleteVirtualModel(t, h, `{"source":"smart"}`); code != http.StatusConflict {
		t.Fatalf("blocked delete status = %d, want 409", code)
	}
	if len(*events) != 0 {
		t.Fatalf("events = %#v, want none for a blocked delete", *events)
	}

	if code, _ := deleteVirtualModel(t, h, `{"source":"smart","force":true}`); code != http.StatusNoContent {
		t.Fatalf("forced delete status = %d, want 204", code)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %#v, want one delete event", *events)
	}
	got := (*events)[0]
	if got.Action != auditlog.VirtualModelActionDelete || !got.Forced || got.Source != "smart" {
		t.Fatalf("event = %#v, want a forced delete of smart", got)
	}
	if got.Credentials != 2 || got.UserPaths != 1 {
		t.Fatalf("counts = %d/%d, want the reachability recorded with the delete", got.Credentials, got.UserPaths)
	}
}

func TestVirtualModelEventsOnRename(t *testing.T) {
	// A rename moves the name callers address, which is a management action even
	// when the target list is unchanged: the old name stops resolving for the
	// credentials that name it.
	h := newEventFixture(t, eventFixtureKeys(), smartAlias())
	events := captureVirtualModelEvents(t, h)

	if code := upsertVirtualModel(t, h, `{"source":"smart2","old_source":"smart","targets":[{"provider":"openai","model":"gpt-4o"}]}`); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(*events) != 1 {
		t.Fatalf("events = %#v, want one rename event", *events)
	}
	got := (*events)[0]
	if got.Action != auditlog.VirtualModelActionRename || got.Source != "smart2" || got.PreviousSource != "smart" {
		t.Fatalf("event = %#v, want a rename smart -> smart2", got)
	}
	if got.Credentials != 2 {
		t.Fatalf("Credentials = %d, want the reachability of the renamed row (2)", got.Credentials)
	}
}

func TestVirtualModelEventsSkipBlockedWrites(t *testing.T) {
	locked := smartAlias()
	locked.Locked = true
	h := newEventFixture(t, eventFixtureKeys(), locked)
	events := captureVirtualModelEvents(t, h)

	if code := upsertVirtualModel(t, h, `{"source":"smart","targets":[{"provider":"openai","model":"gpt-4o-mini"}]}`); code != http.StatusConflict {
		t.Fatalf("locked write status = %d, want 409", code)
	}
	if len(*events) != 0 {
		t.Fatalf("events = %#v, want none for a rejected write", *events)
	}
}
