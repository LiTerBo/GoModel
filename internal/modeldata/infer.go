package modeldata

import "strings"

// embeddingFamilyTokens are model-family names that identify embedding models
// without containing the substring "embed" (bge-m3, e5-large-v2, gte-large,
// all-minilm). Matched as whole delimited tokens only, so IDs like
// "gemma-3n-e4b" or "bge2000-chat" are not misclassified.
var embeddingFamilyTokens = map[string]struct{}{
	"bge":    {},
	"e5":     {},
	"gte":    {},
	"minilm": {},
}

// asrMarkers identify automatic-speech-recognition model families (whisper,
// SenseVoice, wav2vec, Paraformer, Hubert, Seamless, ...). These names are
// specific enough to match as substrings.
var asrMarkers = []string{
	"sensevoice", "whisper", "wav2vec", "paraformer",
	"hubert", "conformer", "canary", "seamless",
}

// ttsMarkers identify text-to-speech model families (Bark, Kokoro, CosyVoice,
// Piper, VITS, XTTS, ...).
var ttsMarkers = []string{
	"bark", "kokoro", "cosyvoice", "speecht5", "piper", "vits", "xtts", "tortoise",
}

// audioConversationMarkers identify audio-language / speech-chat models that
// converse over both text and audio (LFM2.5-Audio, Qwen2-Audio, GLM-4-Voice,
// Qwen2.5-Omni, ...). They keep chat mode and gain audio_transcription.
var audioConversationMarkers = []string{
	"audio", "voice", "speech", "omni",
}

// imageFamilyTokens are model-family names that identify image generation models.
// "sd" (Stable Diffusion) is a prefix match on tokens to catch "sd3", "sdxl" etc.
// without false positives on unrelated model IDs like "gsd".
var imageFamilyTokens = map[string]struct{}{
	"sd": {},
}

// visionFamilyTokens are model-family names that identify vision-language models.
// "vl" (Vision-Language) is a whole-token match so "vllm" is not misclassified.
var visionFamilyTokens = map[string]struct{}{
	"vl": {},
}

// IsVisionModelID checks whether a model ID looks like a vision-capable model.
// Case-insensitive; prefers the final segment of namespaced IDs (hf repo paths).
// This is the single source of truth for ID-based vision detection throughout
// the gateway — use it from InferModesFromID, InferCapabilitiesFromID, and any
// compression or preprocessing path that needs to decide whether images can
// be passed to a model.
func IsVisionModelID(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		id = id[idx+1:]
	}
	if id == "" {
		return false
	}
	if strings.Contains(id, "qwenvl") || strings.Contains(id, "vision") {
		return true
	}
	for _, token := range strings.FieldsFunc(id, isModelIDDelimiter) {
		if _, ok := visionFamilyTokens[token]; ok {
			return true
		}
	}
	return false
}

