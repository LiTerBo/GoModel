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

// observationScanLimit caps entries scanned per suggestion request. Mining
// filters in memory over the JSON data column, so the scan stays bounded;
// three distinct sessions are usually found well inside this cap.
const observationScanLimit = 5000

// defaultObservationWindow is how far back the miner looks for observation
// rows. Matches the capability-error rollup window: a week of traffic is
// current enough to propose from and survives quiet weekends.
const defaultObservationWindow = 7 * 24 * time.Hour

// ListObservedSuggestions serves GET /admin/models/observed-suggestions: the
// passive-observation queue (I-4/W2b). It mines recent audit entries with
// AggregateObservations and returns the suggestions awaiting operator
// confirmation via PUT /admin/models/observed-capabilities. Advisory only —
// nothing here mutates model metadata.
func (h *Handler) ListObservedSuggestions(c *echo.Context) error {
	if h.auditReader == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "audit log is not enabled",
		})
	}
	days := defaultObservationWindow
	if raw := c.QueryParam("days"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 90 {
			days = time.Duration(n) * 24 * time.Hour
		}
	}
	threshold := modeldata.DefaultObservedSessions
	if raw := c.QueryParam("threshold"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			threshold = n
		}
	}
	now := time.Now()
	params := auditlog.LogQueryParams{
		QueryParams: auditlog.QueryParams{
			StartDate: now.Add(-days).Truncate(24 * time.Hour),
			EndDate:   now,
		},
		Limit: observationScanLimit,
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
	rows := observationRowsFromEntries(result.Entries)
	suggestions := modeldata.AggregateObservations(rows, threshold)
	return c.JSON(http.StatusOK, map[string]any{
		"suggestions":       suggestions,
		"threshold_sessions": threshold,
		"window_days":       int(days.Hours() / 24),
	})
}

// observationRowsFromEntries flattens audit entries into miner rows. Signals
// are booleans distilled at capture time; entries without captured signals
// are skipped so partial coverage is never read as all-false.
func observationRowsFromEntries(entries []auditlog.LogEntry) []modeldata.ObservationRow {
	rows := make([]modeldata.ObservationRow, 0, len(entries))
	for i := range entries {
		e := &entries[i]
		if e.Data == nil || e.Data.CapabilitySignals == nil {
			continue
		}
		model := e.ResolvedModel
		if model == "" {
			model = e.RequestedModel
			if prefix := e.Provider + "/"; strings.HasPrefix(model, prefix) {
				model = strings.TrimPrefix(model, prefix)
			}
		}
		rows = append(rows, modeldata.ObservationRow{
			Provider:   providerOfEntry(e),
			Model:      model,
			SessionID:  e.SessionID,
			Signals:    e.Data.CapabilitySignals,
			ErrorType:  e.ErrorType,
			StatusCode: e.StatusCode,
		})
	}
	return rows
}
