package gateway

import (
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

func TestEndpointServesModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		modes    []string
		op       core.Operation
		serves   bool
		detected bool
	}{
		{"chat model on chat", []string{"chat"}, core.OperationChatCompletions, true, true},
		{"embedding model on chat", []string{"embedding"}, core.OperationChatCompletions, false, true},
		{"embedding model on embeddings", []string{"embedding"}, core.OperationEmbeddings, true, true},
		{"audio speech on speech", []string{"audio_speech"}, core.OperationAudioSpeech, true, true},
		{"image on generations", []string{"image_generation"}, core.OperationImageGenerations, true, true},
		{"speech-to-text model on transcriptions", []string{"audio_transcription"}, core.OperationAudioTranscriptions, false, true},
		{"unlabeled model assumed compatible", nil, core.OperationChatCompletions, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serves, detected := EndpointServesModel(tc.modes, tc.op)
			if serves != tc.serves || detected != tc.detected {
				t.Fatalf("EndpointServesModel(%v, %s) = %v/%v, want %v/%v", tc.modes, tc.op, serves, detected, tc.serves, tc.detected)
			}
		})
	}
}

func TestDetectCapabilityError(t *testing.T) {
	t.Parallel()
	// Type mismatch: embedding model requested for chat.
	if got := DetectCapabilityError(nil, []string{"embedding"}, nil, core.OperationChatCompletions); got != CapabilityErrorType {
		t.Fatalf("type mismatch = %q, want type_mismatch", got)
	}
	// Capability mismatch: explicit vision=false and an image request.
	if got := DetectCapabilityError(map[string]bool{"vision": false}, []string{"chat"}, []string{"vision"}, core.OperationChatCompletions); got != CapabilityErrorCapability {
		t.Fatalf("capability mismatch = %q, want capability_mismatch", got)
	}
	// Unlabeled vision must NOT fail (uncertainty respected).
	if got := DetectCapabilityError(nil, []string{"chat"}, []string{"vision"}, core.OperationChatCompletions); got != "" {
		t.Fatalf("unlabeled vision = %q, want no error", got)
	}
	// function_calling is a soft preference: even explicit false never fails.
	if got := DetectCapabilityError(map[string]bool{"function_calling": false}, []string{"chat"}, []string{"function_calling"}, core.OperationChatCompletions); got != "" {
		t.Fatalf("soft preference = %q, want no error", got)
	}
	// Everything compatible.
	if got := DetectCapabilityError(map[string]bool{"vision": true}, []string{"chat"}, []string{"vision"}, core.OperationChatCompletions); got != "" {
		t.Fatalf("compatible = %q, want no error", got)
	}
	// Type mismatch takes precedence (checked first).
	if got := DetectCapabilityError(map[string]bool{"vision": false}, []string{"embedding"}, []string{"vision"}, core.OperationChatCompletions); got != CapabilityErrorType {
		t.Fatalf("precedence = %q, want type_mismatch", got)
	}
}
