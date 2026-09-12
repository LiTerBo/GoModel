package virtualmodels

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/enterpilot/gomodel/internal/core"
)

// ResolutionConfigChanged reports whether an admin write would change which
// models a caller reaching this row by name can end up talking to: the target
// set, their weights and order, and the strategy that picks among them. A row
// that is not a redirect has no pointing of its own, so turning one into a
// redirect (or back) counts as a change.
//
// The comparison mirrors how a write normalizes a row, so an edit that only
// touches the description, the user-path scope, the slowdown factor or the
// enabled flag does not read as a change.
func ResolutionConfigChanged(stored, next VirtualModel) bool {
	return resolutionSignature(stored) != resolutionSignature(next)
}

// resolutionSignature fingerprints the fields that decide the pointing.
func resolutionSignature(vm VirtualModel) string {
	if !vm.IsRedirect() {
		return "policy"
	}
	var builder strings.Builder
	strategy := normalizeStrategy(vm.Strategy)
	builder.WriteString(strategy)
	builder.WriteByte('|')
	// Plugin fields only mean something under the plugin strategy: a write drops
	// them otherwise, so they must not read as a change there.
	if strategy == StrategyPlugin {
		builder.WriteString(strings.TrimSpace(vm.StrategyPlugin))
		builder.WriteByte('|')
		builder.WriteString(strategyConfigSignature(vm.StrategyConfig))
	}
	builder.WriteByte('|')
	// A write collapses duplicate targets, so spelling one twice is not a change.
	seen := make(map[string]struct{}, len(vm.Targets))
	for _, target := range vm.Targets {
		key := targetSignature(target)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		builder.WriteString(key)
		builder.WriteByte(',')
	}
	return builder.String()
}

func targetSignature(target Target) string {
	label := strings.TrimSpace(target.Provider) + "/" + strings.TrimSpace(target.Model)
	if selector, err := core.ParseModelSelector(target.Model, target.Provider); err == nil {
		label = selector.QualifiedModel()
	}
	return label + "@" + strconv.FormatFloat(target.Weight, 'g', -1, 64)
}

// strategyConfigSignature serializes a strategy config deterministically. An
// empty config reads the same whether it arrived as nil or as {}: a plugin row
// is normalized to an empty map on write.
func strategyConfigSignature(config map[string]any) string {
	if len(config) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return "unencodable"
	}
	return string(encoded)
}
