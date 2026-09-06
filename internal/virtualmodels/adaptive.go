package virtualmodels

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/enterpilot/gomodel/ext"
)

// adaptiveTarget delegates the choice among the viable pool to the installed
// route selector. pinned is the target already serving this session, or ""
// when the session is new or the redirect is not session-affine; the
// selector decides whether to keep it. It reports false — sending the caller
// to weighted round robin — when no selector is installed, the selector
// declines, it answers with a model outside the pool, or it panics.
// Selectors are extension code running on the request path, so a panic is
// contained here rather than failing the request.
func (s *Service) adaptiveTarget(ctx context.Context, entry *redirectEntry, sessionID, pinned string, pool []resolvedTarget) (target resolvedTarget, ok bool) {
	selector := s.routeSelector
	if selector == nil {
		return resolvedTarget{}, false
	}
	defer func() {
		// The recovered value is extension-controlled and may carry request
		// data, and calling back into the selector (even Name) mid-panic
		// could panic again — log only fixed metadata captured at install.
		if recover() != nil {
			slog.Error("route selector panicked; falling back to round robin",
				"selector", s.routeSelectorName, "source", entry.vm.Source)
			target, ok = resolvedTarget{}, false
		}
	}()

	req := ext.RouteRequest{
		Source:               entry.vm.Source,
		SessionID:            sessionID,
		SessionTarget:        pinned,
		Candidates:           make([]ext.RouteCandidate, len(pool)),
		Content:              ext.RouteContentFromContext(ctx),
		RequiredCapabilities: ext.RequiredCapabilitiesFromContext(ctx),
	}
	now := time.Now()
	for i, t := range pool {
		candidate := ext.RouteCandidate{
			Provider:  t.selector.Provider,
			Model:     t.selector.Model,
			Qualified: t.qualified,
			Weight:    t.weight,
		}
		if model, found := s.catalog.LookupModel(t.qualified); found && model != nil && model.Metadata != nil {
			// Copies, not the catalog's pointers: extension code must not be
			// able to mutate shared pricing or capabilities (or race catalog
			// updates).
			if pricing := model.Metadata.Pricing.AtTime(now); pricing != nil {
				candidate.InputPerMtok = copyPrice(pricing.InputPerMtok)
				candidate.OutputPerMtok = copyPrice(pricing.OutputPerMtok)
			}
			candidate.Capabilities = copyCapabilities(model.Metadata.Capabilities)
		}
		req.Candidates[i] = candidate
	}

	qualified, answered := selector.Select(req)
	if !answered {
		return resolvedTarget{}, false
	}
	return poolTarget(pool, qualified)
}

// copyPrice clones an optional per-Mtok price.
func copyPrice(price *float64) *float64 {
	if price == nil {
		return nil
	}
	v := *price
	return &v
}

// copyCapabilities clones the catalog's capability flags for a candidate.
// Nil input yields a nil map, which selectors must read as "no capability
// information".
func copyCapabilities(caps map[string]bool) map[string]bool {
	if len(caps) == 0 {
		return nil
	}
	out := make(map[string]bool, len(caps))
	maps.Copy(out, caps)
	return out
}
