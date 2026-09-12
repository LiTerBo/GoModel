package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/auditlog"
	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/plugins"
	"github.com/enterpilot/gomodel/internal/virtualmodels"
	"github.com/enterpilot/gomodel/pluginapi"
)

// upsertVirtualModelRequest is the unified admin upsert contract. Presence of
// target_model or targets makes the row a redirect; absence makes it an access
// policy. A single target_model is a plain alias; multiple targets are load
// balanced across by strategy ("round_robin", "cost", "failover", "adaptive",
// or "plugin" with strategy_plugin naming the routing-strategy plugin).
type upsertVirtualModelRequest struct {
	Source      string                      `json:"source"`
	OldSource   string                      `json:"old_source,omitempty"`
	TargetModel string                      `json:"target_model,omitempty"`
	Targets     []virtualModelTargetRequest `json:"targets,omitempty"`
	Strategy    string                      `json:"strategy,omitempty"`
	// StrategyPlugin names the routing-strategy plugin for strategy "plugin";
	// StrategyConfig holds that plugin's route-scoped settings, validated
	// against its schema (see GET /admin/plugins, route_fields).
	StrategyPlugin string         `json:"strategy_plugin,omitempty"`
	StrategyConfig map[string]any `json:"strategy_config,omitempty"`
	// SessionAffinity keeps a detected session on the target that served it
	// before. Omitted means enabled; false restores stateless balancing.
	SessionAffinity *bool `json:"session_affinity,omitempty"`
	// Failover retries a failed request on the remaining targets. Omitted
	// means enabled; false serves the chosen target only.
	Failover    *bool    `json:"failover,omitempty"`
	UserPaths   []string `json:"user_paths,omitempty"`
	Description string   `json:"description,omitempty"`
	// Slowdown is an extra-time factor from 0.1 to 10; zero disables it.
	Slowdown *float64 `json:"slowdown,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty"`
	// Locked freezes the pointing; omitted keeps the stored state. Unlock is the
	// explicit gesture that lets a locked row change its targets or strategy in
	// the same request (sending locked:false does the same and turns the lock
	// off).
	Locked *bool `json:"locked,omitempty"`
	Unlock bool  `json:"unlock,omitempty"`
}

// virtualModelTargetRequest is one load-balancing destination. Model may be a
// bare id (with provider set) or a "provider/model" selector. Weight biases the
// round_robin strategy and defaults to 1.
type virtualModelTargetRequest struct {
	Provider string  `json:"provider,omitempty"`
	Model    string  `json:"model"`
	Weight   float64 `json:"weight,omitempty"`
}

type deleteVirtualModelRequest struct {
	Source string `json:"source"`
	// Force overrides the reachability guard: deleting a virtual model that
	// credentials or user paths still reach takes it away from them.
	Force bool `json:"force,omitempty"`
}

// ListVirtualModels handles GET /admin/virtual-models.
//
// @Summary      List virtual models (redirects and access policies)
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   virtualmodels.View
// @Failure      401  {object}  core.GatewayError
// @Failure      503  {object}  core.GatewayError
// @Router       /admin/virtual-models [get]
func (h *Handler) ListVirtualModels(c *echo.Context) error {
	if h.virtualModels == nil {
		return handleError(c, featureUnavailableError("virtual models feature is unavailable"))
	}
	views := h.virtualModels.ListViews()
	if views == nil {
		views = []virtualmodels.View{}
	}
	return c.JSON(http.StatusOK, views)
}

// UpsertVirtualModel handles PUT /admin/virtual-models. When old_source is set
// and differs from source, the row is renamed: stored under the new source and
// removed from the old one in a single validated operation.
//
// @Summary      Create, update, or rename one virtual model
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        virtual_model  body      upsertVirtualModelRequest  true  "Virtual model definition"
// @Success      200            {object}  virtualmodels.View
// @Success      204            "No-op access policy removed"
// @Failure      400            {object}  core.GatewayError
// @Failure      401            {object}  core.GatewayError
// @Failure      409            {object}  core.GatewayError  "Virtual model is locked: send an explicit unlock with the change"
// @Failure      502            {object}  core.GatewayError
// @Failure      503            {object}  core.GatewayError
// @Router       /admin/virtual-models [put]
func (h *Handler) UpsertVirtualModel(c *echo.Context) error {
	if h.virtualModels == nil {
		return handleError(c, featureUnavailableError("virtual models feature is unavailable"))
	}

	var req upsertVirtualModelRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		return handleError(c, core.NewInvalidRequestError("source is required", nil))
	}

	vm, err := h.buildVirtualModelUpsert(source, req)
	if err != nil {
		return handleError(c, err)
	}
	// The stored row answers both the lock guard and the audit event, so it is
	// read once; a rename guards the row it moves away from.
	guardSource := strings.TrimSpace(req.OldSource)
	if guardSource == "" {
		guardSource = source
	}
	stored, _ := h.virtualModels.Get(guardSource)
	if reason := h.lockRejection(stored, vm, req.Unlock, req.Locked); reason != "" {
		return handleError(c, lockedVirtualModelError(reason))
	}
	oldSource := strings.TrimSpace(req.OldSource)
	renamed := oldSource != "" && oldSource != source
	if renamed {
		err = h.virtualModels.Rename(c.Request().Context(), oldSource, vm)
	} else {
		err = h.virtualModels.Upsert(c.Request().Context(), vm)
	}
	if err != nil {
		return handleError(c, virtualModelWriteError(err))
	}
	h.upsertVirtualModelChange(c, stored, vm, virtualModelChangeAction(stored, vm, renamed))

	if view, ok := h.findVirtualModelView(vm.Source); ok {
		return c.JSON(http.StatusOK, view)
	}
	return c.NoContent(http.StatusNoContent)
}

