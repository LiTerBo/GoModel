// Package modeltest implements offline, operator-triggered model validation:
// it probes a model's real behaviour (which endpoints answer, does it emit
// tool_calls) to verify — never replace — the static inference pipeline.
//
// Results are advisory until an operator confirms them, at which point they
// are persisted as capabilities with the "test" source. A probe that fails
// to confirm anything reports inconclusive: negative evidence from a probe
// is weak (a model can ignore a tool hint without being unable to call), so
// it must never strip a capability the static pipeline granted.
package modeltest

import "time"

// Verdict is the tri-state outcome of one probe. Fail intentionally does not
// exist as a destructive verdict: UNSUPPORTED is only suggested when the
// upstream explicitly rejects the feature (D5+ observation rules), and even
// then it remains an advisory suggestion for operator confirmation.
type Verdict string

const (
	// Pass: the probe positively confirmed the behaviour.
	Pass Verdict = "pass"
	// Inconclusive: transport error, unexpected shape, or silent ignore.
	// Leaves the static judgement untouched.
	Inconclusive Verdict = "inconclusive"
)

// Probe identifies one kind of validation.
type Probe string

const (
	// ProbeChat: the model answers /chat/completions.
	ProbeChat Probe = "chat"
	// ProbeEmbeddings: the model answers /embeddings.
	ProbeEmbeddings Probe = "embeddings"
	// ProbeFunctionCalling: the model emits tool_calls when prompted.
	ProbeFunctionCalling Probe = "function_calling"
)

// Result is the outcome of probing one model with one probe.
type Result struct {
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Probe     Probe     `json:"probe"`
	Verdict   Verdict   `json:"verdict"`
	Detail    string    `json:"detail,omitempty"`
	LatencyMs int64     `json:"latency_ms"`
	At        time.Time `json:"at"`
}

// Failed is a convenience for callers: only Inconclusive exists for
// negatives, so "not passed" is the single useful complement.
func (r Result) Passed() bool { return r.Verdict == Pass }
