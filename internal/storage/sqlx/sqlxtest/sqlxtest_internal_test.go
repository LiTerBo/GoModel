package sqlxtest

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsTransientCatalogRace(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "tuple concurrently updated", err: &pgconn.PgError{Code: "XX000", Message: "tuple concurrently updated"}, want: true},
		{name: "deadlock", err: &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}, want: true},
		{name: "other internal error", err: &pgconn.PgError{Code: "XX000", Message: "something else broke"}, want: false},
		{name: "missing schema", err: &pgconn.PgError{Code: "3F000", Message: "schema does not exist"}, want: false},
		{name: "wrapped pg error", err: errors.Join(errors.New("exec"), &pgconn.PgError{Code: "XX000", Message: "tuple concurrently updated"}), want: true},
		{name: "non-pg error", err: errors.New("connection reset"), want: false},
		{name: "nil", err: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientCatalogRace(tt.err); got != tt.want {
				t.Fatalf("isTransientCatalogRace(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// Two parallel subtests must never share a database. Truncating long names to a
// fixed length made exactly that happen: both of these cut to
// "testroutecontentneeded_an_alias_scoped_t", so they shared one shared-cache
// in-memory SQLite database and raced over the same row.
func TestSanitizeIdentifierKeepsLongNamesApart(t *testing.T) {
	names := []string{
		"TestRouteContentNeeded/an_alias_scoped_to_another_user_path_is_not_this_caller's",
		"TestRouteContentNeeded/an_alias_scoped_to_this_user_path_applies",
		"TestRouteContentNeeded/a_model_that_is_not_an_alias_needs_nothing",
	}
	seen := make(map[string]string, len(names))
	for _, name := range names {
		got := sanitizeIdentifier(name)
		if len(got) > maxIdentifierBytes {
			t.Fatalf("sanitizeIdentifier(%q) = %q (%d bytes), want <= %d", name, got, len(got), maxIdentifierBytes)
		}
		if prev, ok := seen[got]; ok {
			t.Fatalf("sanitizeIdentifier collides: %q and %q both -> %q", prev, name, got)
		}
		seen[got] = name
	}
	if got := sanitizeIdentifier("TestShort"); got != "testshort" {
		t.Fatalf("sanitizeIdentifier(TestShort) = %q, want it unchanged", got)
	}
}