// DeleteVirtualModel handles DELETE /admin/virtual-models.
//
// @Summary      Delete one virtual model
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  deleteVirtualModelRequest  true  "Virtual model source to remove"
// @Success      204       "No Content"
// @Failure      400       {object}  core.GatewayError
// @Failure      401       {object}  core.GatewayError
// @Failure      404       {object}  core.GatewayError
// @Failure      409       {object}  core.GatewayError  "Credentials or user paths still reach the virtual model: send force to delete it"
// @Failure      502       {object}  core.GatewayError
// @Failure      503       {object}  core.GatewayError
// @Router       /admin/virtual-models [delete]
func (h *Handler) DeleteVirtualModel(c *echo.Context) error {
	if h.virtualModels == nil {
		return handleError(c, featureUnavailableError("virtual models feature is unavailable"))
	}

	var req deleteVirtualModelRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		return handleError(c, core.NewInvalidRequestError("source is required", nil))
	}

	stored, ok := h.virtualModels.Get(source)
	if !ok || stored == nil {
		return handleError(c, core.NewNotFoundError("virtual model not found: "+source))
	}
	// The reachability list answers both the guard and the audit event.
	used := h.virtualModelUsage(stored)
	credentials := countGrantKind(used, "credential")
	userPaths := countGrantKind(used, "user_path")
	if !req.Force && len(used) > 0 {
		return handleError(c, virtualModelInUseError(stored.Source, credentials, userPaths))
	}

	if err := h.virtualModels.Delete(c.Request().Context(), source); err != nil {
		if errors.Is(err, virtualmodels.ErrNotFound) {
			return handleError(c, core.NewNotFoundError("virtual model not found: "+source))
		}
		return handleError(c, virtualModelWriteError(err))
	}
	h.emitVirtualModelChange(c, auditlog.VirtualModelChange{
		Action:      auditlog.VirtualModelActionDelete,
		Source:      stored.Source,
		OldTargets:  targetStrings(*stored),
		Locked:      stored.Locked,
		Forced:      req.Force,
		Credentials: credentials,
		UserPaths:   userPaths,
	})
	return c.NoContent(http.StatusNoContent)
}

// buildVirtualModelUpsert maps the request into a VirtualModel. Presence of
// target_model makes a redirect; otherwise it is an access policy. Enabled
// defaults to true, preserving the existing value when omitted.
func (h *Handler) buildVirtualModelUpsert(source string, req upsertVirtualModelRequest) (virtualmodels.VirtualModel, error) {
	vm := virtualmodels.VirtualModel{
		Source:          source,
		Strategy:        strings.TrimSpace(req.Strategy),
		StrategyPlugin:  strings.TrimSpace(req.StrategyPlugin),
		StrategyConfig:  req.StrategyConfig,
		SessionAffinity: req.SessionAffinity,
		Failover:        req.Failover,
		UserPaths:       req.UserPaths,
		Description:     strings.TrimSpace(req.Description),
		Slowdown:        req.Slowdown,
		Enabled:         h.virtualModels.ResolveUpsertEnabled(source, req.OldSource, req.Enabled),
		Locked:          h.virtualModels.ResolveUpsertLocked(source, req.OldSource, req.Locked),
	}

	targets, err := buildVirtualModelTargets(req)
	if err != nil {
		return virtualmodels.VirtualModel{}, err
	}
	vm.Targets = targets
	if err := h.validateStrategyPlugin(vm); err != nil {
		return virtualmodels.VirtualModel{}, err
	}
	return vm, nil
}

