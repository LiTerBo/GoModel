package complexity

import (
	"testing"

	"github.com/enterpilot/gomodel/ext"
)

// stubOracle marks targets unhealthy by "provider/model" key, the shape the
// gateway's tracker answers with.
type stubOracle map[string]bool

func (s stubOracle) Unhealthy(provider, model string) bool { return s[provider+"/"+model] }

func tierConfig() Config {
	return Config{Tiers: TierMap{
		Simple:  []string{"lite-model"},
		Medium:  []string{"flash-model"},
		Complex: []string{"flash-model"},
	}}
}

func tierPool() []ext.RouteCandidate {
	return []ext.RouteCandidate{
		{Provider: "p", Qualified: "p/lite-model", Model: "lite-model"},
		{Provider: "p", Qualified: "p/flash-model", Model: "flash-model"},
	}
}

func simpleContent() *ext.RouteContent {
	return &ext.RouteContent{MessageCount: 1, CurrentChars: 10}
}

// TestSelectorSkipsUnhealthyTarget pins the health steering rule: when the
// tier-preferred target is failing, the next candidate in the tier ladder wins
// the request instead of the preferred one.
func TestSelectorSkipsUnhealthyTarget(t *testing.T) {
	t.Parallel()
	s := NewSelector(tierConfig())
	s.SetHealthOracle(stubOracle{"p/lite-model": true})

	got, ok := s.Select(ext.RouteRequest{Candidates: tierPool(), Content: simpleContent()})
	if !ok || got != "p/flash-model" {
		t.Fatalf("Select() = %q/%v, want p/flash-model once the simple tier is failing", got, ok)
	}
}

// TestSelectorFailsOpenWhenEveryTargetIsUnhealthy pins the fail-open rule: a
// pool that is entirely unhealthy is not filtered to nothing (that would
// decline the pick and hand the request to round robin over the same set), so
// the configured tier preference still decides.
func TestSelectorFailsOpenWhenEveryTargetIsUnhealthy(t *testing.T) {
	t.Parallel()
	s := NewSelector(tierConfig())
	s.SetHealthOracle(stubOracle{"p/lite-model": true, "p/flash-model": true})

	got, ok := s.Select(ext.RouteRequest{Candidates: tierPool(), Content: simpleContent()})
	if !ok || got != "p/lite-model" {
		t.Fatalf("Select() = %q/%v, want the tier preference p/lite-model when health filtering empties the pool", got, ok)
	}
}

// TestSelectorWithoutOracleIsUnchanged pins that health steering is additive:
// with no oracle injected — the state before the gateway wires the tracker —
// selection is exactly the tier preference.
func TestSelectorWithoutOracleIsUnchanged(t *testing.T) {
	t.Parallel()
	s := NewSelector(tierConfig())
	got, ok := s.Select(ext.RouteRequest{Candidates: tierPool(), Content: simpleContent()})
	if !ok || got != "p/lite-model" {
		t.Fatalf("Select() = %q/%v, want p/lite-model without an oracle", got, ok)
	}
}

// TestSelectorCapabilityFilterOutranksHealth pins the ordering: a target that
// cannot satisfy the request is never substituted for one that can, even when
// the capable target is failing. Health may only reorder among capable targets.
func TestSelectorCapabilityFilterOutranksHealth(t *testing.T) {
	t.Parallel()
	s := NewSelector(tierConfig())
	s.SetHealthOracle(stubOracle{"p/flash-model": true})

	pool := []ext.RouteCandidate{
		{Provider: "p", Qualified: "p/lite-model", Model: "lite-model"},
		{Provider: "p", Qualified: "p/flash-model", Model: "flash-model", Capabilities: map[string]bool{"vision": true}},
	}
	got, ok := s.Select(ext.RouteRequest{
		Candidates:           pool,
		Content:              simpleContent(),
		RequiredCapabilities: []string{"vision"},
	})
	if !ok || got != "p/flash-model" {
		t.Fatalf("Select() = %q/%v, want the vision-capable target even while it is unhealthy", got, ok)
	}
}

// TestSelectorHealthAppliesPerTarget pins that the oracle is consulted per
// candidate: a failing sibling does not remove the healthy one, and vice versa.
func TestSelectorHealthAppliesPerTarget(t *testing.T) {
	t.Parallel()
	s := NewSelector(tierConfig())
	s.SetHealthOracle(stubOracle{"p/flash-model": true})

	heavy := &ext.RouteContent{MessageCount: 30, UserTurns: 15, CurrentChars: 6000, AvgUserChars: 900, ContextChars: 80000, ToolDefs: 8, HasCode: true}
	got, ok := s.Select(ext.RouteRequest{Candidates: tierPool(), Content: heavy})
	if !ok || got != "p/lite-model" {
		t.Fatalf("Select() = %q/%v, want the healthy simple tier when the preferred complex tier is failing", got, ok)
	}
}

// TestSelectorHealthOracleIsSwappable pins that the oracle can be injected
// after construction (the gateway wires it once the request tracker exists) and
// that a later swap takes effect without rebuilding the selector.
func TestSelectorHealthOracleIsSwappable(t *testing.T) {
	t.Parallel()
	s := NewSelector(tierConfig())
	s.SetHealthOracle(stubOracle{"p/lite-model": true})
	if got, _ := s.Select(ext.RouteRequest{Candidates: tierPool(), Content: simpleContent()}); got != "p/flash-model" {
		t.Fatalf("first Select() = %q, want p/flash-model", got)
	}
	s.SetHealthOracle(stubOracle{})
	if got, _ := s.Select(ext.RouteRequest{Candidates: tierPool(), Content: simpleContent()}); got != "p/lite-model" {
		t.Fatalf("Select() after clearing health = %q, want p/lite-model", got)
	}
}

// TestSelectorIsHealthAware pins the interface the gateway type-asserts on.
func TestSelectorIsHealthAware(t *testing.T) {
	t.Parallel()
	var _ ext.HealthAwareSelector = NewSelector(tierConfig())
}
