// Package complexity implements a complexity-aware ext.RouteSelector: it
// scores the incoming request summary (never message bodies), maps the score
// to a model tier, filters candidates by capability (vision hard,
// function_calling soft), and answers with the best candidate — or declines,
// letting core fall back to weighted round robin.
//
// It is a core-shipped reference implementation of the adaptive extension
// point; operators may register their own selector instead.
package complexity

// Config declares the routing tiers under extensions.complexity_routing in
// config.yaml. Zero Config means "selector installed, no tiers configured"
// — every Select declines to round robin.
type Config struct {
	// Enabled wires the selector into the gateway binary (see
	// cmd/gomodel setup). Default: false.
	Enabled bool `yaml:"enabled"`
	// Thresholds are the complexity-score boundaries per tier. A score
	// below SimpleMedium is simple; below MediumComplex is medium; below
	// ComplexVery is complex; otherwise very_complex. Values are
	// empirical during the one-month trial window (D-Q4) and are expected
	// to be recalibrated from audit-log samples afterwards.
	Thresholds Thresholds `yaml:"thresholds"`
	// Tiers maps each complexity tier to its preferred model labels. The
	// first qualified model of a tier that is present in the candidate
	// pool wins; later entries are the in-tier upgrade order.
	Tiers TierMap `yaml:"tiers"`
	// VirtualModel restricts the selector to one virtual model source
	// (e.g. "smart"). Empty applies to every adaptive redirect.
	VirtualModel string `yaml:"virtual_model"`
}

// Thresholds are the boundaries of the four complexity bands.
type Thresholds struct {
	SimpleMedium  float64 `yaml:"simple_medium"`
	MediumComplex float64 `yaml:"medium_complex"`
	ComplexVery   float64 `yaml:"complex_very_complex"`
}

// TierMap holds the model order per tier; empty tiers fall back to the
// selector's built-in ladder (see Engine.fallbackTier).
type TierMap struct {
	Simple      []string `yaml:"simple"`
	Medium      []string `yaml:"medium"`
	Complex     []string `yaml:"complex"`
	VeryComplex []string `yaml:"very_complex"`
}

// ForTier returns the ordered model list for one tier, or nil when unset.
func (t TierMap) ForTier(tier string) []string {
	switch tier {
	case TierSimple:
		return t.Simple
	case TierMedium:
		return t.Medium
	case TierComplex:
		return t.Complex
	case TierVeryComplex:
		return t.VeryComplex
	}
	return nil
}

// Tier names produced by the estimator.
const (
	TierSimple      = "simple"
	TierMedium      = "medium"
	TierComplex     = "complex"
	TierVeryComplex = "very_complex"
)

// DefaultThresholds are the trial-run human estimates (Q4). The bands were
// picked against hand-written representative requests under this package's
// own score distribution (simple ≈0.12, tool-assisted task ≈0.76, agent
// session ≈0.96) — deliberately NOT copied from model-router's K-Means
// values, whose 8-feature estimator saturates differently. Recalibrate
// against GoModel's own audit-log traffic after the one-month trial.
func DefaultThresholds() Thresholds {
	return Thresholds{SimpleMedium: 0.45, MediumComplex: 0.62, ComplexVery: 0.85}
}
