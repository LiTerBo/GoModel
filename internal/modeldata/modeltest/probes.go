package modeltest

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/enterpilot/gomodel/internal/core"
)

// probeMaxTokens keeps every probe payload minimal: probes are cheap, and
// reasoning models happily burn their whole budget otherwise.
const probeMaxTokens = 8

// ChatProber / EmbeddingProber are the minimal upstream surfaces the tester
// needs. Providers.Router satisfies both.
type ChatProber interface {
	ChatCompletion(ctx context.Context, req *core.ChatRequest) (*core.ChatResponse, error)
}

type EmbeddingProber interface {
	Embeddings(ctx context.Context, req *core.EmbeddingRequest) (*core.EmbeddingResponse, error)
}

// weatherTool is the standard tool-calling probe payload: an answer that
// cannot be produced from knowledge without calling the tool. Models that
// merely know about get_weather must still emit the call (or refuse).
var weatherTool = []map[string]any{{
	"type": "function",
	"function": map[string]any{
		"name":        "get_weather",
		"description": "Get the current weather for a city",
		"parameters": map[string]any{
			"type":       "object",
			"properties": map[string]any{"city": map[string]any{"type": "string"}},
			"required":   []string{"city"},
		},
	},
}}

const probeToolPrompt = "What is the weather in Shanghai right now? You must use the get_weather tool."

func probeMessages() []core.Message {
	return []core.Message{{Role: "user", Content: "Reply with the single word: pong"}}
}

func chatRequestFor(model string, tools bool) *core.ChatRequest {
	req := &core.ChatRequest{
		Model:     model,
		Messages:  probeMessages(),
		MaxTokens: intPtr(probeMaxTokens),
	}
	if tools {
		req.Messages = []core.Message{{Role: "user", Content: probeToolPrompt}}
		req.Tools = weatherTool
		req.ToolChoice = "auto"
	}
	return req
}

func intPtr(v int) *int { return &v }

// runChatProbe executes one chat probe, timing it.
func runChatProbe(ctx context.Context, prober ChatProber, qualified string, tools bool) (*core.ChatResponse, time.Duration, error) {
	start := time.Now()
	resp, err := prober.ChatCompletion(ctx, chatRequestFor(qualified, tools))
	return resp, time.Since(start), err
}

// ToolCallObserved reports whether a chat response contains at least one
// non-empty tool call — the positive signal for function_calling.
func ToolCallObserved(resp *core.ChatResponse) bool {
	if resp == nil || len(resp.Choices) == 0 {
		return false
	}
	for _, call := range resp.Choices[0].Message.ToolCalls {
		if call.ID != "" || call.Function.Name != "" {
			return true
		}
	}
	return false
}

// ToolRejectionErr reports whether an upstream error explicitly refuses the
// tools parameter (a 4xx whose message names tools). This is the only
// negative evidence strong enough to suggest "unsupported" (D5+ rule).
func ToolRejectionErr(err error) bool {
	var gw *core.GatewayError
	if !errors.As(err, &gw) {
		return false
	}
	if gw.StatusCode < 400 || gw.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(gw.Message)
	return strings.Contains(msg, "tool") || strings.Contains(msg, "function")
}

// errorText renders an error for the audit-safe Result.Detail (message only;
// never the request or response body).
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
