package ext

import "context"

// RouteContent is the lightweight, non-reversible summary of the incoming
// request that a complexity-aware RouteSelector consumes. Core never ships
// message bodies to extensions: only counts, lengths, and boolean signals
// derived from them, so a selector can estimate task complexity without the
// privacy and memory cost of the full transcript. It is nil on RouteRequest
// when the endpoint has no chat-like body to summarize (embeddings,
// passthrough, realtime).
type RouteContent struct {
	// MessageCount is the total number of messages in the request.
	MessageCount int
	// UserTurns counts messages with role "user".
	UserTurns int
	// ContextChars sums the character length of all message content.
	ContextChars int
	// CurrentChars is the character length of the last message.
	CurrentChars int
	// AvgUserChars is the mean user-message length (0 when none).
	AvgUserChars int
	// ToolDefs is the number of tool definitions the request carries.
	ToolDefs int
	// HasCode reports that some message contains a fenced code block.
	HasCode bool
	// HasMultiStep reports that some message contains a multi-step list
	// (three or more numbered items, or four or more bullet items).
	HasMultiStep bool
}

type routeContentKey struct{}
type requiredCapabilitiesKey struct{}

// WithRouteContent returns a context carrying the request summary for
// downstream model resolution to pick up. A nil content is not stored.
func WithRouteContent(ctx context.Context, content *RouteContent) context.Context {
	if ctx == nil || content == nil {
		return ctx
	}
	return context.WithValue(ctx, routeContentKey{}, content)
}

// RouteContentFromContext retrieves the request summary, or nil when the
// request path attached none.
func RouteContentFromContext(ctx context.Context) *RouteContent {
	if ctx == nil {
		return nil
	}
	content, _ := ctx.Value(routeContentKey{}).(*RouteContent)
	return content
}

// WithRequiredCapabilities returns a context carrying the capability keys the
// request demands (for example "vision" or "function_calling"). An empty or
// nil list is not stored.
func WithRequiredCapabilities(ctx context.Context, capabilities []string) context.Context {
	if ctx == nil || len(capabilities) == 0 {
		return ctx
	}
	return context.WithValue(ctx, requiredCapabilitiesKey{}, capabilities)
}

// RequiredCapabilitiesFromContext retrieves the demanded capability keys, or
// nil when none were attached.
func RequiredCapabilitiesFromContext(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	capabilities, _ := ctx.Value(requiredCapabilitiesKey{}).([]string)
	return capabilities
}
