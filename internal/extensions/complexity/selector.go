package complexity

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/enterpilot/gomodel/ext"
)

// Selector is the complexity-aware ext.RouteSelector: it scores the request
// summary, filters candidates by capability demands, and answers with the
// preferred tier's model that is currently in the viable pool. It declines
// (ok=false → core's weighted round robin) when the request carries no
// summary, the engine has no tiers configured, or nothing matches.
type Selector struct {
	mu     sync.RWMutex
	engine *Engine
	source string // restrict to one virtual model name; "" = any adaptive redirect
}

// NewSelector builds a selector from configuration.
func NewSelector(cfg Config) *Selector {
	engine := NewEngine(cfg)
	return &Selector{engine: engine, source: strings.TrimSpace(cfg.VirtualModel)}
}

// Name identifies the selector in logs.
func (s *Selector) Name() string { return "complexity" }

// Select answers the route request.
func (s *Selector) Select(req ext.RouteRequest) (string, bool) {
	s.mu.RLock()
	engine, source := s.engine, s.source
	s.mu.RUnlock()
	if source != "" && req.Source != source {
		return "", false
	}
	if req.Content == nil {
		return "", false
	}
	if picked := engine.Select(req); picked != "" {
		return picked, true
	}
	return "", false
}

// OnAttemptStart / OnAttemptEnd keep trial-run observations: selections
// before any outcome signal, so both are no-ops for now.
func (s *Selector) OnAttemptStart(ext.RouteTarget) {}
func (s *Selector) OnAttemptEnd(ext.RouteOutcome)  {}

// SetThresholds hot-swaps the complexity band boundaries (dashboard runtime
// setting / reload). Malformed input is rejected; the live engine keeps
// serving.
func (s *Selector) SetThresholds(raw string) error {
	parts := strings.Split(raw, ",")
	if len(parts) != 3 {
		return fmt.Errorf("thresholds expect \"simple_medium,medium_complex,complex_very\", got %q", raw)
	}
	values := make([]float64, 3)
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return fmt.Errorf("threshold %d: %w", i, err)
		}
		if v <= 0 || v >= 1 {
			return fmt.Errorf("threshold %d = %v, want (0,1)", i, v)
		}
		values[i] = v
	}
	if !(values[0] < values[1] && values[1] < values[2]) {
		return fmt.Errorf("thresholds must ascend, got %v", values)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.engine.thresholds = Thresholds{SimpleMedium: values[0], MediumComplex: values[1], ComplexVery: values[2]}
	return nil
}

// Thresholds returns the live band boundaries (for the setting descriptor).
func (s *Selector) Thresholds() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t := s.engine.thresholds
	return strconv.FormatFloat(t.SimpleMedium, 'g', -1, 64) + "," +
		strconv.FormatFloat(t.MediumComplex, 'g', -1, 64) + "," +
		strconv.FormatFloat(t.ComplexVery, 'g', -1, 64)
}

// ThresholdSetting exposes the live thresholds as a dashboard-editable
// runtime setting. It is a thin mutator over a Selector.
type ThresholdSetting struct{ selector *Selector }

// NewThresholdSetting wraps a selector for ext.RegisterSetting.
func NewThresholdSetting(s *Selector) *ThresholdSetting { return &ThresholdSetting{selector: s} }

func (t *ThresholdSetting) Descriptor() ext.SettingDescriptor {
	return ext.SettingDescriptor{
		Key:         "complexity_routing.thresholds",
		Label:       "Complexity thresholds",
		Description: "Band boundaries as \"simple_medium,medium_complex,complex_very\" — values in (0,1), ascending. Trial-run estimates; recalibrate from audit logs.",
		Value:       t.selector.Thresholds(),
		// A free-form triple cannot enumerate its Options, and core
		// rejects unlocked settings without the full option list — so
		// the knob is locked to config/env until a validated editor
		// lands. Apply still works for programmatic callers.
		Locked: true,
	}
}

func (t *ThresholdSetting) Apply(value string) error {
	return t.selector.SetThresholds(value)
}
