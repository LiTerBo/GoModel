package auditlog

import "github.com/labstack/echo/v5"

// Capability signal fields for offline observation mining (W2b). These are
// booleans distilled from the request/response at capture time, NOT the
// bodies themselves: capability evidence only needs "was tools present",
// "did the model emit tool_calls", "did the request carry an image" — so the
// signals stay queryable as columns with zero extra privacy surface, whether
// or not body logging is enabled.

// enrichEntryWithCapabilitySignals applies the signal triple to an entry.
func enrichEntryWithCapabilitySignals(entry *LogEntry, signals CapabilitySignals) {
	if entry == nil {
		return
	}
	data := ensureLogData(entry)
	data.CapabilitySignals = &signals
}

// CapabilitySignals are the per-request booleans consumed by the passive
// observation aggregator. A nil pointer means "not captured" (endpoints the
// summarizer does not cover), so mining skips those rows instead of reading
// them as all-false.
type CapabilitySignals struct {
	// HadTools: the request carried tool definitions.
	HadTools bool `json:"had_tools,omitempty" bson:"had_tools,omitempty"`
	// ResponseToolCalls: the model's response contained tool_calls.
	ResponseToolCalls bool `json:"response_tool_calls,omitempty" bson:"response_tool_calls,omitempty"`
	// HadImage: the request carried image content parts.
	HadImage bool `json:"had_image,omitempty" bson:"had_image,omitempty"`
}

// SignalsFromChatRequest distills the request-side signals from a chat
// request. Kept next to the storage shape so the fields and their producer
// evolve together.
func SignalsFromChatRequest(hadTools, hadImage bool) CapabilitySignals {
	return CapabilitySignals{HadTools: hadTools, HadImage: hadImage}
}

// EnrichEntryWithCapabilitySignals records the capability signal triple on
// the request's audit entry. Safe to call for every request: a missing entry
// is a no-op, and nil signals clear nothing (the caller omits the call).
func EnrichEntryWithCapabilitySignals(c *echo.Context, signals CapabilitySignals) {
	if c == nil {
		return
	}
	entry := entryFromContext(c)
	if entry == nil {
		return
	}
	enrichEntryWithCapabilitySignals(entry, signals)
	publishLiveAuditUpdate(c, entry)
}
