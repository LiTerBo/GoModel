package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
)

func TestHandleCapabilityErrors(t *testing.T) {
	entries := []auditlog.LogEntry{
		{
			Timestamp:      time.Now().Add(-time.Hour),
			RequestedModel: "ollama/llava",
			ResolvedModel:  "llava",
			Provider:       "ollama",
			Data:           &auditlog.LogData{CapabilityError: "capability_mismatch"},
		},
		{
			Timestamp:      time.Now(),
			RequestedModel: "ollama/qwen",
			Provider:       "ollama",
			Data:           &auditlog.LogData{CapabilityError: "type_mismatch"},
		},
		{Timestamp: time.Now(), RequestedModel: "ollama/plain"}, // no verdict → skipped
	}
	h := NewHandler(nil, nil)
	h.auditReader = &mockAuditReader{
		logResult: &auditlog.LogListResult{Entries: entries, Total: len(entries)},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/models/capability-errors", nil)
	rec := httptest.NewRecorder()
	cctx := e.NewContext(req, rec)
	cctx.SetRequest(req)

	if err := h.handleCapabilityErrors(cctx); err != nil {
		t.Fatalf("handleCapabilityErrors: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		CapabilityErrors []struct {
			Provider    string `json:"provider"`
			Model       string `json:"model"`
			LatestKind  string `json:"latest_kind"`
			Occurrences int    `json:"occurrences"`
		} `json:"capability_errors"`
		WindowDays int `json:"window_days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if len(body.CapabilityErrors) != 2 {
		t.Fatalf("got %d rollups, want 2: %+v", len(body.CapabilityErrors), body)
	}
	// newest first: qwen type_mismatch
	first := body.CapabilityErrors[0]
	if first.Model != "qwen" || first.LatestKind != "type_mismatch" || first.Occurrences != 1 {
		t.Fatalf("first rollup wrong: %+v", first)
	}
	if body.WindowDays != 7 {
		t.Fatalf("window = %d, want 7", body.WindowDays)
	}
}

func TestHandleCapabilityErrorsNoReader(t *testing.T) {
	h := NewHandler(nil, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/admin/models/capability-errors", nil)
	rec := httptest.NewRecorder()
	cctx := e.NewContext(req, rec)
	if err := h.handleCapabilityErrors(cctx); err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

var _ = context.Background
