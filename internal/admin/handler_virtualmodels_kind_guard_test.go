package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// A stored redirect owns its source. A write that would take that source over
// as an access policy drops the alias definition, so it has to say so
// explicitly: the models page row switch sends {source, enabled} and used to
// silently replace the alias with a policy (the masking-alias risk).

// putVirtualModelBody wraps the package's putVirtualModel recorder with the
// parsed error envelope, so a case can assert code and param.
func putVirtualModelBody(t *testing.T, h *Handler, body string) (int, errorBody) {
	t.Helper()
	rec := putVirtualModel(t, h, body)
	var parsed errorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	return rec.Code, parsed
}

func TestUpsertVirtualModelRedirectTakeoverNeedsGesture(t *testing.T) {
	t.Run("the row switch payload is rejected and the alias survives", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, smartAlias())

		code, body := putVirtualModelBody(t, h, `{"source":"smart","enabled":false}`)
		if code != http.StatusConflict {
			t.Fatalf("status = %d, want %d (body=%s)", code, http.StatusConflict, body.Error.Message)
		}
		if body.Error.Code != "virtual_model_kind_change" {
			t.Fatalf("code = %q, want virtual_model_kind_change", body.Error.Code)
		}
		if body.Error.Param != "clear_targets" {
			t.Fatalf("param = %q, want clear_targets", body.Error.Param)
		}
		if !containsAll(body.Error.Message, "smart", "clear_targets") {
			t.Fatalf("message = %q, want it to name the source and the gesture", body.Error.Message)
		}
		stored, ok := h.virtualModels.Get("smart")
		if !ok {
			t.Fatalf("Get(smart) after a blocked write = missing, want the redirect kept")
		}
		if stored.Kind() != virtualmodels.KindRedirect || len(stored.Targets) != 1 {
			t.Fatalf("stored = %#v, want the redirect untouched", stored)
		}
	})

	t.Run("the explicit gesture takes the source over", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, smartAlias())

		if code, body := putVirtualModelBody(t, h, `{"source":"smart","enabled":false,"clear_targets":true}`); code >= 400 {
			t.Fatalf("status = %d, error = %#v", code, body.Error)
		}
		// A redundant policy is pruned instead of stored, so the row is either
		// gone or a policy — never still a redirect.
		if stored, ok := h.virtualModels.Get("smart"); ok && stored.Kind() == virtualmodels.KindRedirect {
			t.Fatalf("stored = %#v, want the redirect gone", stored)
		}
	})

	t.Run("a redirect writing the same source still works without the gesture", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, smartAlias())

		if code, body := putVirtualModelBody(t, h, `{"source":"smart","target_model":"openai/gpt-4o","enabled":true}`); code >= 400 {
			t.Fatalf("status = %d, error = %#v", code, body.Error)
		}
		stored, ok := h.virtualModels.Get("smart")
		if !ok || stored.Kind() != virtualmodels.KindRedirect {
			t.Fatalf("stored = %#v, want a redirect", stored)
		}
	})

	t.Run("a policy for a source with no stored row is unaffected", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, smartAlias())

		if code, body := putVirtualModelBody(t, h, `{"source":"openai/gpt-4o","enabled":false}`); code >= 400 {
			t.Fatalf("status = %d, error = %#v", code, body.Error)
		}
	})

	t.Run("a policy row can still be promoted to a redirect", func(t *testing.T) {
		h := newAuthorizedByHandler(t, nil, smartAlias())
		if code, body := putVirtualModelBody(t, h, `{"source":"plain","enabled":false}`); code >= 400 {
			t.Fatalf("seed policy status = %d, error = %#v", code, body.Error)
		}

		if code, body := putVirtualModelBody(t, h, `{"source":"plain","target_model":"openai/gpt-4o","enabled":true}`); code >= 400 {
			t.Fatalf("status = %d, error = %#v", code, body.Error)
		}
		stored, ok := h.virtualModels.Get("plain")
		if !ok || stored.Kind() != virtualmodels.KindRedirect {
			t.Fatalf("stored = %#v, want a redirect", stored)
		}
	})
}