// InferModesFromID guesses a model's modes from its ID alone. It is a
// last-resort fallback for models absent from the remote model registry —
// typically local models served by llama.cpp, LM Studio, Ollama, or vLLM,
// whose IDs (often GGUF file names or user-chosen aliases) the registry can
// never enumerate. Without a mode, such a model is never categorized as an
// embedding model anywhere in the gateway even though calling it works fine.
//
// Rules are ordered from most specific to least specific. The last rule
// defaults to chat mode, which is the most common model type for local engines.
// Real registry entries and operator-declared metadata always take precedence.
func InferModesFromID(modelID string) []string {
	id := strings.ToLower(strings.TrimSpace(modelID))
	// Namespaced IDs (hf repo paths, "org/model") classify by the final segment.
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		id = id[idx+1:]
	}
	if id == "" {
		return nil
	}

	// 1. Rerankers — most specific pattern.
	if strings.Contains(id, "rerank") {
		return []string{"rerank"}
	}

	// 2. Embedding family tokens (whole-token match).
	for _, token := range strings.FieldsFunc(id, isModelIDDelimiter) {
		if _, ok := embeddingFamilyTokens[token]; ok {
			return []string{"embedding"}
		}
	}

	// 3. "embed" substring — the common local-model spelling.
	if strings.Contains(id, "embed") {
		return []string{"embedding"}
	}

	// 4. Audio models — ASR, then TTS, then generic speech-chat markers.
	for _, marker := range asrMarkers {
		if strings.Contains(id, marker) {
			return []string{"audio_transcription"}
		}
	}
	for _, marker := range ttsMarkers {
		if strings.Contains(id, marker) {
			return []string{"audio_speech"}
		}
	}
	for _, marker := range audioConversationMarkers {
		if strings.Contains(id, marker) {
			return []string{"chat", "audio_transcription"}
		}
	}

	// 5. Image generation models.
	if strings.Contains(id, "stablediffusion") ||
		strings.Contains(id, "flux") ||
		strings.Contains(id, "dall-e") {
		return []string{"image_generation"}
	}
	for _, token := range strings.FieldsFunc(id, isModelIDDelimiter) {
		if _, ok := imageFamilyTokens[token]; ok {
			return []string{"image_generation"}
		}
		// Prefix match for "sd" tokens: "sd3", "sdxl", "sd-turbo" etc.
		// Must be at least 2 more chars ("sd" + at least 1) to avoid
		// matching bare "sd" which is already caught above.
		if strings.HasPrefix(token, "sd") && len(token) > 2 {
			return []string{"image_generation"}
		}
	}

	// 6. Vision-language models — chat + vision capability.
	// Single source of truth via IsVisionModelID (see docs/design/model-capability-detection.md §3.A).
	if IsVisionModelID(id) {
		return []string{"chat"}
	}

	// 7. Default: text generation / chat — the most common model type for local engines.
	return []string{"chat"}
}

// unsupportedCapabilityPatterns lists model ID substrings whose models should
// NEVER receive the corresponding capability, even if the registry or provider
// reports it. This prevents specialty models (whisper, embedding, image gen,
// etc.) from optimistically inheriting tool-calling, vision, or reasoning
// through registry metadata that was written for a different family.
//
// Inspired by OmniRoute's TOOL_CALLING_UNSUPPORTED_PATTERNS and
// REASONING_UNSUPPORTED_PATTERNS (modelCapabilities.ts).
var unsupportedCapabilityPatterns = map[string][]string{
	"function_calling": {
		"whisper",
		"tts-1",
		"omni-moderation",
		"moderation",
		"rerank",
		"embedding",
		"embed",
		"dall-e",
		"flux",
		"stable-diffusion",
		"seedance",
		"/veo",
		"veo-",
	},
	"vision": {
		// Known text-only model families that the registry or provider might
		// mislabel as vision-capable. Add more as discovered.
	},
}

// ApplyCapabilityBlacklist removes capabilities that a model's ID identifies
// as unsupported for that model family. Mutates the map in place and returns
// it, so callers can use it inline. Returns nil when the input is nil.
//
// Call this on the final capability set (after registry merge, discovery, and
// heuristic inference) so specialty models never carry bogus capabilities.
func ApplyCapabilityBlacklist(modelID string, caps map[string]bool) map[string]bool {
	if caps == nil {
		return nil
	}
	id := strings.ToLower(strings.TrimSpace(modelID))
	for capability, patterns := range unsupportedCapabilityPatterns {
		if !caps[capability] {
			continue
		}
		for _, pattern := range patterns {
			if strings.Contains(id, pattern) {
				delete(caps, capability)
				break
			}
		}
	}
	return caps
}

// InferCapabilitiesFromID guesses capability flags from a model ID alone.
// Returns the capabilities that can be inferred, or nil if none.
// Must be called alongside InferModesFromID for a complete picture.
// Vision detection delegates to IsVisionModelID (single source of truth);
// the result is passed through ApplyCapabilityBlacklist to strip capabilities
// that are known to be unsupported for this model family.
func InferCapabilitiesFromID(modelID string) map[string]bool {
	var caps map[string]bool
	if IsVisionModelID(modelID) {
		caps = map[string]bool{"vision": true}
	}
	return ApplyCapabilityBlacklist(modelID, caps)
}

func isModelIDDelimiter(r rune) bool {
	switch r {
	case '-', '_', '.', ':', '@', ' ':
		return true
	}
	return false
}
