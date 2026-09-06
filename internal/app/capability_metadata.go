package app

import (
	"github.com/enterpilot/gomodel/internal/providers"
)

// capabilityMetadataResolver adapts *providers.ModelRegistry to the server's
// CapabilityMetadataResolver interface, feeding the W3 runtime mismatch
// detector with the model catalog's detected modes and capabilities.
type capabilityMetadataResolver struct {
	registry *providers.ModelRegistry
}

// ResolveCapabilityMetadata returns the detected modes and capability flags
// for one model, or ok=false when the registry holds no judgment (unknown
// model or no inferred metadata), which the detector treats as "unlabeled".
func (a capabilityMetadataResolver) ResolveCapabilityMetadata(providerType, modelID string) (modes []string, caps map[string]bool, ok bool) {
	meta := a.registry.ResolveMetadata(providerType, modelID)
	if meta == nil {
		return nil, nil, false
	}
	return meta.Modes, meta.Capabilities, true
}
