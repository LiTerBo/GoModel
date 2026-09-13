package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/enterpilot/gomodel/internal/authkeys"
	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// T13-D: deleting a virtual model takes it away from everyone who reaches it by
// name or through its targets. That is a bigger change than an edit, so the
// delete needs an explicit force once anybody still reaches the alias.

func deleteVirtualModel(t *testing.T, h *Handler, body string) (int, errorBody) {
	t.Helper()
	c, rec := jsonRequest(http.MethodDelete, "/admin/virtual-models", body)
	if err := h.DeleteVirtualModel(c); err != nil {
		t.Fatalf("DeleteVirtualModel error = %v", err)
	}
	var body404 errorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body404)
	return rec.Code, body404
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Param   string `json:"param"`
	} `json:"error"`
}

func TestDeleteVirtualModelGuardsReachability(t *testing.T) {
	now := time.Now().UTC()
	keys := []authkeys.AuthKey{
		{ID: "by-name", Name: "by-name", UserPath: "/acme", AllowedModels: []string{"smart"}, SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "by-target", Name: "by-target", UserPath: "/acme", AllowedModels: []string{"openai/"}, SecretHash: "h2", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "other", Name: "other", UserPath: "/acme", AllowedModels: []string{"gpt-4o-mini"}, SecretHash: "h3", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	scoped := smartAlias()
	scoped.Source = "solo"
	scoped.UserPaths = []string{"/acme"}
	h := newAuthorizedByHandler(t, keys, smartAlias(), scoped)
	mustUpsertUser(t, h, `{"user_path":"/team","allowed_models":["smart"]}`)

	t.Run("alias reached by name and by target is blocked", func(t *testing.T) {
		code, body := deleteVirtualModel(t, h, `{"source":"smart"}`)
		if code != http.StatusConflict {
			t.Fatalf("status = %d, want %d body=%s", code, http.StatusConflict, body.Error.Code)
		}
		if body.Error.Code != "virtual_model_in_use" {
			t.Fatalf("code = %q, want virtual_model_in_use", body.Error.Code)
		}
		if body.Error.Param != "force" {
			t.Fatalf("param = %q, want force", body.Error.Param)
		}
		if !containsAll(body.Error.Message, "smart", "force") {
			t.Fatalf("message = %q, want it to name the source and the override", body.Error.Message)
		}
		// The row is still there after a blocked delete.
		if _, ok := h.virtualModels.Get("smart"); !ok {
			t.Fatalf("Get(smart) after blocked delete = missing, want the row kept")
		}
	})

	t.Run("force deletes it", func(t *testing.T) {
		code, _ := deleteVirtualModel(t, h, `{"source":"smart","force":true}`)
		if code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", code, http.StatusNoContent)
		}
		if _, ok := h.virtualModels.Get("smart"); ok {
			t.Fatalf("Get(smart) after forced delete = present, want it gone")
		}
	})

	t.Run("force must be true to override", func(t *testing.T) {
		code, body := deleteVirtualModel(t, h, `{"source":"solo","force":false,"unlock":true}`)
		if code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (%s)", code, http.StatusConflict, body.Error.Code)
		}
	})

	t.Run("target-level reachability blocks the delete too", func(t *testing.T) {
		// Nobody names "solo"; the openai/ wildcard key reaches it through its
		// target, so the delete still needs force. The counts prove the block
		// came from the target-level rule and not from a name match.
		code, body := deleteVirtualModel(t, h, `{"source":"solo"}`)
		if code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", code, http.StatusConflict)
		}
		if !strings.Contains(body.Error.Message, "1 credential(s)") || !strings.Contains(body.Error.Message, "0 user path(s)") {
			t.Fatalf("message = %q, want one credential and no user path", body.Error.Message)
		}
	})

	t.Run("unknown and blank sources keep their old answers", func(t *testing.T) {
		if code, _ := deleteVirtualModel(t, h, `{"source":"ghost"}`); code != http.StatusNotFound {
			t.Fatalf("unknown source status = %d, want %d", code, http.StatusNotFound)
		}
		if code, _ := deleteVirtualModel(t, h, `{"source":"  "}`); code != http.StatusBadRequest {
			t.Fatalf("blank source status = %d, want %d", code, http.StatusBadRequest)
		}
	})
}

