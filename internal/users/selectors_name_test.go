package users

import "testing"

func TestMatchesName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		allowed []string
		model   string
		want    bool
	}{
		{name: "empty allowlist", allowed: nil, model: "smart", want: false},
		{name: "empty name", allowed: []string{"smart"}, model: "", want: false},
		{name: "exact alias", allowed: []string{"smart"}, model: "smart", want: true},
		{name: "exact alias trimmed", allowed: []string{" smart "}, model: " smart ", want: true},
		{name: "sibling alias", allowed: []string{"smart"}, model: "lite", want: false},
		{name: "global", allowed: []string{"/"}, model: "smart", want: true},
		{name: "star is not a canonical entry", allowed: []string{"*"}, model: "smart", want: false},
		{name: "provider wide needs a provider", allowed: []string{"openai/*"}, model: "smart", want: false},
		{name: "provider model needs the resolved path", allowed: []string{"openai/gpt-4o"}, model: "gpt-4o", want: false},
		{name: "qualified name matches qualified entry", allowed: []string{"openai/gpt-4o"}, model: "openai/gpt-4o", want: true},
		{name: "any of several", allowed: []string{"openai/*", "smart"}, model: "smart", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchesName(tc.allowed, tc.model); got != tc.want {
				t.Fatalf("MatchesName(%v, %q) = %v, want %v", tc.allowed, tc.model, got, tc.want)
			}
		})
	}
}
