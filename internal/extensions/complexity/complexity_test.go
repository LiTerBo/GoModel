package complexity

import (
	"testing"

	"github.com/enterpilot/gomodel/ext"
)

// ─── Stage B: estimator ─────────────────────────────────────────────────

func TestDetectHasCode(t *testing.T) {
	t.Parallel()
	if !DetectHasCode("here:\n```go\nfunc main() {}\n```") {
		t.Fatal("fenced go block not detected")
	}
	if DetectHasCode("no fence here, just inline `code`") {
		t.Fatal("inline code must not count as a fenced block")
	}
}

func TestDetectHasMultiStep(t *testing.T) {
	t.Parallel()
	numbered := "1. first\n2. second\n3. third\n"
	if !DetectHasMultiStep(numbered) {
		t.Fatal("3-item numbered list not detected")
	}
	bullets := "- a\n- b\n- c\n- d\n"
	if !DetectHasMultiStep(bullets) {
		t.Fatal("4-item bullet list not detected")
	}
	if DetectHasMultiStep("- a\n- b\n- c\n") {
		t.Fatal("3-item bullet list should not trigger multi-step")
	}
	if DetectHasMultiStep("prose only") {
		t.Fatal("prose must not trigger")
	}
}

func TestScoreNilContentIsZero(t *testing.T) {
	if s := Score(nil); s != 0 {
		t.Fatalf("Score(nil) = %v, want 0", s)
	}
}

func TestScoreMonotonicInComplexity(t *testing.T) {
	t.Parallel()
	trivial := Score(&ext.RouteContent{MessageCount: 1, UserTurns: 1, CurrentChars: 20})
	code := Score(&ext.RouteContent{MessageCount: 1, UserTurns: 1, CurrentChars: 4000, HasCode: true})
	agent := Score(&ext.RouteContent{
		MessageCount: 40, UserTurns: 20, ContextChars: 90000,
		CurrentChars: 6000, AvgUserChars: 900, ToolDefs: 9,
		HasCode: true, HasMultiStep: true,
	})
	if !(trivial < code && code < agent) {
		t.Fatalf("expected trivial(%v) < code(%v) < agent(%v)", trivial, code, agent)
	}
	if agent > 1 {
		t.Fatalf("score %v exceeds 1", agent)
	}
}

func TestScoreBounded(t *testing.T) {
	t.Parallel()
	huge := Score(&ext.RouteContent{
		MessageCount: 1_000_000, UserTurns: 1_000_000, ContextChars: 1_000_000_000,
		CurrentChars: 1_000_000, AvgUserChars: 1_000_000, ToolDefs: 1_000_000,
		HasCode: true, HasMultiStep: true,
	})
	if huge > 1 {
		t.Fatalf("score %v exceeds 1", huge)
	}
}

func TestClassifyBands(t *testing.T) {
	t.Parallel()
	th := DefaultThresholds()
	cases := []struct {
		score float64
		want  string
	}{
		{0.0, TierSimple},
		{th.SimpleMedium - 0.01, TierSimple},
		{th.SimpleMedium, TierMedium},     // boundary: lower band is exclusive
		{th.MediumComplex, TierComplex},   //
		{th.ComplexVery, TierVeryComplex}, //
	}
	for _, tc := range cases {
		if got := Classify(tc.score, th); got != tc.want {
			t.Fatalf("Classify(%v) = %s, want %s", tc.score, got, tc.want)
		}
	}
}

// ─── Stage C: decision engine + capability filtering ────────────────────

func cand(qualified, model string, caps map[string]bool) ext.RouteCandidate {
	return ext.RouteCandidate{Qualified: qualified, Model: model, Capabilities: caps}
}

func testConfig() Config {
	return Config{Tiers: TierMap{
		Simple:      []string{"lite-model"},
		Medium:      []string{"flash-model"},
		Complex:     []string{"pro-model"},
		VeryComplex: []string{"max-model", "pro-model"},
	}}
}

func pool() []ext.RouteCandidate {
	return []ext.RouteCandidate{
		cand("p/lite-model", "lite-model", nil),
		cand("p/flash-model", "flash-model", map[string]bool{"function_calling": true}),
		cand("p/pro-model", "pro-model", map[string]bool{"vision": true, "function_calling": true}),
		cand("p/max-model", "max-model", map[string]bool{"vision": true}),
	}
}