// lockRejection reports why a locked virtual model rejects this write, or ""
// when the write is allowed. A locked row still takes edits that leave the
// pointing alone, but changing which models a caller can end up talking to
// needs an explicit unlock gesture in the same request: unlock:true keeps the
// lock on the new pointing, an explicit locked:false turns the lock off.
//
// The guard lives here rather than in the service because it is the operator
// guard on the admin API: config-declared rows are versioned by their config
// and never come through this path.
func (h *Handler) lockRejection(stored *virtualmodels.VirtualModel, next virtualmodels.VirtualModel, unlock bool, requestedLock *bool) string {
	if unlock || (requestedLock != nil && !*requestedLock) {
		return ""
	}
	if stored == nil || !stored.Locked {
		return ""
	}
	if !virtualmodels.ResolutionConfigChanged(*stored, next) {
		return ""
	}
	return fmt.Sprintf("virtual model %q is locked; send an explicit unlock to change its targets or strategy", stored.Source)
}

// validateStrategyPlugin rejects a plugin-strategy redirect whose plugin is
// not a loaded routing strategy, naming the loaded ones. The service then
// validates strategy_config against the plugin's route-scoped fields.
func (h *Handler) validateStrategyPlugin(vm virtualmodels.VirtualModel) error {
	if !vm.IsRedirect() || strings.ToLower(vm.Strategy) != virtualmodels.StrategyPlugin {
		return nil
	}
	if vm.StrategyPlugin == "" {
		return core.NewInvalidRequestError("strategy_plugin is required with strategy \"plugin\"", nil)
	}
	if h.pluginCatalog == nil {
		return nil
	}
	if entry, ok := h.pluginCatalog.Lookup(vm.StrategyPlugin); !ok || !entry.HasKind(pluginapi.KindRoute) {
		names := plugins.RoutePluginNames(h.pluginCatalog)
		known := "none"
		if len(names) > 0 {
			known = strings.Join(names, ", ")
		}
		return core.NewInvalidRequestError(fmt.Sprintf("unknown routing-strategy plugin %q (loaded: %s)", vm.StrategyPlugin, known), nil)
	}
	return nil
}

// buildVirtualModelTargets resolves the redirect targets from the request. The
// multi-target `targets` form takes precedence; a single `target_model` is the
// backward-compatible shorthand. An empty result makes the row an access policy.
//
// A target given as one name ("openai/gpt-4o", "team/cheap") is stored as
// written, like a config declaration: the name may belong to a virtual model,
// which makes the target a chain leg. Only an explicit `provider` pins the
// target to a concrete model.
func buildVirtualModelTargets(req upsertVirtualModelRequest) ([]virtualmodels.Target, error) {
	if len(req.Targets) > 0 {
		targets := make([]virtualmodels.Target, 0, len(req.Targets))
		for _, t := range req.Targets {
			model := strings.TrimSpace(t.Model)
			if model == "" {
				continue
			}
			target, err := virtualModelTarget(model, strings.TrimSpace(t.Provider))
			if err != nil {
				return nil, core.NewInvalidRequestError("invalid target model "+model+": "+err.Error(), err)
			}
			target.Weight = t.Weight
			targets = append(targets, target)
		}
		// A targets list with only blank entries is a malformed redirect, not an
		// access policy — fail loudly rather than silently demoting it.
		if len(targets) == 0 {
			return nil, core.NewInvalidRequestError("targets must contain at least one model", nil)
		}
		return targets, nil
	}

	if model := strings.TrimSpace(req.TargetModel); model != "" {
		target, err := virtualModelTarget(model, "")
		if err != nil {
			return nil, core.NewInvalidRequestError("invalid target_model: "+err.Error(), err)
		}
		return []virtualmodels.Target{target}, nil
	}
	return nil, nil
}

// virtualModelTarget validates one target declaration and keeps it in the
// shape it was given: a bare name stays whole, an explicit provider is split
// off the model.
func virtualModelTarget(model, provider string) (virtualmodels.Target, error) {
	selector, err := core.ParseModelSelector(model, provider)
	if err != nil {
		return virtualmodels.Target{}, err
	}
	if provider == "" {
		return virtualmodels.Target{Model: model}, nil
	}
	return virtualmodels.Target{Provider: selector.Provider, Model: selector.Model}, nil
}

// findVirtualModelView returns the admin view for a source after an upsert by
// matching it in the refreshed listing.
func (h *Handler) findVirtualModelView(source string) (virtualmodels.View, bool) {
	if stored, ok := h.virtualModels.Get(source); ok && stored != nil {
		source = stored.Source
	}
	for _, view := range h.virtualModels.ListViews() {
		if view.Source == source {
			return view, true
		}
	}
	return virtualmodels.View{}, false
}
