package app

import (
	"testing"

	"github.com/enterpilot/gomodel/ext"
)

// healthOracleStub answers "healthy" for everything; the wiring test only cares
// whether the oracle reached the selector.
type healthOracleStub struct{}

func (healthOracleStub) Unhealthy(string, string) bool { return false }

// healthAwareSelectorStub implements ext.HealthAwareSelector.
type healthAwareSelectorStub struct{ oracle ext.HealthOracle }

func (s *healthAwareSelectorStub) Name() string                            { return "aware" }
func (s *healthAwareSelectorStub) Select(ext.RouteRequest) (string, bool)  { return "", false }
func (s *healthAwareSelectorStub) OnAttemptStart(ext.RouteTarget)          {}
func (s *healthAwareSelectorStub) OnAttemptEnd(ext.RouteOutcome)           {}
func (s *healthAwareSelectorStub) SetHealthOracle(oracle ext.HealthOracle) { s.oracle = oracle }

// healthBlindSelectorStub implements only ext.RouteSelector: a selector written
// before the health oracle existed must keep working untouched.
type healthBlindSelectorStub struct{}

func (healthBlindSelectorStub) Name() string                           { return "blind" }
func (healthBlindSelectorStub) Select(ext.RouteRequest) (string, bool) { return "", false }
func (healthBlindSelectorStub) OnAttemptStart(ext.RouteTarget)         {}
func (healthBlindSelectorStub) OnAttemptEnd(ext.RouteOutcome)          {}

// TestAttachRouteHealth pins the gateway wiring: a selector that accepts a
// health oracle receives the request tracker, and every other shape — a
// selector without the interface, a nil selector, a missing oracle — is left
// alone rather than panicking at startup.
func TestAttachRouteHealth(t *testing.T) {
	t.Parallel()

	t.Run("aware selector receives the oracle", func(t *testing.T) {
		t.Parallel()
		selector := &healthAwareSelectorStub{}
		oracle := healthOracleStub{}
		if !attachRouteHealth(selector, oracle) {
			t.Fatal("attachRouteHealth() = false, want true for a health-aware selector")
		}
		if selector.oracle != oracle {
			t.Fatalf("selector oracle = %#v, want the injected oracle", selector.oracle)
		}
	})

	t.Run("a selector without the interface is untouched", func(t *testing.T) {
		t.Parallel()
		if attachRouteHealth(healthBlindSelectorStub{}, healthOracleStub{}) {
			t.Fatal("attachRouteHealth() = true for a selector that does not accept an oracle")
		}
	})

	t.Run("no selector is not an error", func(t *testing.T) {
		t.Parallel()
		if attachRouteHealth(nil, healthOracleStub{}) {
			t.Fatal("attachRouteHealth(nil) = true")
		}
	})

	t.Run("a missing oracle leaves the selector alone", func(t *testing.T) {
		t.Parallel()
		selector := &healthAwareSelectorStub{}
		if attachRouteHealth(selector, nil) {
			t.Fatal("attachRouteHealth() = true without an oracle")
		}
		if selector.oracle != nil {
			t.Fatal("selector oracle must stay nil when no oracle is available")
		}
	})
}
