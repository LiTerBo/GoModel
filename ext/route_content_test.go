package ext

import (
	"context"
	"testing"
)

func TestRouteContentRoundTrip(t *testing.T) {
	content := &RouteContent{MessageCount: 4, UserTurns: 2, CurrentChars: 120, HasCode: true}
	ctx := WithRouteContent(context.Background(), content)
	if got := RouteContentFromContext(ctx); got != content {
		t.Fatalf("RouteContentFromContext() = %+v, want %+v", got, content)
	}
}

func TestRouteContentNilIsNoOp(t *testing.T) {
	ctx := WithRouteContent(context.Background(), nil)
	if got := RouteContentFromContext(ctx); got != nil {
		t.Fatalf("RouteContentFromContext() = %+v, want nil", got)
	}
	if got := RouteContentFromContext(context.Background()); got != nil {
		t.Fatalf("bare context returned %+v, want nil", got)
	}
}

func TestRequiredCapabilitiesRoundTrip(t *testing.T) {
	ctx := WithRequiredCapabilities(context.Background(), []string{"vision", "function_calling"})
	got := RequiredCapabilitiesFromContext(ctx)
	if len(got) != 2 || got[0] != "vision" || got[1] != "function_calling" {
		t.Fatalf("RequiredCapabilitiesFromContext() = %v", got)
	}
}

func TestRequiredCapabilitiesEmptyIsNoOp(t *testing.T) {
	ctx := WithRequiredCapabilities(context.Background(), nil)
	if got := RequiredCapabilitiesFromContext(ctx); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
	ctx = WithRequiredCapabilities(context.Background(), []string{})
	if got := RequiredCapabilitiesFromContext(ctx); got != nil {
		t.Fatalf("got %v, want nil for empty slice", got)
	}
}

// The zero values added to RouteRequest/RouteCandidate must behave exactly
// like the pre-extension contract: a selector built from an old RouteRequest
// literal still compiles and reads empty content without panics.
func TestRouteRequestZeroValueCompatibility(t *testing.T) {
	req := RouteRequest{Source: "smart", Candidates: []RouteCandidate{{Qualified: "p/m"}}}
	if req.Content != nil {
		t.Fatalf("Content should default to nil, got %+v", req.Content)
	}
	if req.RequiredCapabilities != nil {
		t.Fatalf("RequiredCapabilities should default to nil")
	}
	c := req.Candidates[0]
	if c.Capabilities != nil {
		t.Fatalf("Capabilities should default to nil")
	}
	// Reading a nil capability map must be safe and report "not capable".
	if c.Capabilities["vision"] {
		t.Fatal("nil map lookup claimed vision capability")
	}
}
