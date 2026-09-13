package gateway

import (
	"strings"

	"github.com/enterpilot/gomodel/internal/core"
)

// Capability error kinds for runtime detection (W3, K-1): the two mismatches
// the requirement names.
//   - type_mismatch: a request reached an endpoint the model's detected type
//     does not serve (an embedding model asked to chat).
//   - capability_mismatch: the request demanded a capability the model's
//     detected capability set explicitly lacks.
// Detection only: the result feeds the audit signal and the dashboard WARN,
// never a rejection or reroute of the live request.
type CapabilityErrorKind string

const (
	CapabilityErrorType       CapabilityErrorKind = "type_mismatch"
	CapabilityErrorCapability CapabilityErrorKind = "capability_mismatch"
)

// DetectCapabilityError inspects the resolved model's detected metadata
// against the request's demands. required comes from
// BuildRequiredCapabilities; caps/modes from the resolved model's metadata.
// It reports the first mismatch found, or "" when nothing contradicts.
//
// The static pipeline's uncertainty is respected: a capability missing from
// the map may mean "unsupported" or just "unlabeled" (local models default
// to no flags), so only an EXPLICIT false counts, and only vision — the
// hard gate from Q3 — drives a capability verdict. function_calling is a
// soft preference and never fails detection.
func DetectCapabilityError(caps map[string]bool, modes []string, required []string, operation core.Operation) CapabilityErrorKind {
	if serves, detected := EndpointServesModel(modes, operation); detected && !serves {
		return CapabilityErrorType
	}
	for _, capName := range required {
		if capName == "vision" && caps != nil && !caps["vision"] {
			return CapabilityErrorCapability
		}
	}
	return ""
}

// EndpointServesModel reports whether the operation matches the model's
// detected modes. detected=false means the model carries no mode judgment
// (unlabeled local model): assumed compatible rather than flagged.
func EndpointServesModel(modes []string, operation core.Operation) (serves, detected bool) {
	if len(modes) == 0 {
		return true, false
	}
	required := endpointMode(operation)
	for _, mode := range modes {
		if mode == required {
			return true, true
		}
	}
	return false, true
}

// endpointMode maps a gateway operation to the model mode it exercises.
func endpointMode(operation core.Operation) string {
	switch strings.TrimSpace(string(operation)) {
	case string(core.OperationEmbeddings):
		return "embedding"
	case string(core.OperationImageGenerations), string(core.OperationImageEdits):
		return "image_generation"
	case string(core.OperationAudioSpeech):
		return "audio_speech"
	default:
		// chat_completions, responses, realtime, transcriptions/translations
		// and the infrastructure operations all exercise chat models.
		return "chat"
	}
}
