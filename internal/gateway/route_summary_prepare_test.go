package gateway

import (
	"context"
	"io"
	"testing"

	"github.com/enterpilot/gomodel/ext"
	"github.com/enterpilot/gomodel/internal/core"
)

// summaryCapturingResolver records the context the resolver chain sees.
type summaryCapturingResolver struct {
	content  *ext.RouteContent
	required []string
}

func (r *summaryCapturingResolver) ResolveModel(requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	return core.ModelSelector{Provider: "openai", Model: requested.Model}, true, nil
}

func (r *summaryCapturingResolver) ResolveModelForUserPath(ctx context.Context, requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	r.content = ext.RouteContentFromContext(ctx)
	r.required = ext.RequiredCapabilitiesFromContext(ctx)
	return r.ResolveModel(requested)
}

// routableStub is the minimal RoutableProvider the resolution path needs.
type routableStub struct{}

func (routableStub) ChatCompletion(context.Context, *core.ChatRequest) (*core.ChatResponse, error) {
	return nil, nil
}
func (routableStub) StreamChatCompletion(context.Context, *core.ChatRequest) (io.ReadCloser, error) {
	return nil, nil
}
func (routableStub) ListModels(context.Context) (*core.ModelsResponse, error) { return nil, nil }
func (routableStub) Responses(context.Context, *core.ResponsesRequest) (*core.ResponsesResponse, error) {
	return nil, nil
}
func (routableStub) StreamResponses(context.Context, *core.ResponsesRequest) (io.ReadCloser, error) {
	return nil, nil
}
func (routableStub) Embeddings(context.Context, *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return nil, nil
}
func (routableStub) Supports(string) bool          { return true }
func (routableStub) GetProviderType(string) string { return "openai" }

// PrepareChatRequest must stage the request summary on the context before
// model resolution runs, so an adaptive route selector downstream sees it.
func TestPrepareChatRequestThreadsRouteSummary(t *testing.T) {
	t.Parallel()
	resolver := &summaryCapturingResolver{}
	o := &InferenceOrchestrator{provider: routableStub{}, modelResolver: resolver}
	req := &core.ChatRequest{
		Model:    "smart",
		Messages: []core.Message{{Role: "user", Content: "hello world"}},
		Tools:    []map[string]any{{"type": "function"}},
	}
	meta := RequestMeta{Endpoint: core.EndpointDescriptor{Operation: core.OperationChatCompletions}}

	prepared, err := o.PrepareChatRequest(context.Background(), req, meta)
	if err != nil {
		t.Fatalf("PrepareChatRequest() error = %v", err)
	}
	if resolver.content == nil || resolver.content.MessageCount != 1 || resolver.content.CurrentChars != len("hello world") {
		t.Fatalf("resolver saw content %+v, want the request summary", resolver.content)
	}
	if len(resolver.required) != 1 || resolver.required[0] != "function_calling" {
		t.Fatalf("resolver saw requirements %v, want [function_calling]", resolver.required)
	}
	// The prepared context keeps carrying the summary for later stages.
	if ext.RouteContentFromContext(prepared.Context) == nil {
		t.Fatal("prepared context lost the route summary")
	}
}

// The Responses API path gets the same treatment.
func TestPrepareResponsesRequestThreadsRouteSummary(t *testing.T) {
	t.Parallel()
	resolver := &summaryCapturingResolver{}
	o := &InferenceOrchestrator{provider: routableStub{}, modelResolver: resolver}
	req := &core.ResponsesRequest{Model: "smart", Input: "a question"}
	meta := RequestMeta{Endpoint: core.EndpointDescriptor{Operation: core.OperationResponses}}
	if _, err := o.PrepareResponsesRequest(context.Background(), req, meta); err != nil {
		t.Fatalf("PrepareResponsesRequest() error = %v", err)
	}
	if resolver.content == nil || resolver.content.CurrentChars != len("a question") {
		t.Fatalf("resolver saw content %+v", resolver.content)
	}
}