func TestEngineSelectByTier(t *testing.T) {
	t.Parallel()
	e := NewEngine(testConfig())
	for _, tc := range []struct {
		content *ext.RouteContent
		want    string
	}{
		{&ext.RouteContent{MessageCount: 1, CurrentChars: 10}, "p/lite-model"},
		{&ext.RouteContent{MessageCount: 8, CurrentChars: 3000, AvgUserChars: 800, HasCode: true}, "p/flash-model"},
		{&ext.RouteContent{MessageCount: 20, UserTurns: 8, ContextChars: 60000, CurrentChars: 3000, AvgUserChars: 600, ToolDefs: 5, HasCode: true}, "p/pro-model"},
		{&ext.RouteContent{MessageCount: 40, UserTurns: 20, ContextChars: 90000, CurrentChars: 6000, AvgUserChars: 900, ToolDefs: 9, HasCode: true, HasMultiStep: true}, "p/max-model"},
	} {
		got := e.Select(ext.RouteRequest{Candidates: pool(), Content: tc.content})
		if got != tc.want {
			t.Fatalf("tier %s: Select() = %q, want %q", Classify(Score(tc.content), DefaultThresholds()), got, tc.want)
		}
	}
}

func TestEngineVisionHardFilter(t *testing.T) {
	t.Parallel()
	e := NewEngine(testConfig())
	// Simple request would go to lite (no vision); a vision-demanding
	// request must skip lite/flash even though the tier says simple.
	got := e.Select(ext.RouteRequest{
		Content:              &ext.RouteContent{MessageCount: 1, CurrentChars: 10},
		RequiredCapabilities: []string{"vision"},
		Candidates:           pool(),
	})
	if got != "p/pro-model" && got != "p/max-model" {
		t.Fatalf("vision request routed to %q, want a vision-capable candidate", got)
	}
}

func TestEngineVisionHardFilterEmptiesPool(t *testing.T) {
	t.Parallel()
	e := NewEngine(testConfig())
	// No candidate has vision → decline entirely (core round-robins; the
	// request is not routable intelligently anyway).
	got := e.Select(ext.RouteRequest{
		RequiredCapabilities: []string{"vision"},
		Candidates: []ext.RouteCandidate{
			cand("p/lite-model", "lite-model", nil),
			cand("p/flash-model", "flash-model", nil),
		},
	})
	if got != "" {
		t.Fatalf("Select() = %q, want decline when no candidate sees images", got)
	}
}

func TestEngineFunctionCallingSoftPreference(t *testing.T) {
	t.Parallel()
	e := NewEngine(testConfig())
	// Simple tier prefers lite (unlabeled for tools); soft preference must
	// NOT block routing to it when a tool-calling request arrives —
	// function_calling is a preference, not a hard gate.
	got := e.Select(ext.RouteRequest{
		Content:              &ext.RouteContent{MessageCount: 1, CurrentChars: 10},
		RequiredCapabilities: []string{"function_calling"},
		Candidates:           pool(),
	})
	if got != "p/flash-model" {
		t.Fatalf("Select() = %q, want preferred flash (capable) over lite (unknown)", got)
	}
	// With no capable candidate at all, the simple pick still routes (soft).
	got = e.Select(ext.RouteRequest{
		Content:              &ext.RouteContent{MessageCount: 1, CurrentChars: 10},
		RequiredCapabilities: []string{"function_calling"},
		Candidates: []ext.RouteCandidate{
			cand("p/lite-model", "lite-model", nil),
		},
	})
	if got != "p/lite-model" {
		t.Fatalf("Select() = %q, want soft preference to route to the only candidate", got)
	}
}

func TestEngineTierFallbackLadder(t *testing.T) {
	t.Parallel()
	e := NewEngine(testConfig())
	// Complex tier wants pro/max but neither is in the pool: ladder may
	// downgrade to flash.
	got := e.Select(ext.RouteRequest{
		Content: &ext.RouteContent{MessageCount: 30, UserTurns: 15, CurrentChars: 7000, AvgUserChars: 1100, ContextChars: 100000, ToolDefs: 10, HasCode: true, HasMultiStep: true},
		Candidates: []ext.RouteCandidate{
			cand("p/flash-model", "flash-model", nil),
		},
	})
	if got != "p/flash-model" {
		t.Fatalf("Select() = %q, want downgrade to flash via ladder", got)
	}
}

func TestEngineNoTiersConfiguredDeclines(t *testing.T) {
	t.Parallel()
	e := NewEngine(Config{})
	if got := e.Select(ext.RouteRequest{Candidates: pool()}); got != "" {
		t.Fatalf("unconfigured engine must decline, got %q", got)
	}
}

func TestEngineSessionUnaffected(t *testing.T) {
	// The engine is stateless: same request, same answer.
	e := NewEngine(testConfig())
	req := ext.RouteRequest{Candidates: pool(), Content: &ext.RouteContent{MessageCount: 2, CurrentChars: 50}}
	if e.Select(req) != e.Select(req) {
		t.Fatal("engine is not deterministic")
	}
}
