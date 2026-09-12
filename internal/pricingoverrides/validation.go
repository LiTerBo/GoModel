package pricingoverrides

import (
	"errors"
	"strings"

	"github.com/enterpilot/gomodel/internal/core"
)

type pricingField struct {
	name  string
	value *float64
}

func validatePricing(p Pricing) error {
	for _, field := range pricingScalarFields(p) {
		if field.value != nil && *field.value < 0 {
			return newValidationError("pricing."+field.name+" must be greater than or equal to 0", nil)
		}
	}
	for i, tier := range p.Tiers {
		if tier.UpToTokens != nil && tier.UpToMtok != nil {
			return newValidationError("pricing.tiers must set only one threshold: up_to_tokens or up_to_mtok", nil)
		}
		if tier.UpToTokens != nil && *tier.UpToTokens <= 0 {
			return newValidationError("pricing.tiers up_to_tokens must be greater than 0", nil)
		}
		if tier.UpToMtok != nil && *tier.UpToMtok <= 0 {
			return newValidationError("pricing.tiers up_to_mtok must be greater than 0", nil)
		}
		if tier.UpToTokens == nil && tier.UpToMtok == nil {
			return newValidationError("pricing.tiers threshold is required", nil)
		}
		if tier.InputPerMtok == nil && tier.OutputPerMtok == nil {
			return newValidationError("pricing.tiers rate is required", nil)
		}
		for _, field := range pricingTierFields(tier) {
			if field.value != nil && *field.value < 0 {
				return newValidationError("pricing.tiers "+field.name+" must be greater than or equal to 0", nil)
			}
		}
		if i > 0 && tierLimit(tier) <= tierLimit(p.Tiers[i-1]) {
			return newValidationError("pricing.tiers thresholds must be increasing", nil)
		}
	}
	if err := validatePricingTimeWindows(p.TimeWindows); err != nil {
		return err
	}
	return nil
}

// ValidatePricing validates pricing including time windows for external verification.
func ValidatePricing(p Pricing) error {
	return validatePricing(p)
}

func pricingScalarFields(p Pricing) []pricingField {
	return []pricingField{
		{"input_per_mtok", p.InputPerMtok},
		{"output_per_mtok", p.OutputPerMtok},
		{"cached_input_per_mtok", p.CachedInputPerMtok},
		{"cache_write_per_mtok", p.CacheWritePerMtok},
		{"reasoning_output_per_mtok", p.ReasoningOutputPerMtok},
		{"batch_input_per_mtok", p.BatchInputPerMtok},
		{"batch_output_per_mtok", p.BatchOutputPerMtok},
		{"audio_input_per_mtok", p.AudioInputPerMtok},
		{"audio_output_per_mtok", p.AudioOutputPerMtok},
		{"output_image_per_mtok", p.OutputImagePerMtok},
		{"per_image", p.PerImage},
		{"input_per_image", p.InputPerImage},
		{"per_second_input", p.PerSecondInput},
		{"per_second_output", p.PerSecondOutput},
		{"per_character_input", p.PerCharacterInput},
		{"per_request", p.PerRequest},
		{"per_page", p.PerPage},
	}
}

func pricingTierFields(t PricingTier) []pricingField {
	return []pricingField{
		{"input_per_mtok", t.InputPerMtok},
		{"output_per_mtok", t.OutputPerMtok},
	}
}

func validatePricingTimeWindows(windows []core.ModelPricingTimeWindow) error {
	for wi, window := range windows {
		prefix := "pricing.time_windows[" + itoa(wi) + "]"
		if label := strings.TrimSpace(window.Label); label == "" {
			return newValidationError(prefix+".label is required", nil)
		}
		if len(window.UTCRanges) == 0 {
			return newValidationError(prefix+".utc_ranges must not be empty", nil)
		}
		for ri, rangeW := range window.UTCRanges {
			rprefix := prefix + ".utc_ranges[" + itoa(ri) + "]"
			if err := validateClock(rangeW.Start); err != nil {
				return newValidationError(rprefix+".start must be a valid HH:MM time: "+err.Error(), nil)
			}
			if err := validateClock(rangeW.End); err != nil {
				return newValidationError(rprefix+".end must be a valid HH:MM time: "+err.Error(), nil)
			}
			seen := map[string]bool{}
			for _, day := range rangeW.Days {
				if !validWeekday(day) {
					return newValidationError(rprefix+".days contains unknown weekday: "+day, nil)
				}
				key := weekdayKey(day)
				if seen[key] {
					return newValidationError(rprefix+".days contains duplicate weekday: "+day, nil)
				}
				seen[key] = true
			}
		}
		rates := window.Pricing
		if rates.InputPerMtok == nil &&
			rates.OutputPerMtok == nil &&
			rates.CachedInputPerMtok == nil &&
			rates.CacheWritePerMtok == nil {
			return newValidationError(prefix+".pricing must set at least one rate", nil)
		}
		for _, field := range []pricingField{
			{"input_per_mtok", rates.InputPerMtok},
			{"output_per_mtok", rates.OutputPerMtok},
			{"cached_input_per_mtok", rates.CachedInputPerMtok},
			{"cache_write_per_mtok", rates.CacheWritePerMtok},
		} {
			if field.value != nil && *field.value < 0 {
				return newValidationError(prefix+".pricing."+field.name+" must be greater than or equal to 0", nil)
			}
		}
	}
	return nil
}

func validateClock(value string) error {
	// Reuse core's strict parser: "HH:MM", hours 00-24, minutes 00-59,
	// "24:00" only allowed for an end-of-day bound.
	if _, ok := core.ParseClockMinutes(value); !ok {
		return errors.New("invalid time (\"" + value + "\")")
	}
	return nil
}

func validWeekday(day string) bool {
	_, ok := core.ParseWeekday(day)
	return ok
}

func weekdayKey(day string) string {
	d, _ := core.ParseWeekday(day)
	return d.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func tierLimit(t PricingTier) float64 {
	if t.UpToTokens != nil {
		return *t.UpToTokens
	}
	if t.UpToMtok != nil {
		return *t.UpToMtok * 1_000_000
	}
	return 0
}
