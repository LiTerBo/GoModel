package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/modeldata"
)

// defaultCapabilityErrorWindow is how far back the WARN rollup looks. Runtime
// mismatches are operational signals: a week of traffic is current enough to
// warn on and old enough to survive quiet weekends.
const defaultCapabilityErrorWindow = 7 * 24 * time.Hour

// capabilityErrorScanLimit caps entries scanned per rollup request. The
// filter is in-memory over the JSON data column, so the scan must stay
// bounded; mismatches are rare, the cap is generous.
const capabilityErrorScanLimit = 5000

// handleCapabilityErrors serves GET /admin/models/capability-errors: the
// per-model runtime mismatch rollup (W3/K-3) the models page's WARN state
// reads. Query params: days (window, default 7), provider/model filters.
func (h *Handler) handleCapabilityErrors(c *echo.Context) error {
	if h.auditReader == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "audit log is not enabled",
		})
	}
	days := defaultCapabilityErrorWindow
	if raw := c.QueryParam("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 90 {
			days = time.Duration(n) * 24 * time.Hour
		}
	}
	now := time.Now()
	params := auditlog.LogQueryParams{
		QueryParams: auditlog.QueryParams{
			StartDate: now.Add(-days).Truncate(24 * time.Hour),
			EndDate:   now,
		},
		Limit: capabilityErrorScanLimit,
	}
	if p := c.QueryParam("provider"); p != "" {
		params.Provider = p
	}
	if m := c.QueryParam("model"); m != "" {
		params.RequestedModel = m
	}
	result, err := h.auditReader.GetLogs(c.Request().Context(), params)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	rows := make([]modeldata.CapabilityErrorRow, 0, len(result.Entries))
	for i := range result.Entries {
		e := &result.Entries[i]
		kind := ""
		if e.Data != nil {
			kind = e.Data.CapabilityError
		}
		if kind == "" {
			continue
		}
		rows = append(rows, modeldata.CapabilityErrorRow{
			Provider:  providerOfEntry(e),
			Model:     modelOfEntry(e),
			Kind:      kind,
			Timestamp: e.Timestamp,
		})
	}
	rollup := modeldata.AggregateCapabilityErrors(rows)
	return c.JSON(http.StatusOK, map[string]any{
		"capability_errors": rollup,
		"window_days":       int(days.Hours() / 24),
	})
}

// providerOfEntry picks the best provider label for a mismatch row: the
// resolved provider name when present, else the canonical provider type.
func providerOfEntry(e *auditlog.LogEntry) string {
	if e.ProviderName != "" {
		return e.ProviderName
	}
	return e.Provider
}

// modelOfEntry resolves the executed model id for a mismatch row, stripping
// a qualified "provider/model" prefix from the requested selector when it
// matches the row's provider so the rollup key matches catalog entries.
func modelOfEntry(e *auditlog.LogEntry) string {
	if e.ResolvedModel != "" {
		return e.ResolvedModel
	}
	model := e.RequestedModel
	if prefix := e.Provider + "/"; strings.HasPrefix(model, prefix) {
		model = strings.TrimPrefix(model, prefix)
	}
	return model
}
