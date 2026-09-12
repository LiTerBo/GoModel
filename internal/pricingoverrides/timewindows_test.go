package pricingoverrides

import (
	"reflect"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

func coreFloat(v float64) *float64 { return &v }

func TestValidatePricing_TimeWindows(t *testing.T) {
	validWindow := func() Pricing {
		return Pricing{
			TimeWindows: []core.ModelPricingTimeWindow{{
				Label:     "off_peak",
				UTCRanges: []core.ModelPricingUTCRange{{Start: "10:00", End: "24:00"}},
				Pricing:   core.ModelPricingTimeWindowRates{InputPerMtok: coreFloat(0.22)},
			}},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*Pricing)
		wantErr string
	}{
		{"valid window", func(p *Pricing) {}, ""},
		{"valid window with days", func(p *Pricing) {
			p.TimeWindows[0].UTCRanges[0].Days = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
		}, ""},
		{"valid window all-day weekend", func(p *Pricing) {
			p.TimeWindows[0].UTCRanges[0].Days = []string{"sat", "sun"}
			p.TimeWindows[0].UTCRanges[0].Start = "00:00"
			p.TimeWindows[0].UTCRanges[0].End = "24:00"
		}, ""},
		{"empty label", func(p *Pricing) { p.TimeWindows[0].Label = "" }, "label"},
		{"whitespace label", func(p *Pricing) { p.TimeWindows[0].Label = "   " }, "label"},
		{"no utc ranges", func(p *Pricing) { p.TimeWindows[0].UTCRanges = nil }, "utc_ranges"},
		{"bad clock start", func(p *Pricing) { p.TimeWindows[0].UTCRanges[0].Start = "25:00" }, "start"},
		{"bad clock end", func(p *Pricing) { p.TimeWindows[0].UTCRanges[0].End = "10:60" }, "end"},
		{"non-numeric clock", func(p *Pricing) { p.TimeWindows[0].UTCRanges[0].Start = "abc" }, "start"},
		{"bad day", func(p *Pricing) { p.TimeWindows[0].UTCRanges[0].Days = []string{"tues"} }, "day"},
		{"duplicate day", func(p *Pricing) { p.TimeWindows[0].UTCRanges[0].Days = []string{"mon", "mon"} }, "day"},
		{"negative window rate", func(p *Pricing) {
			p.TimeWindows[0].Pricing.OutputPerMtok = coreFloat(-1)
		}, "must be greater than or equal to 0"},
		{"empty window rates", func(p *Pricing) { p.TimeWindows[0].Pricing = core.ModelPricingTimeWindowRates{} }, "at least one rate"},
		{"multiple valid windows", func(p *Pricing) {
			p.TimeWindows = append(p.TimeWindows, core.ModelPricingTimeWindow{
				Label:     "deep_off_peak",
				UTCRanges: []core.ModelPricingUTCRange{{Start: "00:00", End: "01:00"}},
				Pricing:   core.ModelPricingTimeWindowRates{InputPerMtok: coreFloat(0.11)},
			})
		}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := validWindow()
			tt.mutate(&pricing)
			err := validatePricing(pricing)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validatePricing() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validatePricing() error = nil, want containing %q", tt.wantErr)
			}
			if !containStr(err.Error(), tt.wantErr) {
				t.Fatalf("validatePricing() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPricingClone_TimeWindows(t *testing.T) {
	in := Pricing{
		TimeWindows: []core.ModelPricingTimeWindow{{
			Label:     "off_peak",
			UTCRanges: []core.ModelPricingUTCRange{{Days: []string{"mon"}, Start: "10:00", End: "24:00"}},
			Pricing:   core.ModelPricingTimeWindowRates{InputPerMtok: coreFloat(0.22), OutputPerMtok: coreFloat(0.66)},
		}},
	}
	out := clonePricing(in)

	if !reflect.DeepEqual(out, in) {
		t.Fatalf("clonePricing() = %+v, want %+v", out, in)
	}

	// Mutating the clone must not affect the original: ranges, days, rates.
	out.TimeWindows[0].Label = "mutated"
	out.TimeWindows[0].UTCRanges[0].Start = "00:00"
	out.TimeWindows[0].UTCRanges[0].Days[0] = "tue"
	*out.TimeWindows[0].Pricing.InputPerMtok = 999
	if in.TimeWindows[0].Label != "off_peak" ||
		in.TimeWindows[0].UTCRanges[0].Start != "10:00" ||
		in.TimeWindows[0].UTCRanges[0].Days[0] != "mon" ||
		*in.TimeWindows[0].Pricing.InputPerMtok != 0.22 {
		t.Fatalf("clonePricing() did not deep-copy, original mutated: %+v", in)
	}
}

func TestPricingEmpty_TimeWindows(t *testing.T) {
	if pricingEmpty(Pricing{}) != true {
		t.Fatal("pricingEmpty(empty) = false, want true")
	}
	if pricingEmpty(Pricing{TimeWindows: []core.ModelPricingTimeWindow{{
		Label:     "off_peak",
		UTCRanges: []core.ModelPricingUTCRange{{Start: "00:00", End: "24:00"}},
		Pricing:   core.ModelPricingTimeWindowRates{InputPerMtok: coreFloat(0.1)},
	}}}) {
		t.Fatal("pricingEmpty(window) = true, want false")
	}
}

func TestPricingToCore_TimeWindows(t *testing.T) {
	in := Pricing{
		InputPerMtok: coreFloat(0.44),
		TimeWindows: []core.ModelPricingTimeWindow{{
			Label:     "off_peak",
			UTCRanges: []core.ModelPricingUTCRange{{Days: []string{"sat", "sun"}, Start: "00:00", End: "24:00"}},
			Pricing:   core.ModelPricingTimeWindowRates{InputPerMtok: coreFloat(0.22)},
		}},
	}
	out := pricingToCore(in)
	if len(out.TimeWindows) != 1 {
		t.Fatalf("pricingToCore() TimeWindows = %d, want 1", len(out.TimeWindows))
	}
	w := out.TimeWindows[0]
	if w.Label != "off_peak" || len(w.UTCRanges) != 1 || w.UTCRanges[0].Start != "00:00" ||
		w.UTCRanges[0].End != "24:00" || w.UTCRanges[0].Days[0] != "sat" || *w.Pricing.InputPerMtok != 0.22 {
		t.Fatalf("pricingToCore() window = %+v", w)
	}
}

func TestMergePricing_TimeWindows(t *testing.T) {
	base := &core.ModelPricing{
		InputPerMtok: coreFloat(1.0),
		OutputPerMtok: coreFloat(3.0),
		TimeWindows: []core.ModelPricingTimeWindow{{
			Label:     "catalog_off_peak",
			UTCRanges: []core.ModelPricingUTCRange{{Start: "10:00", End: "24:00"}},
			Pricing:   core.ModelPricingTimeWindowRates{InputPerMtok: coreFloat(0.5), OutputPerMtok: coreFloat(2.0)},
		}},
	}

	t.Run("override supplies windows replaces catalog windows", func(t *testing.T) {
		out := mergePricing(base, Pricing{
			TimeWindows: []core.ModelPricingTimeWindow{{
				Label:     "op_peak",
				UTCRanges: []core.ModelPricingUTCRange{{Start: "00:00", End: "01:00"}},
				Pricing:   core.ModelPricingTimeWindowRates{OutputPerMtok: coreFloat(0.1)},
			}},
		})
		if len(out.TimeWindows) != 1 || out.TimeWindows[0].Label != "op_peak" {
			t.Fatalf("TimeWindows = %+v, want override window", out.TimeWindows)
		}
	})

	t.Run("override scalar drops matching window rates but keeps others", func(t *testing.T) {
		out := mergePricing(base, Pricing{InputPerMtok: coreFloat(1.5)})
		if len(out.TimeWindows) != 1 {
			t.Fatalf("TimeWindows = %d, want 1 kept", len(out.TimeWindows))
		}
		w := out.TimeWindows[0]
		if w.Pricing.InputPerMtok != nil {
			t.Fatalf("window InputPerMtok = %v, want nil (dropped)", w.Pricing.InputPerMtok)
		}
		if w.Pricing.OutputPerMtok == nil || *w.Pricing.OutputPerMtok != 2.0 {
			t.Fatalf("window OutputPerMtok = %v, want kept 2.0", w.Pricing.OutputPerMtok)
		}
	})

	t.Run("catalog windows dropped when overridden scalar empties them", func(t *testing.T) {
		onlyInput := &core.ModelPricing{
			InputPerMtok: coreFloat(1.0),
			TimeWindows: []core.ModelPricingTimeWindow{{
				Label:     "catalog_off_peak",
				UTCRanges: []core.ModelPricingUTCRange{{Start: "10:00", End: "24:00"}},
				Pricing:   core.ModelPricingTimeWindowRates{InputPerMtok: coreFloat(0.5)},
			}},
		}
		out := mergePricing(onlyInput, Pricing{InputPerMtok: coreFloat(2.0)})
		if len(out.TimeWindows) != 0 {
			t.Fatalf("TimeWindows = %d, want 0 (all dropped)", len(out.TimeWindows))
		}
	})

	t.Run("no override windows keeps base windows", func(t *testing.T) {
		out := mergePricing(base, Pricing{OutputPerMtok: coreFloat(2.0)})
		if len(out.TimeWindows) != 1 || out.TimeWindows[0].Label != "catalog_off_peak" {
			t.Fatalf("TimeWindows = %+v, want catalog window kept", out.TimeWindows)
		}
		if out.TimeWindows[0].Pricing.InputPerMtok == nil || *out.TimeWindows[0].Pricing.InputPerMtok != 0.5 {
			t.Fatalf("window InputPerMtok = %v, want kept 0.5", out.TimeWindows[0].Pricing.InputPerMtok)
		}
	})
}

func containStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
