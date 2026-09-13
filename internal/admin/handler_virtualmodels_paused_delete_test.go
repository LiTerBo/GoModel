package admin

import (
	"net/http"
	"testing"
	"time"

	"github.com/enterpilot/gomodel/internal/authkeys"
)

// A paused redirect (enabled=false) resolves no name: authorization refuses a
// request aimed at it before routing, so deleting it takes reachability away
// from nobody. Gating that delete on the reachability tally would only block a
// cleanup, so the force gate must not apply. The sibling subtest holds the
// contrast: the same shape with enabled=true stays gated.
func TestDeleteVirtualModelPausedRedirectNeedsNoForce(t *testing.T) {
	now := time.Now().UTC()
	keys := []authkeys.AuthKey{
		{ID: "by-name", Name: "by-name", UserPath: "/acme", AllowedModels: []string{"paused"}, SecretHash: "h1", Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "live", Name: "live", UserPath: "/acme", AllowedModels: []string{"live"}, SecretHash: "h2", Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
	paused := smartAlias()
	paused.Source = "paused"
	paused.Enabled = false
	live := smartAlias()
	live.Source = "live"
	h := newAuthorizedByHandler(t, keys, paused, live)

	t.Run("paused redirect deletes without force", func(t *testing.T) {
		code, body := deleteVirtualModel(t, h, `{"source":"paused"}`)
		if code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d body=%s", code, http.StatusNoContent, body.Error.Code)
		}
		if _, ok := h.virtualModels.Get("paused"); ok {
			t.Fatalf("Get(paused) after delete = present, want the row gone")
		}
	})

	t.Run("enabled redirect stays gated", func(t *testing.T) {
		code, body := deleteVirtualModel(t, h, `{"source":"live"}`)
		if code != http.StatusConflict || body.Error.Code != "virtual_model_in_use" {
			t.Fatalf("status = %d code = %q, want 409 virtual_model_in_use", code, body.Error.Code)
		}
		if _, ok := h.virtualModels.Get("live"); !ok {
			t.Fatalf("Get(live) after blocked delete = missing, want the row kept")
		}
	})
}
