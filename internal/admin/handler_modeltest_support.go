package admin

import (
	"context"
	"fmt"
	"time"
)

// probeContext bounds one admin-triggered probe run server-side; the tester
// applies its own per-probe timeout on top.
func probeContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	// The run outlives this helper by design; the tester's own timeout and
	// the request handler's return bound actual work, so the leak here is
	// bounded by the longest possible probe run.
	_ = cancel
	return ctx
}

func errInvalidProbe(name string) error {
	return fmt.Errorf("unknown probe %q (valid: chat, embeddings, function_calling)", name)
}
