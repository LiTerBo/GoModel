package modeldata

import (
	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/core"
)

// ObservationRow is one audit entry flattened for mining: what the
// aggregation needs from an entry, nothing else. Produced by the audit-log
// reader; consumed by AggregateObservations.
type ObservationRow struct {
	Provider   string
	Model      string
	SessionID  string
	Signals    *auditlog.CapabilitySignals
	ErrorType  string
	StatusCode int
}

// Suggestion is a mined capability verdict awaiting operator confirmation.
// Only support/unsupport exist: rows that cannot establish either simply
// produce no suggestion (INCONCLUSIVE never becomes one).
type Suggestion struct {
	Provider        string
	Model           string
	Capability      string
	Verdict         string // "support" | "unsupport"
	EvidenceSession int    // distinct sessions backing the verdict
	Source          string // core.CapSrcObserved
}

// Suggestion verdicts.
const (
	VerdictSupport   = "support"
	VerdictUnsupport = "unsupport"
)

// DefaultObservedSessions is the trial threshold for "distinct sessions
// observed" (Q4/D5+): three independent conversations seeing the same
// behaviour is strong enough evidence to propose, weak enough that an
// operator sees the number before confirming.
const DefaultObservedSessions = 3

// AggregateObservations mines capability suggestions from observation rows.
// Semantics (D5+):
//   - support:     ≥N distinct sessions with had_tools ∧ response_tool_calls
//   - unsupport:   ≥N distinct sessions with had_tools ∧ upstream 4xx whose
//     error names tools (the only sanctioned negative)
//   - everything else — including success-but-no-tool-calls — is
//     INCONCLUSIVE and produces nothing.
//
// The result is advisory only: nothing here mutates model metadata; callers
// persist via the operator-gated confirmation channel.
func AggregateObservations(rows []ObservationRow, threshold int) []Suggestion {
	if threshold <= 0 {
		threshold = DefaultObservedSessions
	}
	type counters struct{ support, unsupport map[string]int }
	byModel := make(map[string]*counters)
	for _, row := range rows {
		if row.Signals == nil || !row.Signals.HadTools {
			continue
		}
		if row.Model == "" || row.SessionID == "" {
			continue // unidentified traffic is not evidence
		}
		key := row.Provider + "/" + row.Model
		c, ok := byModel[key]
		if !ok {
			c = &counters{support: map[string]int{}, unsupport: map[string]int{}}
			byModel[key] = c
		}
		if row.Signals.ResponseToolCalls {
			c.support[row.SessionID]++
			continue
		}
		if row.unsupportedToolRejection() {
			c.unsupport[row.SessionID]++
		}
		// Success without tool_calls: INCONCLUSIVE by design.
	}
	suggestions := make([]Suggestion, 0, len(byModel))
	for key, c := range byModel {
		provider, model := splitQualified(key)
		for capName, sessions := range map[string]map[string]int{"function_calling": c.support} {
			if len(sessions) >= threshold {
				suggestions = append(suggestions, Suggestion{
					Provider: provider, Model: model, Capability: capName,
					Verdict: VerdictSupport, EvidenceSession: len(sessions),
					Source: core.CapSrcObserved,
				})
			}
		}
		if len(c.unsupport) >= threshold {
			suggestions = append(suggestions, Suggestion{
				Provider: provider, Model: model, Capability: "function_calling",
				Verdict: VerdictUnsupport, EvidenceSession: len(c.unsupport),
				Source: core.CapSrcObserved,
			})
		}
	}
	return suggestions
}

// unsupportedToolRejection reports whether this row is an upstream 4xx whose
// error text names tools/functions — the only negative evidence the rules
// accept. errorType carries the classified gateway error type; the message
// check happens at classification time.
func (r ObservationRow) unsupportedToolRejection() bool {
	return !r.Signals.ResponseToolCalls &&
		r.StatusCode >= 400 && r.StatusCode < 500 &&
		toolErrorTypes[r.ErrorType]
}

// toolErrorTypes lists classified error types that indicate the upstream
// rejected the request itself (not quota/auth), the shape a tools refusal
// arrives in.
var toolErrorTypes = map[string]bool{
	"invalid_request_error": true,
	"provider_error":        true,
	"model_not_found":       true,
}

func splitQualified(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}
