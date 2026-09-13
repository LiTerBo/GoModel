package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/modeldata/modeltest"
)

// stubModelTest is a fully scripted ModelTestAdmin.
type stubModelTest struct {
	probeErr    bool
	merged      bool
	mergeCaps   map[string]bool
	mergeSource string
}

func (s *stubModelTest) Probe(provider, model string, probes []modeltest.Probe) []modeltest.Result {
	if s.probeErr {
		return nil
	}
	out := make([]modeltest.Result, 0, len(probes))
	for _, p := range probes {
		out = append(out, modeltest.Result{Provider: provider, Model: model, Probe: p, Verdict: modeltest.Pass})
	}
	return out
}

func (s *stubModelTest) MergeModelCapabilities(providerName, modelID string, caps map[string]bool, source string) bool {
	if !s.merged {
		return false
	}
	s.mergeCaps = caps
	s.mergeSource = source
	return true
}

func newModelTestHandler(t *testing.T, stub *stubModelTest) *Handler {
	t.Helper()
	return NewHandler(nil, nil, WithModelTest(stub))
}

func postModelTest(t *testing.T, h *Handler, path, body string) (*modelTestResponse, int) {
	t.Helper()
	c, rec := newHandlerContextWithBody(path, http.MethodPost, body)
	_ = h.RunModelTest(c) // handlers write the response themselves (handleError)
	if rec.Code != http.StatusOK {
		// Validation/feature failures: body is a GatewayError JSON object.
		return nil, rec.Code
	}
	var resp modelTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &resp, rec.Code
}

// newHandlerContextWithBody builds an echo context carrying a JSON body.
func newHandlerContextWithBody(path, method, body string) (*echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// modelTestResponse mirrors the JSON shape without fighting echo generics.
type modelTestResponse = []modeltest.Result

func TestRunModelTestHappyPath(t *testing.T) {
	stub := &stubModelTest{}
	h := newModelTestHandler(t, stub)
	resp, code := postModelTest(t, h, "/admin/models/test", `{"provider":"local","model":"my-llm"}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body recorded", code)
	}
	if len(*resp) != 2 { // default probes: chat + function_calling
		t.Fatalf("results = %d, want 2 default probes", len(*resp))
	}
	// Results are stored for the results endpoint.
	c, rec := newHandlerContext("/admin/models/test-results")
	if err := h.ListModelTestResults(c); err != nil {
		t.Fatalf("ListModelTestResults() error = %v", err)
	}
	var stored map[string][]modeltest.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(stored["local/my-llm"]) != 2 {
		t.Fatalf("stored results = %d, want 2", len(stored["local/my-llm"]))
	}
}

func TestRunModelTestValidation(t *testing.T) {
	stub := &stubModelTest{}
	h := newModelTestHandler(t, stub)
	cases := []struct{ name, body string }{
		{"missing model", `{"provider":"local"}`},
		{"missing provider", `{"model":"m"}`},
		{"bad probe", `{"provider":"local","model":"m","probes":["telepathy"]}`},
		{"broken json", `{"provider":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, code := postModelTest(t, h, "/admin/models/test", tc.body); code == http.StatusOK {
				t.Fatalf("%s accepted", tc.name)
			}
		})
	}
}

func TestRunModelTestUnavailable(t *testing.T) {
	h := NewHandler(nil, nil)
	c, rec := newHandlerContextWithBody("/admin/models/test", http.MethodPost, `{"provider":"p","model":"m"}`)
	_ = h.RunModelTest(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 feature_unavailable", rec.Code)
	}
}

func TestConfirmModelCapabilities(t *testing.T) {
	stub := &stubModelTest{merged: true}
	h := newModelTestHandler(t, stub)
	c, rec := newHandlerContextWithBody("/admin/models/capabilities", http.MethodPut,
		`{"provider":"local","model":"my-llm","capabilities":{"function_calling":true}}`)
	if err := h.ConfirmModelCapabilities(c); err != nil {
		t.Fatalf("ConfirmModelCapabilities() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !stub.mergeCaps["function_calling"] || stub.mergeSource != core.CapSrcTest {
		t.Fatalf("merge args = %v/%q", stub.mergeCaps, stub.mergeSource)
	}
}

func TestConfirmModelCapabilitiesUnknownModel(t *testing.T) {
	stub := &stubModelTest{merged: false}
	h := newModelTestHandler(t, stub)
	c, rec := newHandlerContextWithBody("/admin/models/capabilities", http.MethodPut,
		`{"provider":"local","model":"ghost","capabilities":{"function_calling":true}}`)
	_ = h.ConfirmModelCapabilities(c) // writes the 404 itself via handleError
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 model-not-found", rec.Code)
	}
}

func TestConfirmModelCapabilitiesValidation(t *testing.T) {
	stub := &stubModelTest{merged: true}
	h := newModelTestHandler(t, stub)
	cases := []struct{ name, body string }{
		{"empty caps", `{"provider":"local","model":"m","capabilities":{}}`},
		{"missing model", `{"provider":"local","capabilities":{"vision":true}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := newHandlerContextWithBody("/admin/models/capabilities", http.MethodPut, tc.body)
			_ = h.ConfirmModelCapabilities(c) // writes the 400 itself via handleError
			if rec.Code == http.StatusOK {
				t.Fatalf("%s accepted", tc.name)
			}
		})
	}
}

func TestModelTestResultsUnavailable(t *testing.T) {
	h := NewHandler(nil, nil)
	c, rec := newHandlerContext("/admin/models/test-results")
	_ = h.ListModelTestResults(c) // writes the 503 itself via handleError
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 feature_unavailable", rec.Code)
	}
}
