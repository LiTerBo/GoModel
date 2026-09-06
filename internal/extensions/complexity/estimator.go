package complexity

import (
	"math"

	"github.com/enterpilot/gomodel/ext"
	"github.com/enterpilot/gomodel/internal/core"
)

// Feature weights of the complexity score. These are the trial-run human
// estimates (Q4): the relative shape follows model-router's published
// weighting (current message dominates, context least), absolute values get
// re-fit from GoModel's audit-log traffic after the one-month trial. They
// sum to 1.
var weights = struct {
	currentLength float64
	avgUserLength float64
	messageCount  float64
	userTurns     float64
	toolDefs      float64
	hasCode       float64
	hasMultiStep  float64
	contextLength float64
}{
	currentLength: 0.30,
	avgUserLength: 0.15,
	messageCount:  0.12,
	userTurns:     0.10,
	toolDefs:      0.08,
	hasCode:       0.10,
	hasMultiStep:  0.10,
	contextLength: 0.05,
}

// Log-scale reference points: the raw value mapped to score 1.0 at the upper
// bound and ~0 at the lower (log1p scale). Trial-run estimates; re-fit with
// the weights.
var refPoints = struct {
	currentLength, avgUserLength, messageCount, userTurns, toolDefs, contextLength float64
}{
	currentLength: 8000,
	avgUserLength: 1200,
	messageCount:  60,
	userTurns:     25,
	toolDefs:      12,
	contextLength: 120000,
}

// Score maps a request summary to a complexity score in [0,1]. A nil or
// zero-value summary scores 0 (the most trivial band). Pure function: safe
// for concurrent use.
func Score(content *ext.RouteContent) float64 {
	if content == nil {
		return 0
	}
	total := weights.currentLength*logScaled(float64(content.CurrentChars), refPoints.currentLength) +
		weights.avgUserLength*logScaled(float64(content.AvgUserChars), refPoints.avgUserLength) +
		weights.messageCount*logScaled(float64(content.MessageCount), refPoints.messageCount) +
		weights.userTurns*logScaled(float64(content.UserTurns), refPoints.userTurns) +
		weights.toolDefs*logScaled(float64(content.ToolDefs), refPoints.toolDefs) +
		weights.contextLength*logScaled(float64(content.ContextChars), refPoints.contextLength)
	if content.HasCode {
		total += weights.hasCode
	}
	if content.HasMultiStep {
		total += weights.hasMultiStep
	}
	return math.Min(1, math.Max(0, total))
}

// logScaled maps a non-negative raw count to [0,1] on a log1p curve that
// reaches 1 at ref.
func logScaled(raw, ref float64) float64 {
	if raw <= 0 || ref <= 1 {
		return 0
	}
	return math.Min(1, math.Log1p(raw)/math.Log1p(ref))
}

// Classify maps a score to a tier using the configured thresholds.
func Classify(score float64, t Thresholds) string {
	switch {
	case score < t.SimpleMedium:
		return TierSimple
	case score < t.MediumComplex:
		return TierMedium
	case score < t.ComplexVery:
		return TierComplex
	default:
		return TierVeryComplex
	}
}

// DetectHasCode / DetectHasMultiStep delegate to the core feature detectors —
// the single source of truth shared with the gateway-side summary builder.
var (
	DetectHasCode      = core.DetectCodeFence
	DetectHasMultiStep = core.DetectMultiStepList
)
