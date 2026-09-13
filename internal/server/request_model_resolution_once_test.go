package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

// countingAliasResolver stands in for the virtual-model service: it resolves one
// alias by round robin over its targets and counts how many times the request
// pipeline consults it. Each consultation advances the rotation, so a pipeline
// that resolves a request twice silently moves the alias onto its second target
// and never comes back.
type countingAliasResolver struct {
	source  string
	targets []core.ModelSelector
	// contentAware makes the resolver claim the alias selects on request
	// content, the way an adaptive or plugin strategy does.
	contentAware bool

	mu    sync.Mutex
	next  int
	calls int
}

// RouteContentNeeded completes the gateway's optional content-aware resolver
// contract.
func (r *countingAliasResolver) RouteContentNeeded(context.Context, core.RequestedModelSelector) bool {
	return r.contentAware
}

func (r *countingAliasResolver) ResolveModel(requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls++
	if strings.TrimSpace(requested.Model) != r.source {
		return core.ModelSelector{}, false, nil
	}
	target := r.targets[r.next%len(r.targets)]
	r.next++
	return target, true, nil
}

func (r *countingAliasResolver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// recordingAuthorizer counts authorizations and can reject the resolved model.
type recordingAuthorizer struct {
	mu        sync.Mutex
	validated []string
	err       error
}

func (a *recordingAuthorizer) ValidateModelAccess(_ context.Context, selector core.ModelSelector) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.validated = append(a.validated, selector.QualifiedModel())
	return a.err
}

// AllowsModel and FilterPublicModels complete the gateway.ModelAuthorizer
// contract; this test only exercises the per-request validation.
func (a *recordingAuthorizer) AllowsModel(context.Context, core.ModelSelector) bool { return true }

func (a *recordingAuthorizer) FilterPublicModels(_ context.Context, models []core.Model) []core.Model {
	return models
}

func (a *recordingAuthorizer) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.validated)
}

func aliasChatProvider() *failoverProvider {
	response := func(model string) *core.ChatResponse {
		return &core.ChatResponse{
			ID:       "chatcmpl-" + model,
			Object:   "chat.completion",
			Model:    model,
			Provider: "test",
			Choices: []core.Choice{{
				Index:        0,
				Message:      core.ResponseMessage{Role: "assistant", Content: model + " ok"},
				FinishReason: "stop",
			}},
		}
	}
	return &failoverProvider{
		chatResponses: map[string]*core.ChatResponse{
			"test/gpt-4":         response("gpt-4"),
			"test/gpt-3.5-turbo": response("gpt-3.5-turbo"),
		},
		supportedModels: map[string]string{
			"test/gpt-4":         "test",
			"test/gpt-3.5-turbo": "test",
		},
	}
}

func postAliasChat(t *testing.T, srv *Server, model, requestID string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer master-key")
	req.Header.Set("X-Request-ID", requestID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

// A chat completion resolves its model once. The workflow middleware resolves it
// and caches the answer on the request, so the executor has to reuse that
// resolution: resolving again rotates the alias's round-robin cursor a second
// time, which pins a two-target alias to its second target on every request and
// leaves the first one unused until a failover.
func TestChatCompletionResolvesAliasOncePerRequest(t *testing.T) {
	provider := aliasChatProvider()
	resolver := &countingAliasResolver{
		source: "balanced-chat",
		targets: []core.ModelSelector{
			{Provider: "test", Model: "gpt-4"},
			{Provider: "test", Model: "gpt-3.5-turbo"},
		},
	}
	srv := New(provider, &Config{MasterKey: "master-key", ModelResolver: resolver})

	for i := 0; i < 2; i++ {
		before := resolver.callCount()
		rec := postAliasChat(t, srv, "balanced-chat", fmt.Sprintf("rr-%d", i+1))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200: %s", i+1, rec.Code, rec.Body.String())
		}
		if got := resolver.callCount() - before; got != 1 {
			t.Errorf("request %d consulted the model resolver %d times, want 1", i+1, got)
		}
	}

	if want := []string{"test/gpt-4", "test/gpt-3.5-turbo"}; !reflect.DeepEqual(provider.chatCalls, want) {
		t.Fatalf("upstream calls = %v, want %v: round robin must alternate targets", provider.chatCalls, want)
	}
}

// Reusing the cached resolution must not skip the access check: the resolve call
// was also where the resolved model got authorized. Several layers re-validate
// the same selector on the cached path, so the invariant is that the check runs
// (for the resolved target), not how many times.
func TestChatCompletionAuthorizesReusedResolution(t *testing.T) {
	provider := aliasChatProvider()
	resolver := &countingAliasResolver{
		source:  "balanced-chat",
		targets: []core.ModelSelector{{Provider: "test", Model: "gpt-4"}},
	}
	authorizer := &recordingAuthorizer{}
	srv := New(provider, &Config{MasterKey: "master-key", ModelResolver: resolver, ModelAuthorizer: authorizer})

	rec := postAliasChat(t, srv, "balanced-chat", "authz-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := authorizer.count(); got < 1 {
		t.Fatalf("authorizations = %d, want the resolved model to be authorized", got)
	}
	if got := authorizer.validated[0]; got != "test/gpt-4" {
		t.Fatalf("authorized model = %q, want the resolved target", got)
	}
	if got := resolver.callCount(); got != 1 {
		t.Fatalf("resolver calls = %d, want 1", got)
	}
}

// A content-aware alias (adaptive, plugin) still resolves twice: the second
// resolution is what lets its selector see the request body summary, and that
// trade is the reason the pipeline resolves after the body is parsed at all.
func TestChatCompletionReResolvesContentAwareAlias(t *testing.T) {
	provider := aliasChatProvider()
	resolver := &countingAliasResolver{
		source:       "balanced-chat",
		contentAware: true,
		targets: []core.ModelSelector{
			{Provider: "test", Model: "gpt-4"},
			{Provider: "test", Model: "gpt-3.5-turbo"},
		},
	}
	srv := New(provider, &Config{MasterKey: "master-key", ModelResolver: resolver})

	rec := postAliasChat(t, srv, "balanced-chat", "content-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := resolver.callCount(); got != 2 {
		t.Fatalf("resolver calls = %d, want 2 for a content-aware alias", got)
	}
	if len(provider.chatCalls) != 1 {
		t.Fatalf("upstream calls = %v, want a single attempt", provider.chatCalls)
	}
}