// Embeddings and other body-less paths attach nothing; resolution still works.
func TestPrepareEmbeddingRequestWithoutSummary(t *testing.T) {
	t.Parallel()
	resolver := &summaryCapturingResolver{}
	o := &InferenceOrchestrator{provider: routableStub{}, modelResolver: resolver}
	req := &core.EmbeddingRequest{Model: "embed-model", Input: "text"}
	meta := RequestMeta{Endpoint: core.EndpointDescriptor{Operation: core.OperationEmbeddings}}
	if _, err := o.PrepareEmbeddingRequest(context.Background(), req, meta); err != nil {
		t.Fatalf("PrepareEmbeddingRequest() error = %v", err)
	}
	if resolver.content != nil {
		t.Fatalf("embedding path attached content %+v, want none", resolver.content)
	}
}


// tieringResolver answers differently based on whether the route summary is
// present, simulating an adaptive selector: without the summary it round-robins
// (middleware resolution), with it it picks the tier model.
type tieringResolver struct {
	sawContent *ext.RouteContent
	calls      int
}

func (r *tieringResolver) ResolveModel(requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	return core.ModelSelector{Provider: "openai", Model: requested.Model}, true, nil
}

func (r *tieringResolver) ResolveModelForUserPath(ctx context.Context, requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	r.calls++
	if content := ext.RouteContentFromContext(ctx); content != nil {
		r.sawContent = content
		return core.ModelSelector{Provider: "openai", Model: "tier-picked-model"}, true, nil
	}
	return core.ModelSelector{Provider: "openai", Model: requested.Model}, true, nil
}

// A workflow resolved by the middleware (before the request summary exists)
// must be re-resolved once PrepareChatRequest attaches the summary, so an
// adaptive route selector sees the request content on the live path.
func TestPrepareChatRequestReresolvesMiddlewareWorkflowWithSummary(t *testing.T) {
	t.Parallel()
	resolver := &tieringResolver{}
	o := &InferenceOrchestrator{provider: routableStub{}, modelResolver: resolver}
	req := &core.ChatRequest{
		Model:    "smart",
		Messages: []core.Message{{Role: "user", Content: "analyze this code please"}},
	}
	// Simulate the middleware: resolve without a summary and store the workflow.
	middlewareCtx := context.Background()
	resolved, _, err := resolver.ResolveModelForUserPath(middlewareCtx, core.NewRequestedModelSelector("smart", ""))
	if err != nil {
		t.Fatal(err)
	}
	resolution := &core.RequestModelResolution{
		Requested:        core.NewRequestedModelSelector("smart", ""),
		ResolvedSelector: resolved,
		AliasApplied:     true,
	}
	meta := RequestMeta{
		Endpoint: core.EndpointDescriptor{Operation: core.OperationChatCompletions},
		Workflow: &core.Workflow{
			Mode:       core.ExecutionModeTranslated,
			Endpoint:   core.EndpointDescriptor{Operation: core.OperationChatCompletions},
			Resolution: resolution,
		},
	}

	prepared, err := o.PrepareChatRequest(context.Background(), req, meta)
	if err != nil {
		t.Fatalf("PrepareChatRequest() error = %v", err)
	}
	if resolver.calls < 2 {
		t.Fatalf("resolver called %d times, want re-resolution after summary attach", resolver.calls)
	}
	if resolver.sawContent == nil {
		t.Fatal("re-resolution did not see the route summary")
	}
	got := prepared.Workflow.Resolution.ResolvedQualifiedModel()
	if got != "openai/tier-picked-model" {
		t.Fatalf("resolved model = %q, want the summary-aware pick openai/tier-picked-model", got)
	}
	if req.Model != "tier-picked-model" {
		t.Fatalf("request model = %q, want patched to tier-picked-model", req.Model)
	}
}
