package modeldata

import (
	"testing"
	"time"
)

func TestAggregateCapabilityErrors(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	rows := []CapabilityErrorRow{
		{Provider: "ollama", Model: "llava", Kind: "capability_mismatch", Timestamp: base.Add(-2 * time.Hour)},
		{Provider: "ollama", Model: "llava", Kind: "capability_mismatch", Timestamp: base.Add(-1 * time.Hour)},
		{Provider: "ollama", Model: "qwen", Kind: "type_mismatch", Timestamp: base.Add(-30 * time.Minute)},
		{Provider: "ollama", Model: "qwen", Kind: "capability_mismatch", Timestamp: base.Add(-10 * time.Minute)},
		{Provider: "", Model: "orphan", Kind: "type_mismatch", Timestamp: base}, // skipped: no provider
		{Provider: "ollama", Model: "", Kind: "type_mismatch", Timestamp: base}, // skipped: no model
		{Provider: "openai", Model: "gpt-x", Kind: "", Timestamp: base},         // skipped: no verdict
	}
	got := AggregateCapabilityErrors(rows)
	if len(got) != 2 {
		t.Fatalf("got %d rollups, want 2: %+v", len(got), got)
	}
	// Ordered by last-seen desc: qwen (10m ago) before llava (1h ago).
	if got[0].Model != "qwen" || got[0].LatestKind != "capability_mismatch" || got[0].Occurrences != 2 {
		t.Fatalf("qwen rollup wrong: %+v", got[0])
	}
	if got[1].Model != "llava" || got[1].LatestKind != "capability_mismatch" || got[1].Occurrences != 2 {
		t.Fatalf("llava rollup wrong: %+v", got[1])
	}
	if !got[0].LastSeen.Equal(base.Add(-10 * time.Minute)) {
		t.Fatalf("last seen wrong: %v", got[0].LastSeen)
	}
}

func TestAggregateCapabilityErrorsEmpty(t *testing.T) {
	t.Parallel()
	if got := AggregateCapabilityErrors(nil); len(got) != 0 {
		t.Fatalf("nil rows produced %d rollups", len(got))
	}
}
