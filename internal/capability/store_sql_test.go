package capability

import (
	"context"
	"errors"
	"testing"

	"github.com/enterpilot/gomodel/internal/storage/sqlx"
	"github.com/enterpilot/gomodel/internal/storage/sqlx/sqlxtest"
)

func runSQLStoreTest(t *testing.T, body func(t *testing.T, store *SQLStore, db sqlx.DB)) {
	t.Helper()
	sqlxtest.Run(t, func(t *testing.T, db sqlx.DB) {
		store, err := NewSQLStore(context.Background(), db)
		if err != nil {
			t.Fatalf("NewSQLStore: %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		body(t, store, db)
	})
}

// runStoreSuite exercises behaviour every Store implementation owes its
// callers, against each backend available in this environment (SQLite via
// sqlxtest; MongoDB lands with a Mongo store implementation).
func runStoreSuite(t *testing.T, body func(t *testing.T, store Store)) {
	t.Helper()
	runSQLStoreTest(t, func(t *testing.T, store *SQLStore, _ sqlx.DB) {
		body(t, store)
	})
}

func TestStoreUpsertAndList(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()

		if err := store.Upsert(ctx, Confirmation{
			Provider:   "openai",
			Model:      "gpt-4o",
			Capability: "function_calling",
			Source:     SourceTest,
			Value:      true,
		}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("len(list) = %d, want 1", len(list))
		}
		got := list[0]
		if got.Provider != "openai" || got.Model != "gpt-4o" || got.Capability != "function_calling" {
			t.Errorf("confirmation = %+v, want provider=openai model=gpt-4o capability=function_calling", got)
		}
		if got.Source != SourceTest {
			t.Errorf("Source = %q, want %q", got.Source, SourceTest)
		}
		if got.Value != true {
			t.Errorf("Value = %v, want true", got.Value)
		}
		if got.CreatedAt == 0 {
			t.Errorf("CreatedAt = 0, want set by store")
		}
	})
}

func TestStoreListEmpty(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		list, err := store.List(context.Background())
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 0 {
			t.Fatalf("len(list) = %d, want 0 on empty table", len(list))
		}
	})
}

func TestStoreUpsertReplacesSameKey(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()
		key := Confirmation{Provider: "openai", Model: "gpt-4o", Capability: "function_calling"}

		first := key
		first.Source = SourceTest
		first.Value = true
		if err := store.Upsert(ctx, first); err != nil {
			t.Fatalf("first Upsert: %v", err)
		}
		second := key
		second.Source = SourceObserved
		second.Value = false
		if err := store.Upsert(ctx, second); err != nil {
			t.Fatalf("second Upsert: %v", err)
		}

		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("len(list) = %d, want 1 after re-upsert", len(list))
		}
		if list[0].Source != SourceObserved || list[0].Value != false {
			t.Errorf("confirmation = %+v, want source=observed value=false", list[0])
		}
	})
}

func TestStoreDeleteRemovesConfirmation(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()
		if err := store.Upsert(ctx, Confirmation{
			Provider:   "openai",
			Model:      "gpt-4o",
			Capability: "chat",
			Source:     SourceTest,
			Value:      true,
		}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		if err := store.Delete(ctx, "openai", "gpt-4o", "chat"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("len(list) = %d after delete, want 0", len(list))
		}
	})
}

func TestStoreDeleteMissingReturnsNotFound(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		err := store.Delete(context.Background(), "absent", "model", "chat")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Delete error = %v, want ErrNotFound", err)
		}
	})
}

// TestStoreSourceRoundTrip is the A-3 acceptance criterion: the persisted
// source must survive a write/read cycle intact for every sanctioned source,
// because the dashboard icon state depends on it after a restart.
func TestStoreSourceRoundTrip(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()
		for _, source := range []string{SourceTest, SourceObserved} {
			if err := store.Upsert(ctx, Confirmation{
				Provider:   "local",
				Model:      "my-llm",
				Capability: "chat",
				Source:     source,
				Value:      true,
			}); err != nil {
				t.Fatalf("Upsert source=%s: %v", source, err)
			}
			list, err := store.List(ctx)
			if err != nil {
				t.Fatalf("List after source=%s: %v", source, err)
			}
			var got string
			for _, c := range list {
				if c.Capability == "chat" {
					got = c.Source
				}
			}
			if got != source {
				t.Errorf("source round trip = %q, want %q", got, source)
			}
		}
	})
}
