package capability

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
)

// memStore is an in-memory Store double for service tests. It records what
// the service wrote so tests can assert the store-first write order (AD-2).
type memStore struct {
	mu    sync.Mutex
	items map[string]Confirmation // provider\0model\0capability -> row

	listErr   error
	upsertErr error // returned by every Upsert when set
}

func newMemStore() *memStore {
	return &memStore{items: make(map[string]Confirmation)}
}

func confirmationKey(provider, model, capability string) string {
	return provider + "\x00" + model + "\x00" + capability
}

func (s *memStore) Upsert(_ context.Context, c Confirmation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.items[confirmationKey(c.Provider, c.Model, c.Capability)] = c
	return nil
}

func (s *memStore) List(_ context.Context) ([]Confirmation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]Confirmation, 0, len(s.items))
	for _, c := range s.items {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return confirmationKey(out[i].Provider, out[i].Model, out[i].Capability) <
			confirmationKey(out[j].Provider, out[j].Model, out[j].Capability)
	})
	return out, nil
}

func (s *memStore) Delete(_ context.Context, provider, model, capability string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := confirmationKey(provider, model, capability)
	if _, ok := s.items[key]; !ok {
		return ErrNotFound
	}
	delete(s.items, key)
	return nil
}

func (s *memStore) Close() error { return nil }

// mergeCall records one MergeModelCapabilities invocation.
type mergeCall struct {
	provider string
	model    string
	caps     map[string]bool
	source   string
}

// memRegistry is a ModelRegistry double.
type memRegistry struct {
	mu    sync.Mutex
	calls []mergeCall
	ok    bool // value returned by MergeModelCapabilities
}

func newMemRegistry() *memRegistry { return &memRegistry{ok: true} }

func (r *memRegistry) MergeModelCapabilities(provider, model string, caps map[string]bool, source string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := make(map[string]bool, len(caps))
	for k, v := range caps {
		cloned[k] = v
	}
	r.calls = append(r.calls, mergeCall{provider: provider, model: model, caps: cloned, source: source})
	return r.ok
}

func TestRefreshEmptyStoreLoadsNothing(t *testing.T) {
	store := newMemStore()
	registry := newMemRegistry()
	service := NewService(store, registry)

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v, want nil for an empty store", err)
	}
	if len(registry.calls) != 0 {
		t.Errorf("registry merge calls = %d, want 0", len(registry.calls))
	}
}

func TestRefreshLoadsConfirmationsIntoRegistry(t *testing.T) {
	store := newMemStore()
	// Two capabilities for one model sharing a source must land in one merge
	// call; a different source must be a separate call so the per-capability
	// source attribution inside the registry stays exact.
	for _, c := range []Confirmation{
		{Provider: "local", Model: "my-llm", Capability: "function_calling", Source: SourceTest, Value: true},
		{Provider: "local", Model: "my-llm", Capability: "chat", Source: SourceTest, Value: true},
		{Provider: "other", Model: "embed-1", Capability: "embeddings", Source: SourceObserved, Value: true},
	} {
		if err := store.Upsert(context.Background(), c); err != nil {
			t.Fatalf("seed store: %v", err)
		}
	}
	registry := newMemRegistry()
	service := NewService(store, registry)

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(registry.calls) != 2 {
		t.Fatalf("registry merge calls = %d, want 2", len(registry.calls))
	}
	first := registry.calls[0]
	if first.provider != "local" || first.model != "my-llm" || first.source != SourceTest {
		t.Errorf("first call = %+v, want local/my-llm source=test", first)
	}
	if !first.caps["chat"] || !first.caps["function_calling"] {
		t.Errorf("first call caps = %v, want chat+function_calling merged into one call", first.caps)
	}
	second := registry.calls[1]
	if second.provider != "other" || second.model != "embed-1" || second.source != SourceObserved {
		t.Errorf("second call = %+v, want other/embed-1 source=observed", second)
	}
	if len(second.caps) != 1 || !second.caps["embeddings"] {
		t.Errorf("second call caps = %v, want embeddings only", second.caps)
	}
}

func TestRefreshStoreErrorPropagates(t *testing.T) {
	store := newMemStore()
	store.listErr = errors.New("disk on fire")
	registry := newMemRegistry()
	service := NewService(store, registry)

	if err := service.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh() error = nil, want the store error to propagate")
	}
	if len(registry.calls) != 0 {
		t.Errorf("registry merge calls = %d, want 0 after a store error", len(registry.calls))
	}
}

// TestRefreshUnknownModelDoesNotFailStartup pins the asymmetry with Confirm:
// startup must keep working when a persisted confirmation names a model the
// registry no longer knows; the row stays and re-applies if it returns.
func TestRefreshUnknownModelDoesNotFailStartup(t *testing.T) {
	store := newMemStore()
	if err := store.Upsert(context.Background(), Confirmation{
		Provider: "ghost", Model: "vanished", Capability: "chat", Source: SourceTest, Value: true,
	}); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	registry := newMemRegistry()
	registry.ok = false
	service := NewService(store, registry)

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v, want nil when the registry rejects the model", err)
	}
	if len(registry.calls) != 1 {
		t.Errorf("registry merge calls = %d, want 1 (the attempt must still happen)", len(registry.calls))
	}
}

