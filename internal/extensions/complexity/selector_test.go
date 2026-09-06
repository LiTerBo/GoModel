package complexity

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/ext"
)

func selectorPool() []ext.RouteCandidate {
	return []ext.RouteCandidate{
		{Qualified: "p/lite-model", Model: "lite-model"},
		{Qualified: "p/flash-model", Model: "flash-model", Capabilities: map[string]bool{"function_calling": true}},
	}
}

func selectorConfig() Config {
	return Config{Tiers: TierMap{
		Simple:  []string{"lite-model"},
		Medium:  []string{"flash-model"},
		Complex: []string{"flash-model"},
	}}
}

func TestSelectorNeedsContent(t *testing.T) {
	t.Parallel()
	s := NewSelector(selectorConfig())
	if _, ok := s.Select(ext.RouteRequest{Candidates: selectorPool()}); ok {
		t.Fatal("selector must decline when the request carries no summary")
	}
}

func TestSelectorAnswersByTier(t *testing.T) {
	t.Parallel()
	s := NewSelector(selectorConfig())
	simple := &ext.RouteContent{MessageCount: 1, CurrentChars: 10}
	got, ok := s.Select(ext.RouteRequest{Candidates: selectorPool(), Content: simple})
	if !ok || got != "p/lite-model" {
		t.Fatalf("Select() = %q/%v, want p/lite-model", got, ok)
	}
	heavy := &ext.RouteContent{MessageCount: 30, UserTurns: 15, CurrentChars: 6000, AvgUserChars: 900, ContextChars: 80000, ToolDefs: 8, HasCode: true}
	got, ok = s.Select(ext.RouteRequest{Candidates: selectorPool(), Content: heavy})
	if !ok || got != "p/flash-model" {
		t.Fatalf("Select() = %q/%v, want p/flash-model for complex request", got, ok)
	}
}

func TestSelectorSourceRestriction(t *testing.T) {
	t.Parallel()
	cfg := selectorConfig()
	cfg.VirtualModel = "smart"
	s := NewSelector(cfg)
	content := &ext.RouteContent{MessageCount: 1, CurrentChars: 10}
	if _, ok := s.Select(ext.RouteRequest{Source: "other-vm", Candidates: selectorPool(), Content: content}); ok {
		t.Fatal("selector must decline requests for a different virtual model")
	}
	if _, ok := s.Select(ext.RouteRequest{Source: "smart", Candidates: selectorPool(), Content: content}); !ok {
		t.Fatal("selector must answer its configured source")
	}
}

func TestSelectorUnconfiguredDeclines(t *testing.T) {
	t.Parallel()
	s := NewSelector(Config{})
	content := &ext.RouteContent{MessageCount: 1, CurrentChars: 10}
	if _, ok := s.Select(ext.RouteRequest{Candidates: selectorPool(), Content: content}); ok {
		t.Fatal("no tiers configured must decline to round robin")
	}
}

func TestSetThresholdsValidation(t *testing.T) {
	t.Parallel()
	s := NewSelector(selectorConfig())
	for _, bad := range []string{"", "0.5", "0.5,0.7", "a,b,c", "0.9,0.5,0.7", "0,0.5,0.9", "0.2,0.5,1.5"} {
		if err := s.SetThresholds(bad); err == nil {
			t.Fatalf("SetThresholds(%q) accepted, want rejection", bad)
		}
	}
	if err := s.SetThresholds("0.20, 0.50 ,0.80"); err != nil {
		t.Fatalf("SetThresholds() error = %v", err)
	}
	if got := s.Thresholds(); got != "0.2,0.5,0.8" {
		t.Fatalf("Thresholds() = %q, want 0.2,0.5,0.8", got)
	}
}

func TestThresholdSettingDescriptor(t *testing.T) {
	t.Parallel()
	setting := NewThresholdSetting(NewSelector(selectorConfig()))
	desc := setting.Descriptor()
	if desc.Key != "complexity_routing.thresholds" || !desc.Locked {
		t.Fatalf("descriptor = %+v", desc)
	}
	if err := setting.Apply("0.3,0.6,0.9"); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if err := setting.Apply("nope"); err == nil {
		t.Fatal("Apply(nope) accepted")
	}
}

func TestSelectorImplementsInterface(t *testing.T) {
	var _ ext.RouteSelector = NewSelector(Config{})
	var _ = context.Background // keep context import meaningful for async tests
}
