package admin

import (
	"slices"
	"strings"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/modelselectors"
	"github.com/enterpilot/gomodel/internal/users"
)

// ImpactChange classifies how a virtual model's target change reaches one
// policy holder (an API key or a user path policy).
type ImpactChange string

const (
	// ImpactFollow marks a holder that is authorized by the alias name itself.
	// Its usable set barely notices the change: the alias keeps answering, but
	// it silently points at different concrete models afterwards.
	ImpactFollow ImpactChange = "follow"
	// ImpactPotential marks a holder authorized through target selectors that
	// intersect the change, so it may gain or lose the alias. Deciding which
	// one requires resolving the proposed definition against the catalog, which
	// the coarse pass deliberately does not do.
	ImpactPotential ImpactChange = "potential"
	// ImpactUnrestricted marks a holder with no model allowlist at all: the
	// alias's membership in its usable set never came from the alias targets.
	ImpactUnrestricted ImpactChange = "unrestricted"
	// ImpactNone marks a holder the change does not reach.
	ImpactNone ImpactChange = "none"
)

// GrantImpact is the verdict for one policy holder. Matched lists the allowlist
// entries that produced it, sorted and deduplicated.
type GrantImpact struct {
	Change    ImpactChange `json:"change"`
	MatchedBy string       `json:"matched_by,omitempty"` // name | selector | unrestricted
	Matched   []string     `json:"matched,omitempty"`
}

// classifyGrantImpact reports how a target-set change reaches one allowlist.
// current and proposed hold the concrete selectors the alias points at before
// and after the edit; passing the same set twice answers "who follows this
// alias right now", which the read-only inventory path uses.
//
// It mirrors the authorization semantics exactly by delegating to users.Matches
// and users.MatchesName instead of re-implementing selector parsing: an entry
// that admits a target selector here admits it at request time too.
func classifyGrantImpact(name string, entries []string, current, proposed []core.ModelSelector) GrantImpact {
	if len(entries) == 0 {
		return GrantImpact{Change: ImpactUnrestricted, MatchedBy: "unrestricted"}
	}
	// A name-shaped entry admits the alias no matter what it points at, so it
	// is reported first and on its own: this is the holder the operator cannot
	// see from a target list.
	if users.MatchesName(entries, name) {
		return GrantImpact{Change: ImpactFollow, MatchedBy: "name", Matched: nameEntries(entries, name)}
	}
	if matched := coveringEntries(entries, current, proposed); len(matched) > 0 {
		return GrantImpact{Change: ImpactPotential, MatchedBy: "selector", Matched: matched}
	}
	return GrantImpact{Change: ImpactNone}
}

// nameEntries returns the name-shaped entries that admit name, mirroring
// users.MatchesName (a global entry or the exact name).
func nameEntries(entries []string, name string) []string {
	name = strings.TrimSpace(name)
	matched := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if modelselectors.IsGlobal(entry) || entry == name {
			matched = append(matched, entry)
		}
	}
	return sortedUnique(matched)
}

// coveringEntries returns the entries that admit at least one selector from
// either side of the change, so a removed target and an added target both show
// up as the reason this holder is in scope.
func coveringEntries(entries []string, current, proposed []core.ModelSelector) []string {
	selectors := make([]core.ModelSelector, 0, len(current)+len(proposed))
	selectors = append(selectors, current...)
	selectors = append(selectors, proposed...)
	if len(selectors) == 0 {
		return nil
	}
	matched := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		admitted := false
		for _, selector := range selectors {
			if users.Matches([]string{entry}, selector) {
				admitted = true
				break
			}
		}
		if admitted {
			matched = append(matched, entry)
		}
	}
	return sortedUnique(matched)
}

// sortedUnique sorts and deduplicates allowlist entries for stable reporting.
func sortedUnique(entries []string) []string {
	slices.Sort(entries)
	return slices.Compact(entries)
}