func TestConfirmWritesStoreThenRegistry(t *testing.T) {
	store := newMemStore()
	registry := newMemRegistry()
	service := NewService(store, registry)

	caps := map[string]bool{"chat": true, "function_calling": false}
	if err := service.Confirm(context.Background(), "local", "my-llm", caps, SourceTest); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}

	if len(store.items) != 2 {
		t.Fatalf("store rows = %d, want 2", len(store.items))
	}
	chat := store.items[confirmationKey("local", "my-llm", "chat")]
	if !chat.Value || chat.Source != SourceTest {
		t.Errorf("chat row = %+v, want value=true source=test", chat)
	}
	fc := store.items[confirmationKey("local", "my-llm", "function_calling")]
	if fc.Value {
		t.Errorf("function_calling row = %+v, want the confirmed negative (value=false)", fc)
	}

	if len(registry.calls) != 1 {
		t.Fatalf("registry merge calls = %d, want 1 (single merge with the full cap set)", len(registry.calls))
	}
	call := registry.calls[0]
	if call.provider != "local" || call.model != "my-llm" || call.source != SourceTest {
		t.Errorf("merge call = %+v, want local/my-llm source=test", call)
	}
	if len(call.caps) != 2 || call.caps["chat"] != true || call.caps["function_calling"] != false {
		t.Errorf("merge caps = %v, want the confirmed set including the negative", call.caps)
	}
}

func TestConfirmEmptyCapsIsNoOp(t *testing.T) {
	store := newMemStore()
	registry := newMemRegistry()
	service := NewService(store, registry)

	if err := service.Confirm(context.Background(), "local", "my-llm", nil, SourceTest); err != nil {
		t.Fatalf("Confirm(nil) error = %v, want nil", err)
	}
	if err := service.Confirm(context.Background(), "local", "my-llm", map[string]bool{}, SourceTest); err != nil {
		t.Fatalf("Confirm(empty) error = %v, want nil", err)
	}
	if len(store.items) != 0 {
		t.Errorf("store rows = %d, want 0", len(store.items))
	}
	if len(registry.calls) != 0 {
		t.Errorf("registry merge calls = %d, want 0", len(registry.calls))
	}
}

// TestConfirmStoreErrorStopsBeforeRegistry is the AD-2 acceptance test: when
// persistence fails, the registry must stay untouched so memory and disk do
// not diverge (the operator sees an error and can retry).
func TestConfirmStoreErrorStopsBeforeRegistry(t *testing.T) {
	store := newMemStore()
	store.upsertErr = errors.New("disk on fire")
	registry := newMemRegistry()
	service := NewService(store, registry)

	err := service.Confirm(context.Background(), "local", "my-llm", map[string]bool{"chat": true}, SourceTest)
	if err == nil {
		t.Fatal("Confirm() error = nil, want the store error to propagate")
	}
	if !strings.Contains(err.Error(), "disk on fire") {
		t.Errorf("error = %v, want it to wrap the store failure", err)
	}
	if len(registry.calls) != 0 {
		t.Errorf("registry merge calls = %d, want 0 after a store error", len(registry.calls))
	}
}

func TestConfirmRejectsUnknownSource(t *testing.T) {
	store := newMemStore()
	registry := newMemRegistry()
	service := NewService(store, registry)

	err := service.Confirm(context.Background(), "local", "my-llm", map[string]bool{"chat": true}, "heuristic")
	if err == nil {
		t.Fatal("Confirm(source=heuristic) error = nil, want a validation error")
	}
	if len(store.items) != 0 {
		t.Errorf("store rows = %d, want 0 after a rejected source", len(store.items))
	}
	if len(registry.calls) != 0 {
		t.Errorf("registry merge calls = %d, want 0 after a rejected source", len(registry.calls))
	}
}

func TestConfirmDefaultsEmptySourceToTest(t *testing.T) {
	store := newMemStore()
	registry := newMemRegistry()
	service := NewService(store, registry)

	if err := service.Confirm(context.Background(), "local", "my-llm", map[string]bool{"chat": true}, ""); err != nil {
		t.Fatalf("Confirm(source=\"\") error = %v, want nil", err)
	}
	chat := store.items[confirmationKey("local", "my-llm", "chat")]
	if chat.Source != SourceTest {
		t.Errorf("row source = %q, want %q", chat.Source, SourceTest)
	}
}

// TestConfirmUnknownModelStillPersists documents the deliberate asymmetry
// with Refresh: the interactive path reports the registry rejection to the
// caller, but the durable confirmation stays so a later restart (or the
// model appearing) re-applies it.
func TestConfirmUnknownModelStillPersists(t *testing.T) {
	store := newMemStore()
	registry := newMemRegistry()
	registry.ok = false
	service := NewService(store, registry)

	err := service.Confirm(context.Background(), "ghost", "vanished", map[string]bool{"chat": true}, SourceTest)
	if err == nil {
		t.Fatal("Confirm() error = nil, want the registry rejection to surface")
	}
	if len(store.items) != 1 {
		t.Errorf("store rows = %d, want 1 (persisted despite the rejection)", len(store.items))
	}
}
