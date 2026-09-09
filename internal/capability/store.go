// Package capability persists operator-confirmed model capability verdicts.
//
// Confirming a capability today only mutates the in-memory registry
// (configMetadataOverrides), so a restart silently reverts confirmed models
// to their static/heuristic capability state. This package adds the missing
// durable layer, mirroring the pricingoverrides package: a Store (SQLite /
// MongoDB backends), a Service that loads persisted confirmations into the
// registry at startup (Refresh) and writes-through on operator confirmation
// (Confirm).
//
// The persisted source ("test" or "observed") matters as much as the
// capability itself: the dashboard encodes icon state from it, so a restart
// must be able to restore "verified by probe" versus "verified by traffic".
package capability

import (
	"context"
	"errors"
)

// Source values allowed in persisted confirmations. They mirror the
// core.CapSrc* operator-confirmed sources: probe results (test) and
// audit-log observations (observed). Static sources (registry, discovered,
// config, heuristic) are derived per process and never persisted here.
const (
	SourceTest     = "test"
	SourceObserved = "observed"
)

// ErrNotFound indicates a requested confirmation was not found.
var ErrNotFound = errors.New("capability confirmation not found")

// Confirmation is one persisted operator-confirmed capability verdict for a
// provider-qualified model. Rows are keyed by (provider, model, capability).
type Confirmation struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Capability string `json:"capability"` // "chat" | "embeddings" | "function_calling" | "vision" | ...
	Source     string `json:"source"`     // SourceTest | SourceObserved
	Value      bool   `json:"value"`      // true=supported, false=operator-confirmed unsupported
	CreatedAt  int64  `json:"created_at"` // unix seconds, set by the store when zero
}

// Store defines persistence operations for capability confirmations.
type Store interface {
	// Upsert inserts or replaces one capability confirmation.
	Upsert(ctx context.Context, c Confirmation) error
	// List returns all persisted confirmations ordered by provider, model,
	// capability.
	List(ctx context.Context) ([]Confirmation, error)
	// Delete removes a single capability confirmation.
	Delete(ctx context.Context, provider, model, capability string) error
	Close() error
}

// Confirmer is the admin-handler-facing surface of the service: confirm a
// set of capabilities for one provider/model with the given source.
type Confirmer interface {
	Confirm(ctx context.Context, provider, model string, caps map[string]bool, source string) error
}
