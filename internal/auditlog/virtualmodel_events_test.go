package auditlog

import (
	"testing"
	"time"
)

type captureLogger struct {
	entries []*LogEntry
	enabled bool
}

func (l *captureLogger) Write(entry *LogEntry) { l.entries = append(l.entries, entry) }
func (l *captureLogger) Config() Config        { return Config{Enabled: l.enabled} }
func (l *captureLogger) Close() error          { return nil }

// T13-E: management actions on a virtual model land in the same durable audit
// trail as requests and authentication events, so the console's audit queries
// can answer "what changed, and who did the change reach" later.
func TestVirtualModelChangeEntryMapsToLifecycleEntry(t *testing.T) {
	change := VirtualModelChange{
		Timestamp:   time.Date(2026, 9, 12, 22, 30, 0, 0, time.UTC),
		Action:      VirtualModelActionRetarget,
		Source:      "smart",
		OldTargets:  []string{"openai/gpt-4o"},
		NewTargets:  []string{"openai/gpt-4o-mini", "openai/gpt-4o"},
		Locked:      true,
		Credentials: 2,
		UserPaths:   1,
		Method:      "PUT",
		Path:        "/admin/virtual-models",
		ClientIP:    "192.0.2.10",
		UserAgent:   "Mozilla/5.0",
		RequestID:   "req-1",
	}

	entry := VirtualModelChangeEntry(change)
	if entry == nil {
		t.Fatalf("VirtualModelChangeEntry() = nil, want an entry")
	}
	if entry.Provider != virtualModelEventProvider {
		t.Fatalf("Provider = %q, want %q", entry.Provider, virtualModelEventProvider)
	}
	if entry.Data == nil || entry.Data.EventType != "virtual_model_retarget" {
		t.Fatalf("Data.EventType = %#v, want virtual_model_retarget", entry.Data)
	}
	snapshot := entry.Data.VirtualModel
	if snapshot == nil {
		t.Fatalf("Data.VirtualModel = nil, want the change snapshot")
	}
	if snapshot.Source != "smart" || snapshot.Action != VirtualModelActionRetarget || !snapshot.Locked {
		t.Fatalf("snapshot = %#v, want smart/retarget/locked", snapshot)
	}
	if len(snapshot.OldTargets) != 1 || len(snapshot.NewTargets) != 2 || snapshot.Credentials != 2 || snapshot.UserPaths != 1 {
		t.Fatalf("snapshot targets/counts = %#v, want both sides and the reachability counts", snapshot)
	}
	if entry.Timestamp.IsZero() || entry.Method != "PUT" || entry.Path != "/admin/virtual-models" ||
		entry.ClientIP != "192.0.2.10" || entry.RequestID != "req-1" || entry.Data.UserAgent != "Mozilla/5.0" {
		t.Fatalf("entry = %#v, want the request metadata carried over", entry)
	}
}

func TestVirtualModelChangeEntryStampsTimeAndEventTypes(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{action: VirtualModelActionLock, want: "virtual_model_lock"},
		{action: VirtualModelActionUnlock, want: "virtual_model_unlock"},
		{action: VirtualModelActionDelete, want: "virtual_model_delete"},
		{action: "something-else", want: "virtual_model_unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.action, func(t *testing.T) {
			entry := VirtualModelChangeEntry(VirtualModelChange{Action: tc.action, Source: "smart"})
			if entry.Data == nil || entry.Data.EventType != tc.want {
				t.Fatalf("EventType = %#v, want %q", entry.Data, tc.want)
			}
			// A missing timestamp is stamped so the entry is still queryable.
			if entry.Timestamp.IsZero() {
				t.Fatalf("Timestamp = zero, want it stamped")
			}
		})
	}
}

func TestVirtualModelChangeSinkRespectsAuditConfiguration(t *testing.T) {
	change := VirtualModelChange{Action: VirtualModelActionDelete, Source: "smart", Forced: true}

	disabled := &captureLogger{}
	NewVirtualModelChangeSink(disabled)(change)
	if len(disabled.entries) != 0 {
		t.Fatalf("disabled audit wrote %d entries, want none", len(disabled.entries))
	}

	enabled := &captureLogger{enabled: true}
	NewVirtualModelChangeSink(enabled)(change)
	if len(enabled.entries) != 1 {
		t.Fatalf("enabled audit wrote %d entries, want 1", len(enabled.entries))
	}
	if snapshot := enabled.entries[0].Data.VirtualModel; snapshot == nil || !snapshot.Forced {
		t.Fatalf("entry snapshot = %#v, want Forced recorded", snapshot)
	}

	// A sink built without a logger must not panic.
	NewVirtualModelChangeSink(nil)(change)
}
