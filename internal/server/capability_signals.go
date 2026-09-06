package server

import (
	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/core"
)

// enrichCapabilitySignals records the passive-observation signal triple
// (tools present, tool_calls observed, image present) on the request's audit
// entry. Distilled booleans only — the bodies themselves stay governed by
// the LogBodies setting. Call sites: chat dispatch (request side) and each
// response hand-off (tool_calls side).
func enrichCapabilitySignals(c *echo.Context, req *core.ChatRequest, resp *core.ChatResponse) {
	signals := auditlog.CapabilitySignals{}
	if req != nil {
		signals.HadTools = len(req.Tools) > 0
		signals.HadImage = chatRequestHasImage(req.Messages)
	}
	if resp != nil && chatResponseHasToolCalls(resp) {
		signals.ResponseToolCalls = true
	}
	auditlog.EnrichEntryWithCapabilitySignals(c, signals)
}

func chatRequestHasImage(messages []core.Message) bool {
	for _, msg := range messages {
		if parts, ok := core.NormalizeContentParts(msg.Content); ok {
			for _, part := range parts {
				if (part.Type == "image_url" || part.Type == "input_image") && part.ImageURL != nil {
					return true
				}
			}
		}
	}
	return false
}

func chatResponseHasToolCalls(resp *core.ChatResponse) bool {
	if resp == nil {
		return false
	}
	for _, choice := range resp.Choices {
		for _, call := range choice.Message.ToolCalls {
			if call.ID != "" || call.Function.Name != "" {
				return true
			}
		}
	}
	return false
}
