package providers

import (
	"maps"
	"strings"

	"github.com/enterpilot/gomodel/internal/core"
)

// MergeModelCapabilities persists an operator-confirmed capability verdict
// (from an offline probe or audit-log observation) onto the given provider's
// model metadata overrides. Confirmed capabilities ride the config-override
// channel, so they win over registry and heuristic sources at every later
// re-enrichment, exactly like a config.yaml metadata block.
//
// Pass a value of false to record an operator-confirmed "unsupported" (the
// only sanctioned negative: an explicit upstream rejection or observation
// rule, never a silent probe miss). Returns false when the provider or model
// is unknown, or caps is empty.
func (r *ModelRegistry) MergeModelCapabilities(providerName, modelID string, caps map[string]bool, source string) bool {
	providerName = strings.TrimSpace(providerName)
	modelID = strings.TrimSpace(modelID)
	if providerName == "" || modelID == "" || len(caps) == 0 {
		return false
	}
	r.mu.Lock()
	models, known := r.discoveredByProvider[providerName]
	if !known {
		r.mu.Unlock()
		return false
	}
	if _, ok := models[modelID]; !ok {
		r.mu.Unlock()
		return false
	}
	if r.configMetadataOverrides == nil {
		r.configMetadataOverrides = make(map[string]map[string]*core.ModelMetadata)
	}
	if r.configMetadataOverrides[providerName] == nil {
		r.configMetadataOverrides[providerName] = make(map[string]*core.ModelMetadata)
	}
	existing := r.configMetadataOverrides[providerName][modelID]
	if existing == nil {
		existing = &core.ModelMetadata{}
	} else {
		existing = existing.Clone()
	}
	if existing.Capabilities == nil {
		existing.Capabilities = make(map[string]bool, len(caps))
	}
	if existing.CapabilitySources == nil {
		existing.CapabilitySources = make(map[string]string, len(caps))
	}
	maps.Copy(existing.Capabilities, caps)
	for key := range caps {
		existing.CapabilitySources[key] = source
	}
	// modeldata.MergeMetadata semantics need a modes-bearing override only
	// when the caller probed a type; capabilities alone must not retype the
	// model, so Modes stays untouched here.
	r.configMetadataOverrides[providerName][modelID] = existing
	r.enrichModelsLocked()
	r.mu.Unlock()
	return true
}
