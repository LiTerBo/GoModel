package virtualmodels

import (
	"context"
	"sort"
	"strings"

	"github.com/enterpilot/gomodel/internal/core"
)

// Resolve resolves raw model/provider inputs through the redirect table.
func (s *Service) Resolve(model, provider string) (Resolution, bool, error) {
	return s.resolveRequested(context.Background(), core.NewRequestedModelSelector(model, provider), "", false, "")
}

// resolveRequested resolves one requested selector through the redirect
// table. ctx is the request context when there is one: routing-strategy
// plugins read request metadata from it and are bounded by it.
func (s *Service) resolveRequested(ctx context.Context, requested core.RequestedModelSelector, userPath string, enforceUserPaths bool, sessionID string) (Resolution, bool, error) {
	selector, err := requested.Normalize()
	if err != nil {
		return Resolution{}, false, err
	}
	if requested.ExplicitProvider {
		return Resolution{Requested: selector, Resolved: selector}, false, nil
	}
	snap := s.snapshot()
	if entry, ok := snap.findRedirect(requested.Model, userPath, enforceUserPaths); ok {
		if resolved, ok := s.balancedResolution(ctx, snap, entry, sessionID); ok {
			return Resolution{Requested: selector, Resolved: resolved, Source: entry.vm.Source}, true, nil
		}
	}
	return Resolution{Requested: selector, Resolved: selector}, false, nil
}

// ResolveModel resolves a requested selector and returns the concrete selector
// chosen for execution. It does not consult user_paths; scoped redirects are
// applied by ResolveModelForUserPath on the request path.
func (s *Service) ResolveModel(requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	resolution, changed, err := s.resolveRequested(context.Background(), requested, "", false, "")
	if err != nil {
		return core.ModelSelector{}, false, err
	}
	return resolution.Resolved, changed, nil
}

// ResolveModelForUserPath resolves a requested selector honoring per-redirect
// user_paths against the effective request user path. A redirect scoped to
// user_paths the caller does not match falls through to the literal model name.
func (s *Service) ResolveModelForUserPath(ctx context.Context, requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	resolution, changed, err := s.resolveRequested(ctx, requested, core.UserPathFromContext(ctx), true, core.SessionIDFromContext(ctx))
	if err != nil {
		return core.ModelSelector{}, false, err
	}
	return resolution.Resolved, changed, nil
}

// RouteContentNeeded reports whether resolving requested depends on the request
// body summary: adaptive and plugin strategies choose their target from request
// content, so a resolution computed before the body was summarized has to be
// redone. Every other strategy picks from the target list alone, and redoing
// its resolution would rotate the round-robin cursor a second time for the same
// request.
func (s *Service) RouteContentNeeded(ctx context.Context, requested core.RequestedModelSelector) bool {
	if s == nil || requested.ExplicitProvider {
		return false
	}
	entry, ok := s.snapshot().findRedirect(requested.Model, core.UserPathFromContext(ctx), true)
	if !ok {
		return false
	}
	switch normalizeStrategy(entry.strategy) {
	case StrategyAdaptive, StrategyPlugin:
		return true
	default:
		return false
	}
}

// ResolveRefreshTarget returns a redirect target without consulting the current
// catalog so callers can refresh an unavailable target provider before normal
// resolution is retried.
func (s *Service) ResolveRefreshTarget(requested core.RequestedModelSelector) (core.ModelSelector, bool, error) {
	if s == nil || requested.ExplicitProvider {
		return core.ModelSelector{}, false, nil
	}
	name := strings.TrimSpace(requested.Model)
	if name == "" {
		return core.ModelSelector{}, false, nil
	}
	snap := s.snapshot()
	entry, ok := snap.redirects[name]
	if !ok || !entry.vm.Enabled {
		return core.ModelSelector{}, false, nil
	}
	// Any target's provider serves to refresh an unavailable upstream before the
	// balanced resolution retries, so the first declared concrete model is
	// sufficient.
	representative, ok := snap.representativeLeaf(entry)
	if !ok {
		return core.ModelSelector{}, false, nil
	}
	return representative.selector, true, nil
}

