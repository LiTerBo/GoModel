package health

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/enterpilot/gomodel/internal/llmclient"
)

// TestTrackerUnhealthy pins the signal route selectors steer on: a target is
// unhealthy when its provider's circuit is open or its own windowed error rate
// crosses the flagging rule. Everything else — no traffic, a first failure, a
// minority of failures, an expired window, caller cancellations — must read
// healthy, because the selector only ever withholds a pick and a false positive
// removes a working target from consideration.
func TestTrackerUnhealthy(t *testing.T) {
	start := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	failed := func() llmclient.ResponseInfo {
		return llmclient.ResponseInfo{
			Provider:   "openai",
			Model:      "gpt-4o",
			StatusCode: 500,
			Error:      errors.New("boom"),
		}
	}
	succeeded := llmclient.ResponseInfo{Provider: "openai", Model: "gpt-4o", StatusCode: 200}

	tests := []struct {
		name   string
		record func(*Tracker, *time.Time)
		check  func(*testing.T, *Tracker)
	}{
		{
			name:   "no recorded traffic is healthy",
			record: func(*Tracker, *time.Time) {},
			check: func(t *testing.T, tracker *Tracker) {
				if tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("a target with no traffic must not be reported unhealthy")
				}
			},
		},
		{
			name: "repeated failures flag the model",
			record: func(tracker *Tracker, _ *time.Time) {
				for range 3 {
					tracker.Record(failed())
				}
				for range 2 {
					tracker.Record(succeeded)
				}
			},
			check: func(t *testing.T, tracker *Tracker) {
				if !tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("3 errors out of 5 requests must read unhealthy")
				}
			},
		},
		{
			name: "a first failure stays healthy",
			record: func(tracker *Tracker, _ *time.Time) {
				tracker.Record(failed())
				tracker.Record(succeeded)
			},
			check: func(t *testing.T, tracker *Tracker) {
				if tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("one failure must not remove the target")
				}
			},
		},
		{
			name: "minority failures on a busy model stay healthy",
			record: func(tracker *Tracker, _ *time.Time) {
				for range 7 {
					tracker.Record(succeeded)
				}
				for range 3 {
					tracker.Record(failed())
				}
			},
			check: func(t *testing.T, tracker *Tracker) {
				if tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("a 30% error rate must not remove the target")
				}
			},
		},
		{
			name: "an open circuit marks every model of the provider",
			record: func(tracker *Tracker, _ *time.Time) {
				tracker.Record(llmclient.ResponseInfo{
					Provider:     "openai",
					Model:        "gpt-4o",
					StatusCode:   503,
					CircuitState: "open",
				})
			},
			check: func(t *testing.T, tracker *Tracker) {
				if !tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("an open circuit must read unhealthy for its model")
				}
				if !tracker.Unhealthy("openai", "never-seen") {
					t.Fatal("an open circuit is provider-wide and must cover models with no traffic")
				}
				if tracker.Unhealthy("other-provider", "gpt-4o") {
					t.Fatal("another provider's circuit state must not leak across providers")
				}
			},
		},
		{
			name: "a half-open circuit is not unhealthy",
			record: func(tracker *Tracker, _ *time.Time) {
				tracker.Record(llmclient.ResponseInfo{
					Provider:     "openai",
					Model:        "gpt-4o",
					StatusCode:   200,
					CircuitState: "half-open",
				})
			},
			check: func(t *testing.T, tracker *Tracker) {
				if tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("half-open is the breaker's probe state, not a failure signal")
				}
			},
		},
		{
			name: "a recovered circuit still reports a failing model",
			record: func(tracker *Tracker, _ *time.Time) {
				for range 3 {
					tracker.Record(failed())
				}
				tracker.Record(llmclient.ResponseInfo{
					Provider:     "openai",
					Model:        "gpt-4o",
					StatusCode:   200,
					CircuitState: "closed",
				})
			},
			check: func(t *testing.T, tracker *Tracker) {
				if !tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("a closed circuit does not erase the model's windowed failures")
				}
			},
		},
		{
			name: "errors outside the window are forgotten",
			record: func(tracker *Tracker, now *time.Time) {
				for range 3 {
					tracker.Record(failed())
				}
				*now = now.Add(Window + time.Minute)
				tracker.Record(succeeded)
			},
			check: func(t *testing.T, tracker *Tracker) {
				if tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("a stale window must not keep the target out of the pool")
				}
			},
		},
		{
			name: "client cancellations are not failures",
			record: func(tracker *Tracker, _ *time.Time) {
				for range 3 {
					tracker.Record(llmclient.ResponseInfo{
						Provider: "openai",
						Model:    "gpt-4o",
						Error:    fmt.Errorf("request aborted: %w", context.Canceled),
					})
				}
			},
			check: func(t *testing.T, tracker *Tracker) {
				if tracker.Unhealthy("openai", "gpt-4o") {
					t.Fatal("caller-side cancellations say nothing about the target")
				}
			},
		},
		{
			name: "an unknown provider is healthy",
			record: func(tracker *Tracker, _ *time.Time) {
				tracker.Record(failed())
			},
			check: func(t *testing.T, tracker *Tracker) {
				if tracker.Unhealthy("ghost", "any-model") {
					t.Fatal("an unknown provider must not read unhealthy")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker, now := newTestTracker(start)
			tt.record(tracker, now)
			tt.check(t, tracker)
		})
	}
}

// TestNilTrackerIsHealthy pins the nil-receiver contract: the gateway wires the
// oracle after construction, and a selector that somehow holds no tracker must
// keep serving instead of panicking on the request path.
func TestNilTrackerIsHealthy(t *testing.T) {
	var tracker *Tracker
	if tracker.Unhealthy("openai", "gpt-4o") {
		t.Fatal("a nil tracker must report healthy")
	}
}
