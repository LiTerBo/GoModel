package pricingoverrides

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/enterpilot/gomodel/internal/storage/mongotest"
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
		body(t, store, db)
	})
}

// runStoreSuite exercises behaviour every Store implementation owes its
// callers, against each backend available in this environment.
func runStoreSuite(t *testing.T, body func(t *testing.T, store Store)) {
	t.Helper()
	sqlxtest.Run(t, func(t *testing.T, db sqlx.DB) {
		store, err := NewSQLStore(context.Background(), db)
		if err != nil {
			t.Fatalf("NewSQLStore: %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		body(t, store)
	})
	mongotest.Run(t, func(t *testing.T, db *mongo.Database) {
		store, err := NewMongoDBStore(db)
		if err != nil {
			t.Fatalf("NewMongoDBStore: %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		body(t, store)
	})
}

func TestSQLStoreStoresPricingWithoutCurrency(t *testing.T) {
	runSQLStoreTest(t, func(t *testing.T, store *SQLStore, db sqlx.DB) {
		ctx := context.Background()

		if err := store.Upsert(ctx, Override{
			Selector: "openai/gpt-4o",
			Pricing:  Pricing{InputPerMtok: new(1.25)},
		}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		var rawPricing []byte
		err := db.QueryRow(ctx,
			`SELECT pricing FROM model_pricing_overrides WHERE selector = ?`, "openai/gpt-4o").
			Scan(&rawPricing)
		if err != nil {
			t.Fatalf("read pricing JSON: %v", err)
		}
		// An absent currency must stay absent in storage rather than being
		// persisted as an empty string.
		if strings.Contains(string(rawPricing), "currency") {
			t.Errorf("pricing JSON = %s, did not expect currency field", rawPricing)
		}

		overrides, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(overrides) != 1 {
			t.Fatalf("len(overrides) = %d, want 1", len(overrides))
		}
		if overrides[0].ProviderName != "openai" || overrides[0].Model != "gpt-4o" {
			t.Errorf("stored parts = (%q, %q), want (openai, gpt-4o)",
				overrides[0].ProviderName, overrides[0].Model)
		}
	})
}

func TestStoreUpsertReplacesPricing(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()

		if err := store.Upsert(ctx, Override{
			Selector: "openai/gpt-4o",
			Pricing:  Pricing{InputPerMtok: new(1.0)},
		}); err != nil {
			t.Fatalf("first Upsert: %v", err)
		}
		if err := store.Upsert(ctx, Override{
			Selector: "openai/gpt-4o",
			Pricing:  Pricing{InputPerMtok: new(2.0)},
		}); err != nil {
			t.Fatalf("second Upsert: %v", err)
		}

		overrides, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(overrides) != 1 {
			t.Fatalf("len(overrides) = %d, want 1 after re-upsert", len(overrides))
		}
		if overrides[0].Pricing.InputPerMtok == nil || *overrides[0].Pricing.InputPerMtok != 2.0 {
			t.Errorf("InputPerMtok = %v, want 2.0", overrides[0].Pricing.InputPerMtok)
		}
	})
}

func TestSQLStoreListIsOrderedBySelector(t *testing.T) {
	runSQLStoreTest(t, func(t *testing.T, store *SQLStore, _ sqlx.DB) {
		ctx := context.Background()

		for _, selector := range []string{"openai/gpt-4o", "anthropic/claude", "xai/grok"} {
			override := Override{Selector: selector, Pricing: Pricing{InputPerMtok: new(1.0)}}
			if err := store.Upsert(ctx, override); err != nil {
				t.Fatalf("Upsert %s: %v", selector, err)
			}
		}

		overrides, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := make([]string, 0, len(overrides))
		for _, override := range overrides {
			got = append(got, override.Selector)
		}
		want := []string{"anthropic/claude", "openai/gpt-4o", "xai/grok"}
		if len(got) != len(want) {
			t.Fatalf("selectors = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("selectors = %v, want %v", got, want)
			}
		}
	})
}

func TestStoreDeleteMissingReturnsNotFound(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		err := store.Delete(context.Background(), "absent/model")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Delete error = %v, want ErrNotFound", err)
		}
	})
}

func TestStoreDeleteRemovesOverride(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()

		if err := store.Upsert(ctx, Override{
			Selector: "openai/gpt-4o",
			Pricing:  Pricing{InputPerMtok: new(1.0)},
		}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		// Selectors are trimmed on the way in and out, so a padded delete must
		// still find the row.
		if err := store.Delete(ctx, "  openai/gpt-4o  "); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		overrides, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(overrides) != 0 {
			t.Errorf("len(overrides) = %d after delete, want 0", len(overrides))
		}
	})
}

func TestStorePersistenceTimeWindows(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()

		windows := []core.ModelPricingTimeWindow{{
			Label: "off_peak",
			UTCRanges: []core.ModelPricingUTCRange{
				{Days: []string{"mon", "tue", "wed", "thu", "fri"}, Start: "00:00", End: "01:00"},
				{Days: []string{"sat", "sun"}, Start: "00:00", End: "24:00"},
			},
			Pricing: core.ModelPricingTimeWindowRates{
				InputPerMtok:  coreFloat(0.15),
				OutputPerMtok: coreFloat(0.60),
			},
		}}

		if err := store.Upsert(ctx, Override{
			Selector: "deepseek/deepseek-v4-flash",
			Pricing: Pricing{
				InputPerMtok: coreFloat(0.3),
				OutputPerMtok: coreFloat(1.2),
				TimeWindows:  windows,
			},
		}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		overrides, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(overrides) != 1 {
			t.Fatalf("len(overrides) = %d, want 1", len(overrides))
		}
		got := overrides[0].Pricing.TimeWindows
		if len(got) != 1 {
			t.Fatalf("TimeWindows count = %d, want 1", len(got))
		}
		if got[0].Label != "off_peak" {
			t.Fatalf("window label = %q, want off_peak", got[0].Label)
		}
		ranges := got[0].UTCRanges
		if len(ranges) != 2 {
			t.Fatalf("UTCRanges = %d, want 2", len(ranges))
		}
		if len(ranges[0].Days) != 5 {
			t.Fatalf("range[0] days = %d, want 5", len(ranges[0].Days))
		}
		if len(ranges[1].Days) != 2 {
			t.Fatalf("range[1] days = %d, want 2", len(ranges[1].Days))
		}
		if ranges[0].Start != "00:00" || ranges[1].End != "24:00" {
			t.Fatalf("range bounds = %+v", ranges)
		}
		if got[0].Pricing.InputPerMtok == nil || *got[0].Pricing.InputPerMtok != 0.15 {
			t.Fatalf("window InputPerMtok = %v", got[0].Pricing.InputPerMtok)
		}
	})
}