// Supports reports whether a redirect currently resolves to a concrete model.
func (s *Service) Supports(model string) bool {
	_, ok := s.snapshot().resolveRedirect(model, s.catalog, "", false)
	return ok
}

// GetProviderType returns the resolved provider type for a redirect, or empty
// when unresolved.
func (s *Service) GetProviderType(model string) string {
	if resolution, ok := s.snapshot().resolveRedirect(model, s.catalog, "", false); ok {
		return strings.TrimSpace(s.catalog.GetProviderType(resolution.Resolved.QualifiedModel()))
	}
	return ""
}

// ExposedModels returns enabled redirects projected as model-list entries.
func (s *Service) ExposedModels() []core.Model {
	return s.exposedModels("", false, nil, nil)
}

// ExposedModelsFiltered returns enabled redirects projected as model-list
// entries, filtered by the concrete target selector.
func (s *Service) ExposedModelsFiltered(allow func(core.ModelSelector) bool) []core.Model {
	return s.exposedModels("", false, allow, nil)
}

// ExposedModelsForUserPath is ExposedModelsFiltered plus per-redirect user_path
// scoping: a redirect carrying user_paths is hidden from callers it would not
// apply to, so a scoped alias is not listed (its name exposed) to callers
// outside its scope even though resolution would fall through for them.
func (s *Service) ExposedModelsForUserPath(userPath string, allow func(core.ModelSelector) bool) []core.Model {
	return s.exposedModels(userPath, true, allow, nil)
}

// ExposedModelsForUserPathNamed is ExposedModelsForUserPath plus an optional
// name predicate. A caller that may address a redirect by name sees it even
// when none of its targets is permitted: the alias, not the concrete model
// behind it, is the unit such a caller was authorized for. Target-level
// allowlists keep working through the selector predicate.
func (s *Service) ExposedModelsForUserPathNamed(userPath string, allow func(core.ModelSelector) bool, allowName func(string) bool) []core.Model {
	return s.exposedModels(userPath, true, allow, allowName)
}

func (s *Service) exposedModels(userPath string, enforceUserPaths bool, allow func(core.ModelSelector) bool, allowName func(string) bool) []core.Model {
	snap := s.snapshot()
	result := make([]core.Model, 0, len(snap.order))
	for _, source := range snap.order {
		entry := snap.redirects[source]
		if !entry.vm.Enabled {
			continue
		}
		if enforceUserPaths && len(entry.vm.UserPaths) > 0 && !userPathAllowed(userPath, entry.vm.UserPaths) {
			continue
		}
		// A caller that may address this redirect by name sees it without any
		// target having to be permitted; otherwise the legacy rule stays: expose
		// a load-balanced redirect when at least one concrete model behind it
		// (descending chains) is both catalog-supported and permitted.
		leafs := snap.leafTargets(entry, s.catalog)
		var chosen resolvedTarget
		var ok bool
		if allowName != nil && allowName(entry.vm.Source) {
			chosen, ok = representativeExposedTarget(leafs, nil)
		} else {
			chosen, ok = representativeExposedTarget(leafs, allow)
		}
		if !ok {
			continue
		}
		model, ok := s.catalog.LookupModel(chosen.qualified)
		if !ok || model == nil {
			continue
		}
		cloned := *model
		cloned.ID = entry.vm.Source
		// An alias answers as itself: the concrete model behind it stays off the
		// wire, so a caller neither learns which target serves it nor sees that
		// provenance change when the target disappears.
		cloned.OwnedBy = ""
		cloned.Created = 0
		if !entry.vm.CreatedAt.IsZero() {
			cloned.Created = entry.vm.CreatedAt.Unix()
		}
		result = append(result, cloned)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// representativeExposedTarget returns the first supported target the allow filter
// permits, used to project a redirect into a single model-list entry.
func representativeExposedTarget(supported []resolvedTarget, allow func(core.ModelSelector) bool) (resolvedTarget, bool) {
	for _, target := range supported {
		if allow == nil || allow(target.selector) {
			return target, true
		}
	}
	return resolvedTarget{}, false
}
