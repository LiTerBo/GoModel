package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/enterpilot/gomodel/internal/capability"
	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/modeldata/modeltest"
)

// stubConfirmer scripts the capability.Confirmer the persistence path uses.
type stubConfirmer struct {
	provider string
	model    string
	caps     map[string]bool
	source   string
	err      error
	calls    int
}

func (s *stubConfirmer) Confirm(ctx context.Context, provider, model string, caps map[string]bool, source string) error {
	s.calls++
	s.provider, s.model, s.caps, s.source = provider, model, caps, source
	return s.err
}

func newCapabilityTestHandler(t *testing.T, stub *stubModelTest, confirmer *stubConfirmer) *Handler {
	t.Helper()
	return NewHandler(nil, nil, WithModelTest(stub), WithCapabilityConfirmer(confirmer))
}

func putCapabilities(t *testing.T, h *Handler, path, body string) int {
	t.Helper()
	c, rec := newHandlerContextWithBody(path, http.MethodPut, body)
	_ = h.ConfirmModelCapabilities(c) // writes the response itself via handleError
	return rec.Code
}

// TestConfirmModelCapabilitiesPersistsBeforeMerge pins the two-write order:
// the durable store is updated first, and only then does the in-memory
// registry change. A store failure must therefore leave the registry
// untouched (no in-memory state that the next restart will not have).
func TestConfirmModelCapabilitiesPersistsBeforeMerge(t *testing.T) {
	stub := &stubModelTest{merged: true}
	confirmer := &stubConfirmer{}
	h := newCapabilityTestHandler(t, stub, confirmer)

	code := putCapabilities(t, h, "/admin/models/capabilities",
		`{"provider":"local","model":"my-llm","capabilities":{"function_calling":true}}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if confirmer.calls != 1 {
		t.Fatalf("Confirm calls = %d, want 1", confirmer.calls)
	}
	if confirmer.provider != "local" || confirmer.model != "my-llm" {
		t.Errorf("Confirm target = %s/%s, want local/my-llm", confirmer.provider, confirmer.model)
	}
	if !confirmer.caps["function_calling"] || confirmer.source != core.CapSrcTest {
		t.Errorf("Confirm args = %v/%q, want function_calling=true with test source", confirmer.caps, confirmer.source)
	}
	if !stub.mergeCaps["function_calling"] || stub.mergeSource != core.CapSrcTest {
		t.Errorf("registry merge args = %v/%q, want function_calling=true with test source", stub.mergeCaps, stub.mergeSource)
	}
}

// A store failure surfaces as a 502-style provider error and must stop before
// the registry merge — the operator sees the failure and retries; nothing
// changes in memory behind their back.
func TestConfirmModelCapabilitiesStoreErrorStopsBeforeMerge(t *testing.T) {
	stub := &stubModelTest{merged: true}
	confirmer := &stubConfirmer{err: errors.New("disk on fire")}
	h := newCapabilityTestHandler(t, stub, confirmer)

	code := putCapabilities(t, h, "/admin/models/capabilities",
		`{"provider":"local","model":"my-llm","capabilities":{"function_calling":true}}`)
	if code == http.StatusOK {
		t.Fatal("store error accepted as success")
	}
	if stub.mergeCaps != nil {
		t.Errorf("registry merged despite store failure: %v", stub.mergeCaps)
	}
}

// Without the persistence channel wired the endpoint keeps its historical
// behaviour: confirmation still works through the registry channel alone
// (deployments that predate the capability store, and unit tests).
func TestConfirmModelCapabilitiesWithoutConfirmerStillMerges(t *testing.T) {
	stub := &stubModelTest{merged: true}
	h := NewHandler(nil, nil, WithModelTest(stub))

	code := putCapabilities(t, h, "/admin/models/capabilities",
		`{"provider":"local","model":"my-llm","capabilities":{"function_calling":true}}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !stub.mergeCaps["function_calling"] {
		t.Errorf("registry merge args = %v, want function_calling merged", stub.mergeCaps)
	}
}

// The observed-capabilities endpoint shares the confirmation channel but must
// stamp its own source, so provenance survives persistence.
func TestConfirmObservedCapabilitiesPersistsWithObservedSource(t *testing.T) {
	stub := &stubModelTest{merged: true}
	confirmer := &stubConfirmer{}
	h := newCapabilityTestHandler(t, stub, confirmer)

	c, rec := newHandlerContextWithBody("/admin/models/observed-capabilities", http.MethodPut,
		`{"provider":"local","model":"my-llm","capabilities":{"vision":true}}`)
	_ = h.ConfirmObservedCapabilities(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if confirmer.source != core.CapSrcObserved {
		t.Errorf("Confirm source = %q, want %q", confirmer.source, core.CapSrcObserved)
	}
	if stub.mergeSource != core.CapSrcObserved {
		t.Errorf("registry merge source = %q, want %q", stub.mergeSource, core.CapSrcObserved)
	}
}

// The service reports unknown models with a plain error; the handler maps it
// to the same 404 the registry-only path produced before persistence existed.
func TestConfirmModelCapabilitiesUnknownModelMapsTo404(t *testing.T) {
	stub := &stubModelTest{merged: true}
	// Service-like behaviour: the merge inside Confirm fails for unknown
	// models while the store write still happened. The handler must return
	// the error (404) rather than a false success.
	confirmer := &stubConfirmer{err: capability.ErrUnknownModel}
	h := newCapabilityTestHandler(t, stub, confirmer)

	code := putCapabilities(t, h, "/admin/models/capabilities",
		`{"provider":"local","model":"ghost","capabilities":{"function_calling":true}}`)
	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
}

// Guard the existing validation tests against drift: with the confirmer
// wired, malformed payloads must still fail before any Confirm call.
func TestConfirmModelCapabilitiesValidationDoesNotCallConfirmer(t *testing.T) {
	stub := &stubModelTest{merged: true}
	confirmer := &stubConfirmer{}
	h := newCapabilityTestHandler(t, stub, confirmer)

	for name, body := range map[string]string{
		"empty caps":  `{"provider":"local","model":"m","capabilities":{}}`,
		"blank key":   `{"provider":"local","model":"m","capabilities":{" ":true}}`,
		"missing all": `{"capabilities":{"vision":true}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if code := putCapabilities(t, h, "/admin/models/capabilities", body); code == http.StatusOK {
				t.Fatalf("%s accepted", name)
			}
			if confirmer.calls != 0 {
				t.Fatalf("Confirm called %d times for invalid input", confirmer.calls)
			}
		})
	}
}

// Compile-time contract: the admin option accepts the real service.
var _ capability.Confirmer = (*stubConfirmer)(nil)

// probeResultsJSON documents the wire shape the UI consumes (kept textual so
// this file has no dependency on echo response marshalling specifics).
func TestProbeResultsWireShapeUnchanged(t *testing.T) {
	results := []modeltest.Result{{
		Provider: "local", Model: "m", Probe: modeltest.ProbeChat,
		Verdict: modeltest.Pass, LatencyMs: 12,
	}}
	data, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"probe":"chat"`, `"verdict":"pass"`, `"latency_ms":12`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("probe result JSON missing %s: %s", want, data)
		}
	}
}
