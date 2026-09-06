package modeldata

import (
	"testing"

	"github.com/enterpilot/gomodel/internal/auditlog"
)

func row(provider, model, session string, signals *auditlog.CapabilitySignals, status int, errType string) ObservationRow {
	return ObservationRow{Provider: provider, Model: model, SessionID: session, Signals: signals, StatusCode: status, ErrorType: errType}
}

func toolSignals(toolCalls bool) *auditlog.CapabilitySignals {
	return &auditlog.CapabilitySignals{HadTools: true, ResponseToolCalls: toolCalls}
}

func TestAggregateSupportAtThreshold(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "s1", toolSignals(true), 200, ""),
		row("local", "m1", "s2", toolSignals(true), 200, ""),
		row("local", "m1", "s3", toolSignals(true), 200, ""),
	}
	got := AggregateObservations(rows, DefaultObservedSessions)
	if len(got) != 1 {
		t.Fatalf("suggestions = %+v, want 1", got)
	}
	s := got[0]
	if s.Capability != "function_calling" || s.Verdict != VerdictSupport || s.EvidenceSession != 3 || s.Source != "observed" {
		t.Fatalf("suggestion = %+v", s)
	}
}

func TestAggregateBelowThresholdProducesNothing(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "s1", toolSignals(true), 200, ""),
		row("local", "m1", "s2", toolSignals(true), 200, ""),
	}
	if got := AggregateObservations(rows, DefaultObservedSessions); len(got) != 0 {
		t.Fatalf("2 sessions must stay INCONCLUSIVE, got %+v", got)
	}
}

// Same session twice counts once: distinct sessions, not rows.
func TestAggregateDeduplicatesSessions(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "s1", toolSignals(true), 200, ""),
		row("local", "m1", "s1", toolSignals(true), 200, ""),
		row("local", "m1", "s1", toolSignals(true), 200, ""),
	}
	if got := AggregateObservations(rows, DefaultObservedSessions); len(got) != 0 {
		t.Fatalf("one session repeated must not reach threshold, got %+v", got)
	}
}

// Success WITHOUT tool_calls is the trap this whole design exists to avoid:
// it must never produce an unsupport suggestion.
func TestAggregateSuccessWithoutToolCallsIsInconclusive(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "s1", toolSignals(false), 200, ""),
		row("local", "m1", "s2", toolSignals(false), 200, ""),
		row("local", "m1", "s3", toolSignals(false), 200, ""),
		row("local", "m1", "s4", toolSignals(false), 200, ""),
	}
	if got := AggregateObservations(rows, DefaultObservedSessions); len(got) != 0 {
		t.Fatalf("silent ignore must stay INCONCLUSIVE, got %+v", got)
	}
}

func TestAggregateUnsupportOnExplicitRejection(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "s1", toolSignals(false), 400, "invalid_request_error"),
		row("local", "m1", "s2", toolSignals(false), 400, "invalid_request_error"),
		row("local", "m1", "s3", toolSignals(false), 400, "invalid_request_error"),
	}
	got := AggregateObservations(rows, DefaultObservedSessions)
	if len(got) != 1 || got[0].Verdict != VerdictUnsupport {
		t.Fatalf("suggestions = %+v, want 1 unsupport", got)
	}
}

// A model that mostly supports tools but saw one rejection must NOT flip to
// unsupport: the evidence streams are counted separately and support wins.
func TestAggregateMixedEvidence(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "s1", toolSignals(true), 200, ""),
		row("local", "m1", "s2", toolSignals(true), 200, ""),
		row("local", "m1", "s3", toolSignals(true), 200, ""),
		row("local", "m1", "s9", toolSignals(false), 400, "invalid_request_error"),
	}
	got := AggregateObservations(rows, DefaultObservedSessions)
	if len(got) != 1 || got[0].Verdict != VerdictSupport {
		t.Fatalf("suggestions = %+v, want support only", got)
	}
}

func TestAggregateIgnoresUnidentifiedTraffic(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "", toolSignals(true), 200, ""),
		row("local", "m1", "s1", nil, 200, ""),              // no signals captured
		row("local", "m1", "s2", toolSignals(false), 0, ""), // no tools in request
	}
	if got := AggregateObservations(rows, DefaultObservedSessions); len(got) != 0 {
		t.Fatalf("unidentified rows must be skipped, got %+v", got)
	}
}

func TestAggregateSeparatesModels(t *testing.T) {
	t.Parallel()
	rows := []ObservationRow{
		row("local", "m1", "s1", toolSignals(true), 200, ""),
		row("local", "m1", "s2", toolSignals(true), 200, ""),
		row("local", "m1", "s3", toolSignals(true), 200, ""),
		row("local", "m2", "s1", toolSignals(true), 200, ""),
		row("local", "m2", "s2", toolSignals(true), 200, ""),
	}
	got := AggregateObservations(rows, DefaultObservedSessions)
	if len(got) != 1 || got[0].Model != "m1" {
		t.Fatalf("suggestions = %+v, want only m1 at threshold", got)
	}
}
