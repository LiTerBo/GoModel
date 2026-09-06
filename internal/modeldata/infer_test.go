package modeldata

import (
	"reflect"
	"testing"
)

func TestInferModesFromID(t *testing.T) {
	tests := []struct {
		id   string
		want []string // expected modes; nil means no inference
	}{
		// ── Embedding: "embed" substring ───────────────────────────────
		{"nomic-embed-text", []string{"embedding"}},
		{"nomic-embed-text-v1.5.Q8_0.gguf", []string{"embedding"}},
		{"text-embedding-nomic-embed-text-v1.5@q8_0", []string{"embedding"}},
		{"mxbai-embed-large", []string{"embedding"}},
		{"snowflake-arctic-embed", []string{"embedding"}},
		{"embeddinggemma", []string{"embedding"}},
		{"qwen3-embedding-0.6b", []string{"embedding"}},
		{"text-embedding-3-small", []string{"embedding"}},
		{"granite-embedding:278m", []string{"embedding"}},
		// Family tokens without "embed" in the name.
		{"bge-m3", []string{"embedding"}},
		{"bge-large-en-v1.5", []string{"embedding"}},
		{"e5-large-v2", []string{"embedding"}},
		{"gte-large", []string{"embedding"}},
		{"all-minilm", []string{"embedding"}},
		{"all-MiniLM-L6-v2", []string{"embedding"}},
		// Namespaced IDs classify by the final path segment.
		{"BAAI/bge-m3", []string{"embedding"}},
		{"intfloat/e5-mistral-7b-instruct", []string{"embedding"}},
		// Rerankers.
		{"bge-reranker-v2-m3", []string{"rerank"}},
		{"jina-reranker-v2", []string{"rerank"}},
		// Token matching must not fire on lookalike substrings.
		{"gemma-3n-e4b", []string{"chat"}},
		{"bge2000-chat", []string{"chat"}},
		{"gte", []string{"embedding"}}, // bare family name still counts

		// ── Audio: ASR families → audio_transcription ─────────────────
		{"whisper-1", []string{"audio_transcription"}},
		{"openai/whisper-large-v3", []string{"audio_transcription"}},
		{"SenseVoiceSmall", []string{"audio_transcription"}},
		{"iic/SenseVoiceSmall", []string{"audio_transcription"}},
		{"wav2vec2-large", []string{"audio_transcription"}},
		{"facebook/wav2vec2-base", []string{"audio_transcription"}},
		{"paraformer-zh", []string{"audio_transcription"}},
		{"hubert-large", []string{"audio_transcription"}},
		{"seamless-m4t", []string{"audio_transcription"}},
		{"nvidia/canary-1b", []string{"audio_transcription"}},

		// ── Audio: TTS families → audio_speech ────────────────────────
		{"suno/bark", []string{"audio_speech"}},
		{"kokoro-v1.0", []string{"audio_speech"}},
		{"cosyvoice2-0.5b", []string{"audio_speech"}},
		{"piper-voice-en-us", []string{"audio_speech"}},
		{"speecht5-tts", []string{"audio_speech"}},
		{"vits-zh", []string{"audio_speech"}},

		// ── Audio: speech-chat / audio-language models ────────────────
		// Keep chat (they converse in text) and gain audio_transcription.
		{"LFM2.5-Audio-1.5B", []string{"chat", "audio_transcription"}},
		{"qwen2-audio-7b", []string{"chat", "audio_transcription"}},
		{"glm-4-voice", []string{"chat", "audio_transcription"}},
		{"qwen2.5-omni-3b", []string{"chat", "audio_transcription"}},

		// ── Image generation ──────────────────────────────────────────
		{"stablediffusion-xl", []string{"image_generation"}},
		{"sd3-medium", []string{"image_generation"}},
		{"sdxl-turbo", []string{"image_generation"}},
		{"flux-schnell", []string{"image_generation"}},
		{"dall-e-3", []string{"image_generation"}},
		{"FLUX.1-schnell", []string{"image_generation"}},

		// ── Vision-language models → chat ─────────────────────────────
		{"Qwen/Qwen2.5-VL-72B-Instruct", []string{"chat"}},
		{"qwen-vl-plus", []string{"chat"}},
		{"llava-1.6-vision", []string{"chat"}},

		// ── Default: chat ─────────────────────────────────────────────
		{"gpt-4o", []string{"chat"}},
		{"llama-3.1-8b-instruct", []string{"chat"}},
		{"qwen2.5-coder:7b", []string{"chat"}},
		{"", nil},
		{"  ", nil},
		{"org/", nil},
	}
	for _, tt := range tests {
		got := InferModesFromID(tt.id)
		if tt.want == nil {
			if len(got) != 0 {
				t.Errorf("InferModesFromID(%q) = %v, want none", tt.id, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("InferModesFromID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestInferCapabilitiesFromID(t *testing.T) {
	tests := []struct {
		id   string
		want map[string]bool // nil means no capabilities
	}{
		{"qwen-vl-plus", map[string]bool{"vision": true}},
		{"Qwen/Qwen2.5-VL-72B-Instruct", map[string]bool{"vision": true}},
		{"llava-vision-7b", map[string]bool{"vision": true}},
		{"qwen2-vl-7b", map[string]bool{"vision": true}},
		{"llama-3.1-8b-instruct", nil},
		{"whisper-1", nil},
		{"bge-m3", nil},
		{"vllm", nil},       // "vl" prefix must not match "vllm"
		{"vlan-model", nil}, // "vl" substring must not match "vlan"
		{"LFM2.5-Audio-1.5B", nil},
		{"", nil},
	}
	for _, tt := range tests {
		got := InferCapabilitiesFromID(tt.id)
		if tt.want == nil {
			if len(got) != 0 {
				t.Errorf("InferCapabilitiesFromID(%q) = %v, want none", tt.id, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("InferCapabilitiesFromID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestIsVisionModelID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		// ── Positive cases (current conservative heuristic) ────────────
		// Matched by "vl" token: qwen-vl-plus → token "vl"
		{"qwen-vl-plus", true},
		{"Qwen/Qwen2.5-VL-72B-Instruct", true},
		{"qwen2-vl-7b", true},
		{"qwen2.5-vl-72b", true},
		{"qwen3-vl-7b", true},
		// Matched by "vision" substring
		{"llava-vision-7b", true},
		{"llava-1.6-vision", true},
		// Matched by "qwenvl" substring
		{"qwenvl-plus", true},

		// ── Negative cases ──────────────────────────────────────────────
		{"llama-3.1-8b-instruct", false},
		{"whisper-1", false},
		{"bge-m3", false},
		{"vllm", false},       // "vl" is a whole-token match, not a substring
		{"vlan-model", false}, // "vl" is not a delimited token in "vlan"
		{"LFM2.5-Audio-1.5B", false},
		{"", false},
		{"  ", false},
		{"org/", false},
		{"gemma-3-12b", false},
		{"kimi-k2", false},
		{"qwen3-embedding-0.6b", false},
		// Known vision models NOT yet caught by the conservative heuristic.
		// These pass because the current fragment list is deliberately narrow
		// (see §3.A in docs/design/model-capability-detection.md). Broaden
		// the fragment list when the heuristic is the only source available
		// for a model and the false-negative cost is unacceptable.
		{"qvq-72b", false},
		{"internvl2-8b", false},
		{"minicpm-v-2.6", false},
		{"moondream2", false},
		{"glm-4v-9b", false},
		{"gpt-4o", false},
		{"gpt-4.1", false},
		{"gpt-4-turbo", false},
		{"gpt-5", false},
		{"gemini-2.0-flash", false},
		{"gemini-2.5-pro", false},
		{"claude-3-haiku", false},
		{"claude-sonnet-4", false},
		{"hcx-005", false},
		{"pixtral-large", false},
		{"llava-hf/llava-v1.6-mistral-7b", false},
		{"bakllava", false},
		{"mistral-medium-3", false},
		{"minimax-m3", false},
	}
	for _, tt := range tests {
		got := IsVisionModelID(tt.id)
		if got != tt.want {
			t.Errorf("IsVisionModelID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestApplyCapabilityBlacklist(t *testing.T) {
	tests := []struct {
		id   string
		caps map[string]bool
		want map[string]bool
	}{
		// Nil input returns nil
		{"whisper-1", nil, nil},
		// Empty input returns nil
		{"gpt-4o", map[string]bool{}, nil},
		// Normal chat model keeps its capabilities
		{"gpt-4o", map[string]bool{"function_calling": true, "vision": true}, map[string]bool{"function_calling": true, "vision": true}},
		{"llama-3.1-8b", map[string]bool{"function_calling": true}, map[string]bool{"function_calling": true}},
		// Whisper loses function_calling
		{"whisper-1", map[string]bool{"function_calling": true, "vision": true}, map[string]bool{"vision": true}},
		{"openai/whisper-large-v3", map[string]bool{"function_calling": true}, nil},
		// TTS models lose function_calling
		{"tts-1", map[string]bool{"function_calling": true}, nil},
		{"tts-1-hd", map[string]bool{"function_calling": true}, nil},
		// Moderation models lose function_calling
		{"omni-moderation-latest", map[string]bool{"function_calling": true}, nil},
		{"text-moderation-latest", map[string]bool{"function_calling": true}, nil},
		// Rerank + embedding models lose function_calling
		{"bge-reranker-v2-m3", map[string]bool{"function_calling": true}, nil},
		{"text-embedding-3-small", map[string]bool{"function_calling": true}, nil},
		{"nomic-embed-text", map[string]bool{"function_calling": true}, nil},
		// Image generation models lose function_calling
		{"dall-e-3", map[string]bool{"function_calling": true}, nil},
		{"flux-schnell", map[string]bool{"function_calling": true}, nil},
		{"stable-diffusion-xl", map[string]bool{"function_calling": true}, nil},
		{"FLUX.1-dev", map[string]bool{"function_calling": true}, nil},
		// Video generation models lose function_calling
		{"google/veo-2", map[string]bool{"function_calling": true}, nil},
		{"seedance-1.0", map[string]bool{"function_calling": true}, nil},
		// Non-matching IDs keep their capabilities
		{"some-unknown-model", map[string]bool{"function_calling": true}, map[string]bool{"function_calling": true}},
	}
	for _, tt := range tests {
		got := ApplyCapabilityBlacklist(tt.id, tt.caps)
		if tt.want == nil {
			if len(got) != 0 {
				t.Errorf("ApplyCapabilityBlacklist(%q, %v) = %v, want nil", tt.id, tt.caps, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ApplyCapabilityBlacklist(%q, %v) = %v, want %v", tt.id, tt.caps, got, tt.want)
		}
	}
}
