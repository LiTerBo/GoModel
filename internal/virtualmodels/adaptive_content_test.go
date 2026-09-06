package virtualmodels

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/ext"
	"github.com/enterpilot/gomodel/internal/core"
)

// Adaptive requests must carry the request summary and required capabilities
// staged on the context by the gateway, and candidates must expose the
// catalog's capability flags as defensive copies (same rule as pricing).
func TestBalancer_AdaptiveRequestCarriesContentAndCapabilities(t *testing.T) {
	t.Parallel()
	catalog := balancingCatalog()
	// Give gpt-4o capabilities in the catalog.
	model := catalog.supported["openai/gpt-4o"]
	model.Metadata.Capabilities = map[string]bool{"vision": true, "function_calling": true}
	catalog.supported["openai/gpt-4o"] = model

	svc, err := NewService(newSQLVMStore(t), catalog, true)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	selector := &scriptedSelector{answer: "groq/llama"}
	svc.SetRouteSelector(selector)
	upsertAdaptive(t, svc)

	content := &ext.RouteContent{MessageCount: 6, UserTurns: 3, CurrentChars: 90, HasCode: true}
	ctx := ext.WithRouteContent(context.Background(), content)
	ctx = ext.WithRequiredCapabilities(ctx, []string{"vision"})

	if _, _, err := svc.resolveRequested(ctx, core.NewRequestedModelSelector("smart", ""), "", false, ""); err != nil {
		t.Fatalf("resolveRequested() error = %v", err)
	}

	req := selector.seen()[0]
	if req.Content == nil || *req.Content != *content {
		t.Fatalf("RouteRequest.Content = %+v, want %+v", req.Content, content)
	}
	if len(req.RequiredCapabilities) != 1 || req.RequiredCapabilities[0] != "vision" {
		t.Fatalf("RequiredCapabilities = %v, want [vision]", req.RequiredCapabilities)
	}
	for _, candidate := range req.Candidates {
		if candidate.Qualified == "openai/gpt-4o" {
			if !candidate.Capabilities["vision"] || !candidate.Capabilities["function_calling"] {
				t.Fatalf("candidate lost catalog capabilities: %+v", candidate.Capabilities)
			}
		}
		if candidate.Capabilities != nil {
			// A selector must never see the catalog's own map: mutating the
			// copy must not change what a later resolution reads.
			candidate.Capabilities["vision"] = false
		}
	}
	refreshed, _ := svc.catalog.LookupModel("openai/gpt-4o")
	if !refreshed.Metadata.Capabilities["vision"] {
		t.Fatal("selector-visible capabilities were not defensive copies")
	}
}

// With no content/capabilities staged (the legacy path: admin resolution,
// batch, startup), the request must simply carry zero values — old selectors
// keep working unchanged.
func TestBalancer_AdaptiveZeroContentBackwardCompatible(t *testing.T) {
	t.Parallel()
	svc := newBalancingService(t)
	selector := &scriptedSelector{answer: "groq/llama"}
	svc.SetRouteSelector(selector)
	upsertAdaptive(t, svc)

	resolvedModels(t, svc, "smart", 1)
	req := selector.seen()[0]
	if req.Content != nil {
		t.Fatalf("Content = %+v, want nil without staged summary", req.Content)
	}
	if req.RequiredCapabilities != nil {
		t.Fatalf("RequiredCapabilities = %v, want nil", req.RequiredCapabilities)
	}
}