func TestDeleteVirtualModelScopeKeepsOutOfScopeCallersOut(t *testing.T) {
	now := time.Now().UTC()
	// The only holder that names the alias sits outside its user-path scope, so
	// a delete takes nothing away from it and must not need force.
	keys := []authkeys.AuthKey{
		{ID: "elsewhere", Name: "elsewhere", UserPath: "/sales", AllowedModels: []string{"solo"}, SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	scoped := smartAlias()
	scoped.Source = "solo"
	scoped.UserPaths = []string{"/acme"}
	h := newAuthorizedByHandler(t, keys, scoped)

	if code, body := deleteVirtualModel(t, h, `{"source":"solo"}`); code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (%s)", code, http.StatusNoContent, body.Error.Message)
	}
	if _, ok := h.virtualModels.Get("solo"); ok {
		t.Fatalf("Get(solo) = present, want it deleted")
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

// The guard protects addressable rows only. An access policy carries no
// targets, so it resolves no name for anybody: deleting it drops the row's
// restriction and restores the catalog default, which takes nothing away from
// the holders the tally lists — and that tally lists them for ANY source when
// their allowlist is empty. Gating the delete on it made un-pausing a single
// model impossible (report 2026-09-13: oMLX/bge-m3-mlx-8bit answered 409 with
// "still reachable by 2 credential(s) and 1 user path(s)").
func TestDeleteVirtualModelPolicyRowNeedsNoForce(t *testing.T) {
	now := time.Now().UTC()
	keys := []authkeys.AuthKey{
		// Both holders are unrestricted (empty allowlist): they reach every
		// model, which is why they must not stand in for "references this row".
		{ID: "team", Name: "team", UserPath: "/acme", SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	policy := virtualmodels.VirtualModel{Source: "oMLX/bge-m3-mlx-8bit", Enabled: false}
	h := newAuthorizedByHandler(t, keys, policy)
	mustUpsertUser(t, h, `{"user_path":"/acme","allowed_models":[]}`)

	code, body := deleteVirtualModel(t, h, `{"source":"oMLX/bge-m3-mlx-8bit"}`)
	if code != http.StatusNoContent {
		t.Fatalf("policy delete status = %d, want %d (code=%s message=%s)", code, http.StatusNoContent, body.Error.Code, body.Error.Message)
	}
	if _, ok := h.virtualModels.Get("oMLX/bge-m3-mlx-8bit"); ok {
		t.Fatalf("Get(policy) after delete = present, want it deleted")
	}
}

// The same holders must still gate an alias delete: a redirect IS the name
// callers address, so an unrestricted credential — which can call the alias by
// name — is a holder the operator has to confirm against.
func TestDeleteVirtualModelAliasStaysGatedByUnrestrictedHolders(t *testing.T) {
	now := time.Now().UTC()
	keys := []authkeys.AuthKey{
		{ID: "team", Name: "team", UserPath: "/acme", SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	alias := virtualmodels.VirtualModel{
		Source:  "smart",
		Targets: []virtualmodels.Target{{Provider: "openai", Model: "gpt-4o"}},
		Enabled: true,
	}
	h := newAuthorizedByHandler(t, keys, alias)
	mustUpsertUser(t, h, `{"user_path":"/acme","allowed_models":[]}`)

	code, body := deleteVirtualModel(t, h, `{"source":"smart"}`)
	if code != http.StatusConflict {
		t.Fatalf("alias delete status = %d, want %d", code, http.StatusConflict)
	}
	if body.Error.Code != "virtual_model_in_use" {
		t.Fatalf("code = %q, want virtual_model_in_use", body.Error.Code)
	}
	if !containsAll(body.Error.Message, "1 credential(s)", "1 user path(s)") {
		t.Fatalf("message = %q, want the unrestricted holder tally", body.Error.Message)
	}
	// Force still overrides it.
	if code, _ := deleteVirtualModel(t, h, `{"source":"smart","force":true}`); code != http.StatusNoContent {
		t.Fatalf("forced alias delete status = %d, want %d", code, http.StatusNoContent)
	}
}
