package complexity

import (
	"testing"

	"github.com/enterpilot/gomodel/ext"
)

// F-2: operator-provided thresholds change banding without code changes —
// the same request lands in a different tier after hot-updating boundaries.
func TestThresholdsFromConfigDriveSelection(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Enabled: true,
		Thresholds: Thresholds{
			SimpleMedium: 0.05, MediumComplex: 0.10, ComplexVery: 0.20,
		},
		Tiers: TierMap{
			Simple:      []string{"lite-model"},
			Medium:      []string{"flash-model"},
			Complex:     []string{"pro-model"},
			VeryComplex: []string{"max-model"},
		},
	}
	s := NewSelector(cfg)
	content := &ext.RouteContent{MessageCount: 3, CurrentChars: 200} // score ≈ 0.22
	// Aggressive bands put the same request in very_complex; the ladder
	// upgrades first (max/pro not in pool) then downgrades to flash.
	got, ok := s.Select(ext.RouteRequest{Candidates: selectorPool(), Content: content})
	if !ok || got != "p/flash-model" {
		t.Fatalf("aggressive thresholds: Select() = %q/%v, want p/flash-model", got, ok)
	}
	// With human default bands the same request is clearly simple.
	s2 := NewSelector(Config{Tiers: cfg.Tiers})
	got, _ = s2.Select(ext.RouteRequest{Candidates: selectorPool(), Content: content})
	if got != "p/lite-model" {
		t.Fatalf("default bands: Select() = %q, want p/lite-model", got)
	}
}

// F-1 (selector side): a configured virtual_model restricts which redirect
// the selector steers; any adaptive redirect answers when unset.
func TestVirtualModelScoping(t *testing.T) {
	t.Parallel()
	content := &ext.RouteContent{MessageCount: 1, CurrentChars: 10}
	scoped := NewSelector(Config{VirtualModel: "smart", Tiers: TierMap{Simple: []string{"lite-model"}}})
	if _, ok := scoped.Select(ext.RouteRequest{Source: "smart", Candidates: selectorPool(), Content: content}); !ok {
		t.Fatal("scoped selector must answer smart")
	}
	if _, ok := scoped.Select(ext.RouteRequest{Source: "other", Candidates: selectorPool(), Content: content}); ok {
		t.Fatal("scoped selector must ignore other sources")
	}
	any := NewSelector(Config{Tiers: TierMap{Simple: []string{"lite-model"}}})
	for _, src := range []string{"smart", "other", ""} {
		if _, ok := any.Select(ext.RouteRequest{Source: src, Candidates: selectorPool(), Content: content}); !ok {
			t.Fatalf("unscoped selector must answer source %q", src)
		}
	}
}
