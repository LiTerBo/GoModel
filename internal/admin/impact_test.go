package admin

import (
	"reflect"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

// T13-A: the impact classifier answers, for one policy holder, how a virtual
// model's target-list change reaches it. Three outcomes matter, because a
// name-authorized holder keeps working (it silently follows the new targets)
// while a target-authorized holder may gain or lose the alias entirely. An
// unrestricted holder (empty allowlist) is unaffected in its usable set, and an
// unrelated holder is simply not reported.
func TestClassifyGrantImpact(t *testing.T) {
	openai := core.ModelSelector{Provider: "openai", Model: "gpt-4o"}
	anthropic := core.ModelSelector{Provider: "anthropic", Model: "claude-sonnet"}

	tests := []struct {
		name     string
		entries  []string
		current  []core.ModelSelector
		proposed []core.ModelSelector
		want     GrantImpact
	}{
		{
			name:    "name entry follows a target change",
			entries: []string{"smart"},
			current: []core.ModelSelector{openai},
			want:    GrantImpact{Change: ImpactFollow, MatchedBy: "name", Matched: []string{"smart"}},
		},
		{
			name:    "global entry follows and reports unrestricted name hit",
			entries: []string{"/"},
			current: []core.ModelSelector{openai},
			want:    GrantImpact{Change: ImpactFollow, MatchedBy: "name", Matched: []string{"/"}},
		},
		{
			name:    "provider wide entry hits through the current target",
			entries: []string{"openai/"},
			current: []core.ModelSelector{openai},
			proposed: []core.ModelSelector{
				openai,
			},
			want: GrantImpact{Change: ImpactPotential, MatchedBy: "selector", Matched: []string{"openai/"}},
		},
		{
			name:     "provider wide entry hit by the target being removed",
			entries:  []string{"openai/"},
			current:  []core.ModelSelector{openai},
			proposed: []core.ModelSelector{anthropic},
			want:     GrantImpact{Change: ImpactPotential, MatchedBy: "selector", Matched: []string{"openai/"}},
		},
		{
			name:     "provider wide entry hit by the target being added",
			entries:  []string{"anthropic/"},
			current:  []core.ModelSelector{openai},
			proposed: []core.ModelSelector{anthropic},
			want:     GrantImpact{Change: ImpactPotential, MatchedBy: "selector", Matched: []string{"anthropic/"}},
		},
		{
			name:     "exact selector entry hits the target",
			entries:  []string{"openai/gpt-4o"},
			current:  []core.ModelSelector{openai},
			proposed: []core.ModelSelector{anthropic},
			want:     GrantImpact{Change: ImpactPotential, MatchedBy: "selector", Matched: []string{"openai/gpt-4o"}},
		},
		{
			name:     "unrelated entry is not reported",
			entries:  []string{"groq/"},
			current:  []core.ModelSelector{openai},
			proposed: []core.ModelSelector{anthropic},
			want:     GrantImpact{Change: ImpactNone},
		},
		{
			name:     "empty allowlist is unrestricted, not potentially affected",
			entries:  nil,
			current:  []core.ModelSelector{openai},
			proposed: []core.ModelSelector{anthropic},
			want:     GrantImpact{Change: ImpactUnrestricted, MatchedBy: "unrestricted"},
		},
		{
			name:     "name hit wins over selector hits and matched entries are sorted and deduped",
			entries:  []string{"openai/", "smart", "openai/", "smart"},
			current:  []core.ModelSelector{openai},
			proposed: []core.ModelSelector{anthropic},
			want:     GrantImpact{Change: ImpactFollow, MatchedBy: "name", Matched: []string{"smart"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyGrantImpact("smart", tc.entries, tc.current, tc.proposed)
			if got.Change != tc.want.Change {
				t.Fatalf("Change = %q, want %q", got.Change, tc.want.Change)
			}
			if got.MatchedBy != tc.want.MatchedBy {
				t.Fatalf("MatchedBy = %q, want %q", got.MatchedBy, tc.want.MatchedBy)
			}
			if !reflect.DeepEqual(got.Matched, tc.want.Matched) {
				t.Fatalf("Matched = %v, want %v", got.Matched, tc.want.Matched)
			}
		})
	}
}

// A target change that leaves the target set identical still reports who
// follows the alias: the read-only inventory path ("who is authorized by this
// name right now") reuses the same classifier.
func TestClassifyGrantImpactUnchangedTargetsStillReportsFollowers(t *testing.T) {
	openai := core.ModelSelector{Provider: "openai", Model: "gpt-4o"}

	got := classifyGrantImpact("smart", []string{"smart"}, []core.ModelSelector{openai}, []core.ModelSelector{openai})
	if got.Change != ImpactFollow {
		t.Fatalf("Change = %q, want %q", got.Change, ImpactFollow)
	}
}
