package admin

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// T13: a virtual model authorized by name is an authorization unit, so its
// target list may change under callers that never touched their own policy.
// This read-only endpoint answers "who does a change reach", for the alias
// editor's pre-save preview and for closing time on an alias. It only reads
// the in-memory key and user snapshots and reuses the request-time scope rule,
// so a preview cannot disagree with who can actually address the alias.

// authorizedByGrant is one policy holder the change reaches.
type authorizedByGrant struct {
	Kind      string       `json:"kind"` // credential | user_path
	ID        string       `json:"id,omitempty"`
	Label     string       `json:"label"`
	Path      string       `json:"path,omitempty"`
	Change    ImpactChange `json:"change"`
	MatchedBy string       `json:"matched_by,omitempty"`
	Matched   []string     `json:"matched,omitempty"`
}

// authorizedBySummary counts holders per verdict so the UI can lead with a
// one-line sentence.
type authorizedBySummary struct {
	Follow       int `json:"follow"`
	Potential    int `json:"potential"`
	Unrestricted int `json:"unrestricted"`
}

// authorizedByResponse is the impact report for one virtual model, optionally
// evaluated against a proposed target list.
type authorizedByResponse struct {
	Source     string              `json:"source"`
	OldTargets []string            `json:"old_targets"`
	NewTargets []string            `json:"new_targets"`
	Grants     []authorizedByGrant `json:"grants"`
	Summary    authorizedBySummary `json:"summary"`
}

// AuthorizedByVirtualModel reports the credentials and user paths a virtual
// model reaches. With new_targets it reports the effect of a proposed edit
// instead of the stored definition.
//
// @Summary      Report who a virtual model reaches, optionally against a proposed target list
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        source      query     string  true   "Virtual model source (alias name)"
// @Param        new_targets query     string  false  "Comma-separated proposed targets (provider/model); defaults to the stored ones"
// @Success      200         {object}  authorizedByResponse
// @Failure      400         {object}  core.GatewayError
// @Failure      401         {object}  core.GatewayError
// @Failure      404         {object}  core.GatewayError
// @Failure      503         {object}  core.GatewayError
// @Router       /admin/virtual-models/authorized-by [get]
func (h *Handler) AuthorizedByVirtualModel(c *echo.Context) error {
	if h.virtualModels == nil {
		return handleError(c, featureUnavailableError("virtual models feature is unavailable"))
	}

	source := strings.TrimSpace(c.QueryParam("source"))
	if source == "" {
		return handleError(c, core.NewInvalidRequestError("source is required", nil))
	}
	vm, ok := h.virtualModels.Get(source)
	if !ok {
		return handleError(c, core.NewNotFoundError("virtual model not found: "+source))
	}

	current := targetSelectors(vm.Targets)
	proposed := current
	if raw := strings.TrimSpace(c.QueryParam("new_targets")); raw != "" {
		selectors, err := parseTargetSelectorList(raw)
		if err != nil {
			return handleError(c, core.NewInvalidRequestError("invalid new_targets: "+err.Error(), err))
		}
		proposed = selectors
	}

	return c.JSON(http.StatusOK, h.authorizedByResponse(vm, current, proposed))
}

