package virtualmodels

import (
	"context"
	"testing"
)

// T13-C: the lock flag is stored state, so it has to survive a roundtrip on
// both engines — including an update that leaves it untouched.
func TestStore_RoundTripLockedFlag(t *testing.T) {
	runStoreSuite(t, func(t *testing.T, store Store) {
		ctx := context.Background()
		locked := VirtualModel{
			Source:  "smart",
			Targets: []Target{{Provider: "openai", Model: "gpt-4o"}},
			Locked:  true,
			Enabled: true,
		}
		open := VirtualModel{
			Source:  "plain",
			Targets: []Target{{Provider: "openai", Model: "gpt-4o"}},
			Enabled: true,
		}
		if err := store.Upsert(ctx, locked); err != nil {
			t.Fatalf("Upsert(locked) error = %v", err)
		}
		if err := store.Upsert(ctx, open); err != nil {
			t.Fatalf("Upsert(open) error = %v", err)
		}

		got, err := store.Get(ctx, "smart")
		if err != nil {
			t.Fatalf("Get(smart) error = %v", err)
		}
		if !got.Locked {
			t.Fatalf("Get(smart).Locked = false, want true")
		}
		got, err = store.Get(ctx, "plain")
		if err != nil {
			t.Fatalf("Get(plain) error = %v", err)
		}
		if got.Locked {
			t.Fatalf("Get(plain).Locked = true, want false")
		}

		// An unrelated edit on a locked row must not clear the flag.
		locked.Description = "renamed description"
		if err := store.Upsert(ctx, locked); err != nil {
			t.Fatalf("Upsert(locked, description) error = %v", err)
		}
		found := false
		rows, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		for _, row := range rows {
			if row.Source == "smart" {
				found = true
				if !row.Locked {
					t.Fatalf("List() smart row = %#v, want Locked true", row)
				}
			}
		}
		if !found {
			t.Fatalf("List() = %#v, want the smart row", rows)
		}

		// And unlocking round-trips too.
		locked.Locked = false
		if err := store.Upsert(ctx, locked); err != nil {
			t.Fatalf("Upsert(unlock) error = %v", err)
		}
		if got, err = store.Get(ctx, "smart"); err != nil {
			t.Fatalf("Get(smart) after unlock error = %v", err)
		}
		if got.Locked {
			t.Fatalf("Get(smart) after unlock = true, want false")
		}
	})
}
