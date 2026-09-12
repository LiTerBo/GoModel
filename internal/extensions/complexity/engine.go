package complexity

import (
	"github.com/enterpilot/gomodel/ext"
)

// hardCapabilities are filtered out of the pool when a candidate lacks them:
// a model that cannot see images will never answer an image request, so
// routing there is a guaranteed failure.
var hardCapabilities = map[string]bool{"vision": true}

// Engine turns a RouteRequest into a candidate pick: score → tier →
// capability filtering → request-health filtering → ordered model preference.
type Engine struct {
	thresholds Thresholds
	tiers      TierMap
	health     ext.HealthOracle
}

// NewEngine builds an engine from configuration.
func NewEngine(cfg Config) *Engine {
	thresholds := cfg.Thresholds
	if thresholds == (Thresholds{}) {
		thresholds = DefaultThresholds()
	}
	return &Engine{thresholds: thresholds, tiers: cfg.Tiers}
}

// Select picks a qualified model for the request, or "" when it declines
// (no candidates, nothing configured, or no hard-capable candidate).
// Declining is normal: core then applies weighted round robin.
func (e *Engine) Select(req ext.RouteRequest) string {
	if len(req.Candidates) == 0 {
		return ""
	}
	pool := e.filterCapabilities(req.Candidates, req.RequiredCapabilities)
	if len(pool) == 0 {
		return ""
	}
	pool = e.filterUnhealthy(pool)
	tier := Classify(Score(req.Content), e.thresholds)
	return matchPreferred(pool, e.orderFor(tier))
}

// filterUnhealthy drops candidates the health oracle reports as failing, so a
// target that is timing out or serving 429s does not win a request just because
// its tier is preferred or its session pin is warm. Capability filtering runs
// first: a model that cannot satisfy the request is never a substitute for one
// that can.
//
// It fails open. When every candidate is unhealthy, filtering the pool to
// nothing would decline the pick and hand the request to round robin over the
// same failing set — the tier preference is what the operator configured, so
// the unfiltered pool is returned and the pick proceeds as if health were
// unknown.
func (e *Engine) filterUnhealthy(candidates []ext.RouteCandidate) []ext.RouteCandidate {
	if e.health == nil || len(candidates) == 0 {
		return candidates
	}
	healthy := make([]ext.RouteCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !e.health.Unhealthy(c.Provider, c.Model) {
			healthy = append(healthy, c)
		}
	}
	if len(healthy) == 0 {
		return candidates
	}
	return healthy
}

func (e *Engine) filterCapabilities(candidates []ext.RouteCandidate, required []string) []ext.RouteCandidate {
	var hard, soft []string
	for _, capName := range required {
		if hardCapabilities[capName] {
			hard = append(hard, capName)
		} else {
			soft = append(soft, capName)
		}
	}
	pool := make([]ext.RouteCandidate, 0, len(candidates))
	for _, c := range candidates {
		if hasAll(c.Capabilities, hard) {
			pool = append(pool, c)
		}
	}
	if len(soft) == 0 || len(pool) == 0 {
		return pool
	}
	preferred := make([]ext.RouteCandidate, 0, len(pool))
	for _, c := range pool {
		if hasAll(c.Capabilities, soft) {
			preferred = append(preferred, c)
		}
	}
	if len(preferred) > 0 {
		return preferred
	}
	return pool
}

func hasAll(caps map[string]bool, required []string) bool {
	for _, name := range required {
		if !caps[name] {
			return false
		}
	}
	return true
}

// orderFor returns the tier ladder: configured tier first, then the adjacent
// tiers outward (upgrade first, then downgrade), so an unfillable tier
// degrades to the nearest capable one instead of declining outright.
func (e *Engine) orderFor(tier string) []string {
	all := []string{TierSimple, TierMedium, TierComplex, TierVeryComplex}
	idx := tierIndex(tier)
	ladder := []string{all[idx]}
	for i := idx + 1; i < len(all); i++ {
		ladder = append(ladder, all[i])
	}
	for i := idx - 1; i >= 0; i-- {
		ladder = append(ladder, all[i])
	}
	var models []string
	for _, t := range ladder {
		models = append(models, e.tiers.ForTier(t)...)
	}
	return dedupe(models)
}

func tierIndex(tier string) int {
	for i, t := range []string{TierSimple, TierMedium, TierComplex, TierVeryComplex} {
		if t == tier {
			return i
		}
	}
	return 0
}

func tierAt(i int) string {
	switch i {
	case 0:
		return TierSimple
	case 1:
		return TierMedium
	case 2:
		return TierComplex
	default:
		return TierVeryComplex
	}
}

func matchPreferred(pool []ext.RouteCandidate, preferred []string) string {
	for _, want := range preferred {
		for _, c := range pool {
			if c.Model == want || c.Qualified == want {
				return c.Qualified
			}
		}
	}
	return ""
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
