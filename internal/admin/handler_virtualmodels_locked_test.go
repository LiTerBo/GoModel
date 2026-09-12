package admin

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// T13-C: locking a virtual model guards its pointing. A locked row accepts
// unrelated edits, but a change that alters which models a caller can end up
// talking to needs an explicit unlock gesture in the same request.
func lockedRedirect(source, target string) virtualmodels.VirtualModel {
	vm := redirectVM(source, target, true)
	vm.Locked = true
	return vm
}

func upsertVirtualModel(t *testing.T, h *Handler, body string) int {
	t.Helper()
	c, rec := jsonRequest(http.MethodPut, "/admin/virtual-models", body)
	if err := h.UpsertVirtualModel(c); err != nil {
		t.Fatalf("UpsertVirtualModel error = %v", err)
	}
	return rec.Code
}

func TestUpsertVirtualModelLockGuard(t *testing.T) {
	const changedTargets = `"targets":[{"provider":"openai","model":"gpt-4o-mini"}]`
	sameTargets := `"targets":[{"provider":"openai","model":"gpt-4o"}]`

	policyRow := virtualmodels.VirtualModel{
		Source:       "openai/gpt-4o",
		ProviderName: "openai",
		Model:        "gpt-4o",
		Locked:       true,
		Enabled:      true,
	}

	tests := []struct {
		name        string
		stored      virtualmodels.VirtualModel
		body        string
		want        int
		wantTargets []string
		wantLocked  bool
	}{
		{
			name:        "locked row rejects a target change",
			stored:      lockedRedirect("smart", "openai/gpt-4o"),
			body:        `{"source":"smart",` + changedTargets + `}`,
			want:        http.StatusConflict,
			wantTargets: []string{"openai/gpt-4o"},
			wantLocked:  true,
		},
		{
			name:        "unlock confirms the change and keeps the lock",
			stored:      lockedRedirect("smart", "openai/gpt-4o"),
			body:        `{"source":"smart",` + changedTargets + `,"unlock":true}`,
			want:        http.StatusOK,
			wantTargets: []string{"openai/gpt-4o-mini"},
			wantLocked:  true,
		},
		{
			name:        "explicit locked false unlocks and applies the change",
			stored:      lockedRedirect("smart", "openai/gpt-4o"),
			body:        `{"source":"smart",` + changedTargets + `,"locked":false}`,
			want:        http.StatusOK,
			wantTargets: []string{"openai/gpt-4o-mini"},
			wantLocked:  false,
		},
		{
			name:        "unrelated edit passes on a locked row",
			stored:      lockedRedirect("smart", "openai/gpt-4o"),
			body:        `{"source":"smart",` + sameTargets + `,"description":"note"}`,
			want:        http.StatusOK,
			wantTargets: []string{"openai/gpt-4o"},
			wantLocked:  true,
		},
		{
			name:        "lock can be turned on without touching the pointing",
			stored:      redirectVM("smart", "openai/gpt-4o", true),
			body:        `{"source":"smart",` + sameTargets + `,"locked":true}`,
			want:        http.StatusOK,
			wantTargets: []string{"openai/gpt-4o"},
			wantLocked:  true,
		},
		{
			name:        "strategy change is guarded too",
			stored:      lockedRedirect("smart", "openai/gpt-4o"),
			body:        `{"source":"smart",` + sameTargets + `,"strategy":"cost"}`,
			want:        http.StatusConflict,
			wantTargets: []string{"openai/gpt-4o"},
			wantLocked:  true,
		},
		{
			name:        "unlocked row keeps behaving as before",
			stored:      redirectVM("smart", "openai/gpt-4o", true),
			body:        `{"source":"smart",` + changedTargets + `}`,
			want:        http.StatusOK,
			wantTargets: []string{"openai/gpt-4o-mini"},
			wantLocked:  false,
		},
		{
			name:       "locked policy row cannot gain a target",
			stored:     policyRow,
			body:       `{"source":"openai/gpt-4o",` + changedTargets + `}`,
			want:       http.StatusConflict,
			wantLocked: true,
		},
		{
			name:        "rename off a locked source is guarded",
			stored:      lockedRedirect("smart", "openai/gpt-4o"),
			body:        `{"source":"smart2","old_source":"smart",` + changedTargets + `}`,
			want:        http.StatusConflict,
			wantTargets: []string{"openai/gpt-4o"},
			wantLocked:  true,
		},
		{
			name:        "rename off a locked source with an unlock passes",
			stored:      lockedRedirect("smart", "openai/gpt-4o"),
			body:        `{"source":"smart2","old_source":"smart",` + changedTargets + `,"unlock":true}`,
			want:        http.StatusOK,
			wantTargets: []string{"openai/gpt-4o-mini"},
			wantLocked:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newVMHandler(t, tc.stored)

			if got := upsertVirtualModel(t, h, tc.body); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
			// The row that must keep its old shape after a blocked write is the
			// stored source; a successful rename moved it to the new one.
			source := tc.stored.Source
			var body struct {
				Source    string `json:"source"`
				OldSource string `json:"old_source"`
			}
			_ = json.Unmarshal([]byte(tc.body), &body)
			if tc.want == http.StatusOK && body.OldSource != "" {
				source = body.Source
			}
			stored, ok := h.virtualModels.Get(source)
			if !ok || stored == nil {
				t.Fatalf("Get(%s) = %#v, want the row", source, stored)
			}
			if stored.Locked != tc.wantLocked {
				t.Fatalf("Locked = %v, want %v", stored.Locked, tc.wantLocked)
			}
			if tc.wantTargets != nil {
				got := make([]string, 0, len(stored.Targets))
				for _, target := range stored.Targets {
					got = append(got, target.Provider+"/"+target.Model)
				}
				if !reflect.DeepEqual(got, tc.wantTargets) {
					t.Fatalf("targets = %v, want %v", got, tc.wantTargets)
				}
			}
		})
	}
}

func TestUpsertVirtualModelLockedErrorBody(t *testing.T) {
	h := newVMHandler(t, lockedRedirect("smart", "openai/gpt-4o"))
	c, rec := jsonRequest(http.MethodPut, "/admin/virtual-models",
		`{"source":"smart","targets":[{"provider":"openai","model":"gpt-4o-mini"}]}`)
	if err := h.UpsertVirtualModel(c); err != nil {
		t.Fatalf("UpsertVirtualModel error = %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, rec.Body.String())
	}
	if body.Error.Code != "virtual_model_locked" {
		t.Fatalf("code = %q, want virtual_model_locked (body=%s)", body.Error.Code, rec.Body.String())
	}
	if !strings.Contains(body.Error.Message, "smart") {
		t.Fatalf("message = %q, want it to name the locked source", body.Error.Message)
	}
}
