package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/anthropicapi"
	"github.com/enterpilot/gomodel/internal/core"
)

// visibleModels builds the model catalog this caller is allowed to see: the
// provider catalog (unless only aliases are exposed), filtered by the key's
// model policy, merged with the virtual models in scope for the request's user
// path. Both GET /v1/models and GET /v1/models/{model} answer from it, so a
// retrieve can never surface a model the list would have hidden.
func (h *Handler) visibleModels(c *echo.Context) (*core.ModelsResponse, error) {
	// Create context with request ID for provider
	requestID := c.Request().Header.Get(core.RequestIDHeader)
	ctx := core.WithRequestID(c.Request().Context(), requestID)

	resp, err := h.provider.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	if h.keepOnlyAliasesAtModelsEndpoint {
		object := "list"
		if resp != nil && resp.Object != "" {
			object = resp.Object
		}
		resp = &core.ModelsResponse{Object: object, Data: []core.Model{}}
	}
	if h.modelAuthorizer != nil && resp != nil {
		resp = &core.ModelsResponse{
			Object: resp.Object,
			Data:   h.modelAuthorizer.FilterPublicModels(c.Request().Context(), resp.Data),
		}
	}
	if h.exposedModelLister != nil {
		ctx := c.Request().Context()
		// The target-access filter is only available when an authorizer is set.
		var allow func(core.ModelSelector) bool
		var allowName func(string) bool
		if h.modelAuthorizer != nil {
			allow = func(selector core.ModelSelector) bool {
				return h.modelAuthorizer.AllowsModel(ctx, selector)
			}
			// An alias is addressed by name, so a credential whose allowlist
			// names it must see it even when its concrete targets are not
			// permitted: probe the same authorizer with a name-only selector.
			allowName = func(name string) bool {
				return name != "" && h.modelAuthorizer.AllowsModel(ctx, core.ModelSelector{Model: name})
			}
		}
		// User-path scoping of redirects is a property of the redirect itself, not
		// of the authorizer, so it must apply even when no authorizer is configured
		// (allow is nil there) — otherwise scoped redirect IDs leak to callers
		// outside their user_paths.
		if named, ok := h.exposedModelLister.(NamedUserPathExposedModelLister); ok && allowName != nil {
			resp = mergeExposedModelsResponse(resp, named.ExposedModelsForUserPathNamed(core.UserPathFromContext(ctx), allow, allowName))
		} else if scoped, ok := h.exposedModelLister.(UserPathExposedModelLister); ok {
			resp = mergeExposedModelsResponse(resp, scoped.ExposedModelsForUserPath(core.UserPathFromContext(ctx), allow))
		} else if filtered, ok := h.exposedModelLister.(FilteredExposedModelLister); ok && allow != nil {
			resp = mergeExposedModelsResponse(resp, filtered.ExposedModelsFiltered(allow))
		} else {
			exposed := h.exposedModelLister.ExposedModels()
			if allow != nil {
				kept := make([]core.Model, 0, len(exposed))
				for _, model := range exposed {
					selector, err := core.ParseModelSelector(model.ID, "")
					if err != nil || !allow(selector) {
						continue
					}
					kept = append(kept, model)
				}
				exposed = kept
			}
			resp = mergeExposedModelsResponse(resp, exposed)
		}
	}
	return resp, nil
}

// RetrieveModel handles GET /v1/models/{model}
//
// Model IDs carry the provider prefix and may contain several slashes
// (groq/openai/gpt-oss-20b, fireworks/accounts/fireworks/models/glm-5p3-flash),
// so the route is a wildcard and the ID is the whole remainder of the path,
// raw or percent-encoded.
//
// @Summary      Retrieve a model
// @Tags         models
// @Produce      json
// @Security     BearerAuth
// @Param        model  path      string  true  "Model ID, e.g. openai/gpt-4.1-mini"
// @Success      200    {object}  core.Model
// @Failure      401    {object}  core.OpenAIErrorEnvelope
// @Failure      404    {object}  core.OpenAIErrorEnvelope
// @Failure      502    {object}  core.OpenAIErrorEnvelope
// @Router       /v1/models/{model} [get]
func (h *Handler) RetrieveModel(c *echo.Context) error {
	modelID := retrieveModelID(c)
	if modelID == "" {
		// "/v1/models/" names no model; answer it as the list rather than as a
		// 404 for the empty model ID.
		return h.ListModels(c)
	}

	// Same user-path handling as the list endpoint, so both apply the same
	// visibility policy.
	if ok, err := applyUserPathHeaderToContext(c); !ok {
		return err
	}

	resp, err := h.visibleModels(c)
	if err != nil {
		return handleError(c, err)
	}

	var models []core.Model
	if resp != nil {
		models = resp.Data
	}
	for _, model := range models {
		if model.ID != modelID {
			continue
		}
		// The models routes are shared by both wire dialects; Anthropic SDK
		// clients are identified by the anthropic-version header they always
		// send (see ListModels).
		if c.Request().Header.Get("anthropic-version") != "" {
			return c.JSON(http.StatusOK, anthropicapi.FromModel(model))
		}
		if model.Object == "" {
			model.Object = "model"
		}
		return c.JSON(http.StatusOK, model)
	}

	notFound := core.NewModelNotFoundError(modelID)
	if c.Request().Header.Get("anthropic-version") != "" {
		status, body := anthropicapi.ErrorFromGateway(notFound)
		return c.JSON(status, body)
	}
	return handleError(c, notFound)
}

// retrieveModelID reads the model ID from the wildcard segment of
// /v1/models/*. Echo hands path params back percent-encoded, so an ID sent as
// openai%2Fgpt-4.1-mini resolves to the same model as openai/gpt-4.1-mini.
func retrieveModelID(c *echo.Context) string {
	raw := strings.TrimPrefix(c.Param("*"), "/")
	if raw == "" {
		return ""
	}
	if decoded, err := url.PathUnescape(raw); err == nil {
		return decoded
	}
	return raw
}