// collectAuthorizedBy lists the credentials and user paths the virtual model
// reaches, ordered by how much attention each verdict deserves: holders that
// silently follow the name first, then the ones a change may add or remove.
func (h *Handler) collectAuthorizedBy(vm *virtualmodels.VirtualModel, current, proposed []core.ModelSelector) []authorizedByGrant {
	grants := make([]authorizedByGrant, 0, 4)
	if h.authKeys != nil {
		// Every key is listed, including deactivated ones: the operator wants to
		// know which policies are configured against the name, and a reactivated
		// key starts following immediately.
		for _, view := range h.authKeys.ListViews() {
			if !virtualmodels.UserPathAllowed(view.UserPath, vm.UserPaths) {
				continue
			}
			verdict := classifyGrantImpact(vm.Source, view.AllowedModels, current, proposed)
			if verdict.Change == ImpactNone {
				continue
			}
			grants = append(grants, authorizedByGrant{
				Kind:      "credential",
				ID:        view.ID,
				Label:     view.Name,
				Path:      view.UserPath,
				Change:    verdict.Change,
				MatchedBy: verdict.MatchedBy,
				Matched:   verdict.Matched,
			})
		}
	}
	if h.users != nil {
		// User rows carry per-path policies; an empty entry list means the node
		// imposes no restriction of its own, so a restricted ancestor is the row
		// that shows up as the affected holder.
		for _, user := range h.users.List() {
			if !virtualmodels.UserPathAllowed(user.UserPath, vm.UserPaths) {
				continue
			}
			verdict := classifyGrantImpact(vm.Source, user.AllowedModels, current, proposed)
			if verdict.Change == ImpactNone {
				continue
			}
			grants = append(grants, authorizedByGrant{
				Kind:      "user_path",
				Label:     user.UserPath,
				Path:      user.UserPath,
				Change:    verdict.Change,
				MatchedBy: verdict.MatchedBy,
				Matched:   verdict.Matched,
			})
		}
	}

	sort.SliceStable(grants, func(i, j int) bool {
		if grants[i].Change != grants[j].Change {
			return impactRank(grants[i].Change) < impactRank(grants[j].Change)
		}
		if grants[i].Kind != grants[j].Kind {
			return grants[i].Kind < grants[j].Kind
		}
		return grants[i].Label < grants[j].Label
	})

	return grants
}

// virtualModelUsage returns the policy holders a delete would take the virtual
// model away from.
func (h *Handler) virtualModelUsage(vm *virtualmodels.VirtualModel) []authorizedByGrant {
	targets := targetSelectors(vm.Targets)
	return h.collectAuthorizedBy(vm, targets, targets)
}

func countGrantKind(grants []authorizedByGrant, kind string) int {
	count := 0
	for _, grant := range grants {
		if grant.Kind == kind {
			count++
		}
	}
	return count
}

func (h *Handler) authorizedByResponse(vm *virtualmodels.VirtualModel, current, proposed []core.ModelSelector) authorizedByResponse {
	grants := h.collectAuthorizedBy(vm, current, proposed)

	summary := authorizedBySummary{}
	for _, grant := range grants {
		switch grant.Change {
		case ImpactFollow:
			summary.Follow++
		case ImpactPotential:
			summary.Potential++
		case ImpactUnrestricted:
			summary.Unrestricted++
		}
	}

	return authorizedByResponse{
		Source:     vm.Source,
		OldTargets: selectorStrings(current),
		NewTargets: selectorStrings(proposed),
		Grants:     grants,
		Summary:    summary,
	}
}

// impactRank orders verdicts by how much they deserve attention: holders that
// silently follow first, then the ones the change may add or remove.
func impactRank(change ImpactChange) int {
	switch change {
	case ImpactFollow:
		return 0
	case ImpactPotential:
		return 1
	default:
		return 2
	}
}

func targetSelectors(targets []virtualmodels.Target) []core.ModelSelector {
	selectors := make([]core.ModelSelector, 0, len(targets))
	for _, target := range targets {
		selector, err := core.ParseModelSelector(target.Model, target.Provider)
		if err != nil {
			continue
		}
		selectors = append(selectors, selector)
	}
	return selectors
}

func parseTargetSelectorList(raw string) ([]core.ModelSelector, error) {
	parts := strings.Split(raw, ",")
	selectors := make([]core.ModelSelector, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("empty target selector")
		}
		selector, err := core.ParseModelSelector(part, "")
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, selector)
	}
	return selectors, nil
}

func selectorStrings(selectors []core.ModelSelector) []string {
	out := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		out = append(out, selector.QualifiedModel())
	}
	return out
}
