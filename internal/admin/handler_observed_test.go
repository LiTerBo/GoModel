package admin

import (
	"net/http"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

func TestConfirmObservedCapabilities(t *testing.T) {
	stub := &stubModelTest{merged: true}
	h := newModelTestHandler(t, stub)
	c, rec := newHandlerContextWithBody("/admin/models/observed-capabilities", http.MethodPut,
		`{"provider":"local","model":"my-llm","capabilities":{"function_calling":true}}`)
	_ = h.ConfirmObservedCapabilities(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !stub.mergeCaps["function_calling"] {
		t.Fatalf("merge caps = %v", stub.mergeCaps)
	}
	if stub.mergeSource != core.CapSrcObserved {
		t.Fatalf("source = %q, want observed", stub.mergeSource)
	}
}

func TestConfirmObservedCapabilitiesUnknownModel(t *testing.T) {
	stub := &stubModelTest{merged: false}
	h := newModelTestHandler(t, stub)
	c, rec := newHandlerContextWithBody("/admin/models/observed-capabilities", http.MethodPut,
		`{"provider":"local","model":"ghost","capabilities":{"function_calling":true}}`)
	_ = h.ConfirmObservedCapabilities(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestConfirmObservedCapabilitiesUnavailable(t *testing.T) {
	h := NewHandler(nil, nil)
	c, rec := newHandlerContextWithBody("/admin/models/observed-capabilities", http.MethodPut,
		`{"provider":"local","model":"m","capabilities":{"vision":true}}`)
	_ = h.ConfirmObservedCapabilities(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
