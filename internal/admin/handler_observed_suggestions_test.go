package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
)

func TestListObservedSuggestions(t *testing.T) {
	tools, calls := true, true
	entries := []auditlog.LogEntry{
		// 3 distinct sessions of tools → tool_calls on one model: support suggestion.
		{Timestamp: time.Now(), RequestedModel: "ollama/qwen", ResolvedModel: "qwen", Provider: "ollama", SessionID: "s1", StatusCode: 200, Data: &auditlog.LogData{CapabilitySignals: &auditlog.CapabilitySignals{HadTools: tools, ResponseToolCalls: calls}}},
		{Timestamp: time.Now(), RequestedModel: "ollama/qwen", ResolvedModel: "qwen", Provider: "ollama", SessionID: "s2", StatusCode: 200, Data: &auditlog.LogData{CapabilitySignals: &auditlog.CapabilitySignals{HadTools: tools, ResponseToolCalls: calls}}},
		{Timestamp: time.Now(), RequestedModel: "ollama/qwen", ResolvedModel: "qwen", Provider: "ollama", SessionID: "s3", StatusCode: 200, Data: &auditlog.LogData{CapabilitySignals: &auditlog.CapabilitySignals{HadTools: tools, ResponseToolCalls: calls}}},
		// Same session again — must NOT raise the distinct count.
		{Timestamp: time.Now(), RequestedModel: "ollama/qwen", ResolvedModel: "qwen", Provider: "ollama", SessionID: "s3", StatusCode: 200, Data: &auditlog.LogData{CapabilitySignals: &auditlog.CapabilitySignals{HadTools: tools, ResponseToolCalls: calls}}},
		// No signals captured — skipped.
		{Timestamp: time.Now(), RequestedModel: "ollama/plain", Provider: "ollama"},
	}
	h := NewHandler(nil, nil)
	h.auditReader = &mockAuditReader{
		logResult: &auditlog.LogListResult{Entries: entries, Total: len(entries)},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/models/observed-suggestions", nil)
	rec := httptest.NewRecorder()
	cctx := e.NewContext(req, rec)

	if err := h.ListObservedSuggestions(cctx); err != nil {
		t.Fatalf("ListObservedSuggestions: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Suggestions []struct {
			Provider   string `json:"provider"`
			Model      string `json:"model"`
			Capability string `json:"capability"`
			Verdict    string `json:"verdict"`
			Evidence   int    `json:"evidence_session"`
		} `json:"suggestions"`
		Threshold int `json:"threshold_sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body.Threshold != 3 {
		t.Fatalf("threshold = %d", body.Threshold)
	}
	if len(body.Suggestions) != 1 {
		t.Fatalf("got %d suggestions, want 1", len(body.Suggestions))
	}
	s := body.Suggestions[0]
	if s.Provider != "ollama" || s.Model != "qwen" || s.Capability != "function_calling" || s.Verdict != "support" || s.Evidence != 3 {
		t.Fatalf("suggestion wrong: %+v", s)
	}
}

func TestListObservedSuggestionsNoReader(t *testing.T) {
	h := NewHandler(nil, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/models/observed-suggestions", nil)
	rec := httptest.NewRecorder()
	cctx := e.NewContext(req, rec)
	if err := h.ListObservedSuggestions(cctx); err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
