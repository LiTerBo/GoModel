package admin

import (
	"net/http"
	"testing"

	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// A write that does not mention the pointing is a metadata-only edit: lock,
// description, user paths, enabled. It keeps the stored definition instead of
// dropping it, the rule the enabled and locked fields already followed. The e2e
// alias CRUD step (PUT {source, locked:true} → 204) is what caught the old
// replace-everything behaviour: the alias silently lost its targets.

func metadataAlias() virtualmodels.VirtualModel {
	slowdown := 1.5
	affinity := false
	return virtualmodels.VirtualModel{
		Source: "smart",
		// The fixture registry carries exactly one model (openai/gpt-4o), so the
		// stored pointing is single-target; a write that sends its own pointing
		// is told apart by the target weight it carries.
		Targets:         []virtualmodels.Target{{Model: "openai/gpt-4o", Weight: 1}},
		Strategy:        virtualmodels.StrategyRoundRobin,
		Description:     "team alias",
		UserPaths:       []string{"/team"},
		Slowdown:        &slowdown,
		SessionAffinity: &affinity,
		Enabled:         true,
	}
}

// requireMetadataAliasIntact asserts the parts a metadata-only write must not
// touch.
func requireMetadataAliasIntact(t *testing.T, h *Handler) virtualmodels.VirtualModel {
	t.Helper()
	stored, ok := h.virtualModels.Get("smart")
	if !ok || stored == nil {
		t.Fatalf("Get(smart) = missing, want the row kept")
	}
	if stored.Kind() != virtualmodels.KindRedirect || len(stored.Targets) != 1 {
		t.Fatalf("targets = %#v, want the stored pointing", stored.Targets)
	}
	if stored.Targets[0].Model != "openai/gpt-4o" {
		t.Fatalf("targets = %#v, want the stored selector kept", stored.Targets)
	}
	if stored.Strategy != virtualmodels.StrategyRoundRobin {
		t.Fatalf("strategy = %q, want round_robin kept", stored.Strategy)
	}
	if len(stored.UserPaths) != 1 || stored.UserPaths[0] != "/team" {
		t.Fatalf("user_paths = %#v, want them kept", stored.UserPaths)
	}
	if stored.Slowdown == nil || *stored.Slowdown != 1.5 {
		t.Fatalf("slowdown = %#v, want it kept", stored.Slowdown)
	}
	if stored.SessionAffinity == nil || *stored.SessionAffinity {
		t.Fatalf("session_affinity = %#v, want the stored opt-out kept", stored.SessionAffinity)
	}
	return *stored
}

func TestUpsertVirtualModelMetadataOnlyWrite(t *testing.T) {
	t.Run("locking keeps the definition", func(t *testing.T) {
		// The e2e step: a metadata-only change keeps the alias and answers 200
		// with the row it kept (204 only answers writes that leave no visible
		// row).
		h := newAuthorizedByHandler(t, nil, metadataAlias())

		rec := putVirtualModel(t, h, `{"source":"smart","locked":true}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		stored := requireMetadataAliasIntact(t, h)
		if !stored.Locked {
			t.Fatalf("locked = false, want the lock the caller sent")
		}
		if stored.Description != "team alias" {
			t.Fatalf("description = %q, want it kept", stored.Description)
		}
	})

	t.Run("a description edit keeps the definition", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, metadataAlias())

		if rec := putVirtualModel(t, h, `{"source":"smart","description":"after"}`); rec.Code >= 400 {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		stored := requireMetadataAliasIntact(t, h)
		if stored.Description != "after" {
			t.Fatalf("description = %q, want the value the caller sent", stored.Description)
		}
	})

	t.Run("an explicit empty user_paths clears while the pointing stays", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, metadataAlias())

		if rec := putVirtualModel(t, h, `{"source":"smart","user_paths":[]}`); rec.Code >= 400 {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		stored, ok := h.virtualModels.Get("smart")
		if !ok || len(stored.Targets) != 1 {
			t.Fatalf("targets = %#v, want the pointing kept", stored)
		}
		if len(stored.UserPaths) != 0 {
			t.Fatalf("user_paths = %#v, want the explicit clear to apply", stored.UserPaths)
		}
	})

	t.Run("a policy row keeps its restrictions on a metadata write", func(t *testing.T) {
		// The models page switch sends {source, enabled} at a row that may be a
		// policy restricting one model to some user paths: the switch must not
		// unlock the model for everyone on the way past.
		policy := virtualmodels.VirtualModel{
			Source:      "openai/gpt-4o",
			Description: "team only",
			UserPaths:   []string{"/team"},
			Enabled:     true,
		}
		h := newAuthorizedByHandler(t, nil, policy)

		if rec := putVirtualModel(t, h, `{"source":"openai/gpt-4o","enabled":false}`); rec.Code >= 400 {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		stored, ok := h.virtualModels.Get("openai/gpt-4o")
		if !ok || stored == nil {
			t.Fatalf("Get(openai/gpt-4o) = missing, want the policy kept")
		}
		if len(stored.UserPaths) != 1 || stored.UserPaths[0] != "/team" {
			t.Fatalf("user_paths = %#v, want the restriction kept", stored.UserPaths)
		}
		if stored.Description != "team only" {
			t.Fatalf("description = %q, want it kept", stored.Description)
		}
		if stored.Enabled {
			t.Fatalf("enabled = true, want the switch the caller sent")
		}
	})

	t.Run("sending the pointing replaces it", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, metadataAlias())

		// A weight the stored row does not carry: the caller's pointing wins.
		body := `{"source":"smart","targets":[{"model":"openai/gpt-4o","weight":3}]}`
		if rec := putVirtualModel(t, h, body); rec.Code >= 400 {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		stored, ok := h.virtualModels.Get("smart")
		if !ok || len(stored.Targets) != 1 {
			t.Fatalf("targets = %#v, want the pointing the caller sent", stored.Targets)
		}
		if stored.Targets[0].Weight != 3 {
			t.Fatalf("weight = %v, want the caller's 3 (stored row carried 1)", stored.Targets[0].Weight)
		}
	})
}
