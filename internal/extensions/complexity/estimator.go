package complexity

import (
	"math"
	"regexp"
	"strings"

	"github.com/enterpilot/gomodel/ext"
)

// Complexity level band names come from Tier*.

// Feature weights of the complexity score. Calibrated from model-router's
// 88-session K-Means baseline, to be re-fit from GoModel audit samples after
// the trial month (D-Q4). They sum to 1.
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
// bound and ~0 at the lower (log1p scale). Derived from the same corpus as
// the weights; re-fit alongside them.
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

var (
	codeFenceRe = regexp.MustCompile("(?s)```[^\\n]+")
	numberedRe  = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+`)
	bulletRe    = regexp.MustCompile("(?m)^\\s*[-*•]\\s+")
)

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

// DetectHasCode reports whether a content block contains a fenced code block.
func DetectHasCode(text string) bool { return codeFenceRe.MatchString(text) }

// DetectHasMultiStep reports numbered (≥3) or bulleted (≥4) step lists.
func DetectHasMultiStep(text string) bool {
	return len(numberedRe.FindAllString(text, 3)) >= 3 ||
		len(bulletRe.FindAllString(text, 4)) >= 4
}

// Summarize builds the lightweight RouteContent for a chat-style request:
// counts and lengths only, message text never leaves the caller.
func Summarize(messages int, userTurns, contextChars, currentChars, avgUserChars, toolDefs int, hasCode, hasMultiStep bool) *ext.RouteContent {
	return &ext.RouteContent{
		MessageCount: messages, UserTurns: userTurns,
		ContextChars: contextChars, CurrentChars: currentChars,
		AvgUserChars: avgUserChars, ToolDefs: toolDefs,
		HasCode: hasCode, HasMultiStep: hasMultiStep,
	}
}

// HasFencedCode scans joined text for a code fence (kept for gateway-side
// convenience).
func HasFencedCode(text string) bool { return strings.Contains(text, "```") }
