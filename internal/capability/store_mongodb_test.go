package capability

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/enterpilot/gomodel/internal/storage/mongotest"
)

// runMongoStoreTest opens the MongoDB store over a throwaway database, the
// counterpart of runSQLStoreTest.
func runMongoStoreTest(t *testing.T, body func(t *testing.T, store *MongoDBStore, db *mongo.Database)) {
	t.Helper()
	mongotest.Run(t, func(t *testing.T, database *mongo.Database) {
		store, err := NewMongoDBStore(database)
		if err != nil {
			t.Fatalf("NewMongoDBStore: %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		body(t, store, database)
	})
}

func TestMongoDBStoreRequiresDatabase(t *testing.T) {
	if _, err := NewMongoDBStore(nil); err == nil {
		t.Fatal("expected error for nil database")
	}
}

// Re-initialization must be idempotent: a restart (or a second subsystem over
// the same database) creates the same indexes again without colliding.
func TestMongoDBStoreInitIsIdempotent(t *testing.T) {
	mongotest.Run(t, func(t *testing.T, database *mongo.Database) {
		first, err := NewMongoDBStore(database)
		if err != nil {
			t.Fatalf("first NewMongoDBStore: %v", err)
		}
		second, err := NewMongoDBStore(database)
		if err != nil {
			t.Fatalf("second NewMongoDBStore: %v", err)
		}
		if err := first.Upsert(context.Background(), Confirmation{
			Provider: "local", Model: "my-llm", Capability: "vision", Value: true,
		}); err != nil {
			t.Fatalf("Upsert after re-init: %v", err)
		}
		if err := second.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
}

func TestMongoDBStoreUpsertAndList(t *testing.T) {
	runMongoStoreTest(t, func(t *testing.T, store *MongoDBStore, _ *mongo.Database) {
		ctx := context.Background()
		for _, c := range []Confirmation{
			{Provider: "openai", Model: "gpt-5", Capability: "vision", Source: SourceTest, Value: true},
			{Provider: "openai", Model: "gpt-5", Capability: "function_calling", Source: SourceObserved, Value: false},
			{Provider: "acme", Model: "zephyr", Capability: "chat", Source: SourceTest, Value: true},
		} {
			if err := store.Upsert(ctx, c); err != nil {
				t.Fatalf("Upsert(%s/%s/%s): %v", c.Provider, c.Model, c.Capability, err)
			}
		}

		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 3 {
			t.Fatalf("List returned %d rows, want 3: %#v", len(list), list)
		}
		// Ordered by provider, model, capability.
		want := []string{"acme", "openai", "openai"}
		for i, provider := range want {
			if list[i].Provider != provider {
				t.Fatalf("List[%d].Provider = %q, want %q (list=%#v)", i, list[i].Provider, provider, list)
			}
		}
		if list[1].Capability != "function_calling" || list[2].Capability != "vision" {
			t.Fatalf("capability order = %q, %q; want function_calling, vision", list[1].Capability, list[2].Capability)
		}
		if list[1].Value {
			t.Fatalf("operator-confirmed unsupported stored as supported: %#v", list[1])
		}
		if list[0].CreatedAt == 0 {
			t.Fatalf("CreatedAt = 0, want the store to stamp it")
		}
	})
}

func TestMongoDBStoreListEmpty(t *testing.T) {
	runMongoStoreTest(t, func(t *testing.T, store *MongoDBStore, _ *mongo.Database) {
		list, err := store.List(context.Background())
		if err != nil {
			t.Fatalf("List on an empty store: %v", err)
		}
		if len(list) != 0 {
			t.Fatalf("List on an empty store = %#v, want no rows", list)
		}
	})
}

func TestMongoDBStoreUpsertReplacesSameKey(t *testing.T) {
	runMongoStoreTest(t, func(t *testing.T, store *MongoDBStore, _ *mongo.Database) {
		ctx := context.Background()
		base := Confirmation{Provider: "openai", Model: "gpt-5", Capability: "vision", Source: SourceTest, Value: true}
		if err := store.Upsert(ctx, base); err != nil {
			t.Fatalf("first Upsert: %v", err)
		}
		flipped := base
		flipped.Value = false
		flipped.Source = SourceObserved
		if err := store.Upsert(ctx, flipped); err != nil {
			t.Fatalf("second Upsert: %v", err)
		}

		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("List returned %d rows, want the single key replaced: %#v", len(list), list)
		}
		if list[0].Value || list[0].Source != SourceObserved {
			t.Fatalf("stored = %#v, want the second write's value and source", list[0])
		}
	})
}

func TestMongoDBStoreDeleteRemovesConfirmation(t *testing.T) {
	runMongoStoreTest(t, func(t *testing.T, store *MongoDBStore, _ *mongo.Database) {
		ctx := context.Background()
		keep := Confirmation{Provider: "openai", Model: "gpt-5", Capability: "vision", Value: true}
		drop := Confirmation{Provider: "openai", Model: "gpt-5", Capability: "chat", Value: true}
		for _, c := range []Confirmation{keep, drop} {
			if err := store.Upsert(ctx, c); err != nil {
				t.Fatalf("Upsert: %v", err)
			}
		}
		if err := store.Delete(ctx, "openai", "gpt-5", "chat"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 || list[0].Capability != "vision" {
			t.Fatalf("List after delete = %#v, want only the vision row", list)
		}
		if err := store.Delete(ctx, "openai", "gpt-5", "chat"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Delete of a missing key = %v, want ErrNotFound", err)
		}
	})
}

func TestMongoDBStoreSourceRoundTrip(t *testing.T) {
	runMongoStoreTest(t, func(t *testing.T, store *MongoDBStore, _ *mongo.Database) {
		ctx := context.Background()
		if err := store.Upsert(ctx, Confirmation{
			Provider: "openai", Model: "gpt-5", Capability: "vision", Source: SourceObserved, Value: true,
		}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 || list[0].Source != SourceObserved {
			t.Fatalf("stored source = %#v, want %q", list, SourceObserved)
		}
	})
}

func TestMongoDBStoreRejectsIncompleteKey(t *testing.T) {
	runMongoStoreTest(t, func(t *testing.T, store *MongoDBStore, _ *mongo.Database) {
		if err := store.Upsert(context.Background(), Confirmation{Model: "gpt-5", Capability: "chat"}); err == nil {
			t.Fatal("expected an error for a confirmation without a provider")
		}
	})
}
