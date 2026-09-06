package gateway

import (
	"context"

	"github.com/enterpilot/gomodel/ext"
	"github.com/enterpilot/gomodel/internal/core"
)

// BuildRouteContent summarizes a chat request into the lightweight,
// non-reversible RouteContent that adaptive route selectors consume. Only
// counts and lengths cross the boundary — never message bodies.
func BuildRouteContent(req *core.ChatRequest) *ext.RouteContent {
	if req == nil || len(req.Messages) == 0 {
		return nil
	}
	content := &ext.RouteContent{MessageCount: len(req.Messages), ToolDefs: len(req.Tools)}
	var totalChars, userChars, userTurns int
	lastUserText := ""
	for _, msg := range req.Messages {
		text := core.ExtractTextContent(msg.Content)
		totalChars += len(text)
		if msg.Role == "user" {
			userTurns++
			userChars += len(text)
			lastUserText = text
		}
	}
	if userTurns > 0 {
		content.AvgUserChars = userChars / userTurns
	}
	content.UserTurns = userTurns
	content.CurrentChars = len(lastUserText)
	content.ContextChars = totalChars
	content.HasCode = core.DetectCodeFence(lastUserText)
	content.HasMultiStep = core.DetectMultiStepList(lastUserText)
	return content
}

// BuildRequiredCapabilities lists the capability keys the request demands:
// tool definitions require function_calling; any image part requires vision.
func BuildRequiredCapabilities(req *core.ChatRequest) []string {
	if req == nil {
		return nil
	}
	var required []string
	if len(req.Tools) > 0 {
		required = append(required, "function_calling")
	}
	if messagesHaveImage(req.Messages) {
		required = append(required, "vision")
	}
	return required
}

func messagesHaveImage(messages []core.Message) bool {
	for _, msg := range messages {
		if parts, ok := core.NormalizeContentParts(msg.Content); ok {
			for _, part := range parts {
				if part.Type == "image_url" && part.ImageURL != nil {
					return true
				}
			}
		}
	}
	return false
}

// BuildRouteContentResponses summarizes a Responses-API request (string or
// element input) into the same lightweight shape as the chat variant.
func BuildRouteContentResponses(req *core.ResponsesRequest) *ext.RouteContent {
	if req == nil {
		return nil
	}
	content := &ext.RouteContent{ToolDefs: len(req.Tools)}
	switch input := req.Input.(type) {
	case string:
		if input == "" {
			return nil
		}
		content.MessageCount = 1
		content.UserTurns = 1
		content.CurrentChars = len(input)
		content.AvgUserChars = len(input)
		content.ContextChars = len(input)
		content.HasCode = core.DetectCodeFence(input)
		content.HasMultiStep = core.DetectMultiStepList(input)
	case []core.ResponsesInputElement:
		if len(input) == 0 {
			return nil
		}
		content.MessageCount = len(input)
		var totalChars, userChars, userTurns int
		lastUserText := ""
		for _, item := range input {
			if item.Type != "" && item.Type != "message" {
				continue
			}
			text := core.ExtractTextContent(item.Content)
			totalChars += len(text)
			if item.Role == "user" {
				userTurns++
				userChars += len(text)
				lastUserText = text
			}
		}
		content.UserTurns = userTurns
		if userTurns > 0 {
			content.AvgUserChars = userChars / userTurns
		}
		content.CurrentChars = len(lastUserText)
		content.ContextChars = totalChars
		content.HasCode = core.DetectCodeFence(lastUserText)
		content.HasMultiStep = core.DetectMultiStepList(lastUserText)
	default:
		if req.Input == nil {
			return nil
		}
		return content
	}
	return content
}

// BuildRequiredCapabilitiesResponses mirrors BuildRequiredCapabilities for
// the Responses API: the image part type there is "input_image".
func BuildRequiredCapabilitiesResponses(req *core.ResponsesRequest) []string {
	if req == nil {
		return nil
	}
	var required []string
	if len(req.Tools) > 0 {
		required = append(required, "function_calling")
	}
	if responsesInputHasImage(req.Input) {
		required = append(required, "vision")
	}
	return required
}

func responsesInputHasImage(input any) bool {
	items, ok := input.([]core.ResponsesInputElement)
	if !ok {
		return false
	}
	for _, item := range items {
		if parts, ok := core.NormalizeContentParts(item.Content); ok {
			for _, part := range parts {
				if (part.Type == "input_image" || part.Type == "image_url") && part.ImageURL != nil {
					return true
				}
			}
		}
	}
	return false
}

// WithRouteSummary attaches the request summary and capability demands to
// ctx so model resolution (and, through it, the adaptive route selector) can
// consult them without the whole request body crossing package boundaries.
// Nil content and empty requirements are simply not stored.
func WithRouteSummary(ctx context.Context, content *ext.RouteContent, required []string) context.Context {
	ctx = ext.WithRouteContent(ctx, content)
	return ext.WithRequiredCapabilities(ctx, required)
}
