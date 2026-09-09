package capability

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ModelRegistry is the slice of the provider registry the service needs:
// operator-confirmed capability verdicts ride the config-override channel.
// *providers.ModelRegistry satisfies it via MergeModelCapabilities.
type ModelRegistry interface {
	MergeModelCapabilities(providerName, modelID string, caps map[string]bool, source string) bool
}

// Service persists operator-confirmed capabilities and replays them into the
// registry. Refresh runs once at startup (and may run again later); Confirm
// is the interactive write path behind PUT /admin/models/capabilities.
type Service struct {
	store    Store
	registry ModelRegistry

	// mu serializes Refresh and Confirm against each other. Both touch the
	// same store rows and the same registry entries; the registry and SQL
	// store are individually safe for concurrent use, but the two-step
	// write order (store first, registry second — AD-2) must not interleave.
	mu sync.Mutex
}

// NewService builds a service over the given store and registry. Both must
// be non-nil.
func NewService(store Store, registry ModelRegistry) *Service {
	return &Service{store: store, registry: registry}
}

// Refresh loads every persisted confirmation into the registry. It is the
// startup hook that makes confirmations survive a restart.
//
// Failure semantics: a store error aborts the refresh (startup fails loudly
// — the operator asked for these capabilities to be authoritative), but a
// registry rejection (unknown provider/model, e.g. the model disappeared
// from its provider inventory) is skipped: the row stays persisted and
// re-applies on the next refresh if the model returns. Startup must not
// depend on upstream inventory.
func (s *Service) Refresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	confirmations, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("load capability confirmations: %w", err)
	}

	for _, merge := range groupByTargetAndSource(confirmations) {
		s.registry.MergeModelCapabilities(merge.provider, merge.model, merge.caps, merge.source)
	}
	return nil
}

// Confirm persists one operator confirmation and applies it to the registry.
//
// Write order is fixed (AD-2): store first — the only step that can fail —
// then the registry merge, which is an in-memory operation. When the store
// write fails, the registry stays untouched so memory and disk never
// diverge; the caller surfaces the error and the operator retries.
//
// A registry rejection (unknown provider/model) still persists the row: the
// error is reported to the caller, but the confirmation is durable and
// re-applies if the model appears later. This mirrors Refresh's tolerance
// while keeping the interactive path honest about what happened.
func (s *Service) Confirm(ctx context.Context, provider, model string, caps map[string]bool, source string) error {
	if len(caps) == 0 {
		return nil
	}

	// Validate the source up front and stamp rows before any I/O, so a bad
	// source cannot leave a half-written confirmation set behind.
	rows := make([]Confirmation, 0, len(caps))
	for _, capability := range sortedCapabilities(caps) {
		row, err := normalizeConfirmation(Confirmation{
			Provider:   provider,
			Model:      model,
			Capability: capability,
			Source:     source,
			Value:      caps[capability],
		})
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, row := range rows {
		if err := s.store.Upsert(ctx, row); err != nil {
			return fmt.Errorf("persist capability confirmation: %w", err)
		}
	}
	if !s.registry.MergeModelCapabilities(provider, model, caps, rows[0].Source) {
		return fmt.Errorf("apply capability confirmation: unknown provider/model %s/%s", provider, model)
	}
	return nil
}

// List returns every persisted confirmation (debug/admin surface).
func (s *Service) List(ctx context.Context) ([]Confirmation, error) {
	return s.store.List(ctx)
}

// targetSource groups confirmations by provider/model/source so each merge
// call carries one source attribution — merging across sources would let the
// last call's source silently re-label earlier capabilities.
type targetSource struct {
	provider string
	model    string
	source   string
	caps     map[string]bool
}

// targetSourceKey is the comparable identity of a targetSource group.
type targetSourceKey struct {
	provider string
	model    string
	source   string
}

func groupByTargetAndSource(confirmations []Confirmation) []targetSource {
	groups := make(map[targetSourceKey]*targetSource)
	keys := make([]targetSourceKey, 0, len(confirmations))
	for _, c := range confirmations {
		key := targetSourceKey{provider: c.Provider, model: c.Model, source: c.Source}
		group, ok := groups[key]
		if !ok {
			group = &targetSource{
				provider: c.Provider,
				model:    c.Model,
				source:   c.Source,
				caps:     make(map[string]bool),
			}
			groups[key] = group
			keys = append(keys, key)
		}
		group.caps[c.Capability] = c.Value
	}
	// Deterministic order for tests and logs: sort by provider/model/source.
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].provider != keys[j].provider {
			return keys[i].provider < keys[j].provider
		}
		if keys[i].model != keys[j].model {
			return keys[i].model < keys[j].model
		}
		return keys[i].source < keys[j].source
	})
	merged := make([]targetSource, 0, len(keys))
	for _, key := range keys {
		merged = append(merged, *groups[key])
	}
	return merged
}

func sortedCapabilities(caps map[string]bool) []string {
	names := make([]string, 0, len(caps))
	for name := range caps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
