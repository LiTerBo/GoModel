package core

import (
	"context"
	"testing"
)

func TestRequestedModelNameContext(t *testing.T) {
	t.Parallel()

	if got := GetRequestedModelName(context.Background()); got != "" {
		t.Fatalf("GetRequestedModelName(empty) = %q, want empty", got)
	}

	// The stamped name is trimmed and read back verbatim.
	stamped := WithRequestedModelName(context.Background(), "  smart  ")
	if got := GetRequestedModelName(stamped); got != "smart" {
		t.Fatalf("GetRequestedModelName(stamped) = %q, want smart", got)
	}

	// The workflow resolution already on the context is the fallback.
	workflow := &Workflow{Resolution: &RequestModelResolution{
		Requested: NewRequestedModelSelector("smart", ""),
	}}
	fromWorkflow := WithWorkflow(context.Background(), workflow)
	if got := GetRequestedModelName(fromWorkflow); got != "smart" {
		t.Fatalf("GetRequestedModelName(workflow) = %q, want smart", got)
	}

	// An explicit stamp wins over the workflow value.
	both := WithRequestedModelName(fromWorkflow, "lite")
	if got := GetRequestedModelName(both); got != "lite" {
		t.Fatalf("GetRequestedModelName(stamped+workflow) = %q, want lite", got)
	}

	// An empty stamp does not mask the workflow fallback.
	cleared := WithRequestedModelName(fromWorkflow, "")
	if got := GetRequestedModelName(cleared); got != "smart" {
		t.Fatalf("GetRequestedModelName(cleared stamp) = %q, want smart from workflow", got)
	}
}
