package server

import (
	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/gateway"
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

// CapabilityMetadataResolver supplies the detected metadata the runtime
// detector needs. *providers.ModelRegistry satisfies it; the server keeps an
// interface so tests can stub it.
type CapabilityMetadataResolver interface {
	// ResolveCapabilityMetadata returns the model's detected modes and
	// capability flags, or ok=false when the registry has no judgment.
	ResolveCapabilityMetadata(providerType, modelID string) (modes []string, caps map[string]bool, ok bool)
}

// detectCapabilityError runs the W3 mismatch detector for a resolved model
// and records the verdict on the audit entry. Best-effort: without a
// resolver (tests, embedders) it is a no-op.
func detectCapabilityError(c *echo.Context, resolver CapabilityMetadataResolver, workflow *core.Workflow, req *core.ChatRequest) {
	if resolver == nil || workflow == nil || workflow.Resolution == nil {
		return
	}
	providerType := workflow.Resolution.ProviderType
	model := workflow.Resolution.ResolvedSelector.Model
	modes, caps, ok := resolver.ResolveCapabilityMetadata(providerType, model)
	if !ok {
		return
	}
	required := gateway.BuildRequiredCapabilities(req)
	kind := gateway.DetectCapabilityError(caps, modes, required, workflow.Endpoint.Operation)
	if kind == "" {
		return
	}
	auditlog.EnrichEntryWithCapabilityError(c, string(kind))
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
