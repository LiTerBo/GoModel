package admin

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/capability"
	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/modeldata/modeltest"
)

// ModelTestAdmin is the surface the admin API needs from the model-test
// feature: run probes, and persist operator-confirmed capabilities.
type ModelTestAdmin interface {
	Probe(provider, model string, probes []modeltest.Probe) []modeltest.Result
	MergeModelCapabilities(providerName, modelID string, caps map[string]bool, source string) bool
}

// ModelCapabilityMerger is the confirmation half (satisfied by *ModelRegistry).
type ModelCapabilityMerger interface {
	MergeModelCapabilities(providerName, modelID string, caps map[string]bool, source string) bool
}

// ProbeRunner is the probe half (satisfied by a *modeltest.Tester wrapper).
type ProbeRunner interface {
	Probe(provider, model string, probes []modeltest.Probe) []modeltest.Result
}

// ModelTestService bundles the probe runner and the registry-backed
// confirmation channel into the ModelTestAdmin the handlers consume.
type ModelTestService struct {
	Prober   ProbeRunner
	Registry ModelCapabilityMerger
}

func (m *ModelTestService) Probe(provider, model string, probes []modeltest.Probe) []modeltest.Result {
	if m == nil || m.Prober == nil {
		return nil
	}
	return m.Prober.Probe(provider, model, probes)
}

func (m *ModelTestService) MergeModelCapabilities(providerName, modelID string, caps map[string]bool, source string) bool {
	if m == nil || m.Registry == nil {
		return false
	}
	return m.Registry.MergeModelCapabilities(providerName, modelID, caps, source)
}

// modelTestStore keeps the most recent probe results per selector so
// GET /admin/models/test-results can replay the last run without re-probing.
type modelTestStore struct {
	mu      sync.Mutex
	latest  map[string][]modeltest.Result // "provider/model" → results of the last run
	ordered []string                      // insertion order for stable listing
}

func newModelTestStore() *modelTestStore {
	return &modelTestStore{latest: make(map[string][]modeltest.Result)}
}

func selectorOf(provider, model string) string { return provider + "/" + model }

func (s *modelTestStore) record(provider, model string, results []modeltest.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := selectorOf(provider, model)
	if _, seen := s.latest[key]; !seen {
		s.ordered = append(s.ordered, key)
	}
	s.latest[key] = results
}

func (s *modelTestStore) snapshot() map[string][]modeltest.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]modeltest.Result, len(s.latest))
	for k, v := range s.latest {
		out[k] = v
	}
	return out
}

func (s *modelTestStore) order() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.ordered...)
}

// validProbes whitelists the probe names accepted over the admin API.
var validProbes = map[string]modeltest.Probe{
	string(modeltest.ProbeChat):            modeltest.ProbeChat,
	string(modeltest.ProbeEmbeddings):      modeltest.ProbeEmbeddings,
	string(modeltest.ProbeFunctionCalling): modeltest.ProbeFunctionCalling,
}

type runModelTestRequest struct {
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Probes   []string `json:"probes,omitempty"` // default: chat + function_calling
}

type confirmCapabilityRequest struct {
	Provider string          `json:"provider"`
	Model    string          `json:"model"`
	Caps     map[string]bool `json:"capabilities"`
}

// RunModelTest handles POST /admin/models/test: runs offline probes against
// one provider/model pair and records the results.
//
// @Summary      Run offline model capability probes
// @Description  Executes bounded, minimal-payload probes (chat answer, embeddings vector, tool_calls emission) against one model and stores the verdicts. Probes never mutate model metadata by themselves.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  runModelTestRequest  true  "Target and probes"
// @Success      200  {array}   modeltest.Result
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Failure      503  {object}  core.GatewayError
// @Router       /admin/models/test [post]
func (h *Handler) RunModelTest(c *echo.Context) error {
	if h.modelTest == nil {
		return handleError(c, featureUnavailableError("model test feature is unavailable"))
	}
	var req runModelTestRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	req.Provider = strings.TrimSpace(req.Provider)
	req.Model = strings.TrimSpace(req.Model)
	if req.Provider == "" || req.Model == "" {
		return handleError(c, core.NewInvalidRequestError("provider and model are required", nil))
	}
	probes, err := resolveProbes(req.Probes)
	if err != nil {
		return handleError(c, core.NewInvalidRequestError(err.Error(), nil))
	}

	results := h.modelTest.Probe(req.Provider, req.Model, probes)
	h.modelTestResults.record(req.Provider, req.Model, results)
	if results == nil {
		results = []modeltest.Result{}
	}
	return c.JSON(http.StatusOK, results)
}

// ListModelTestResults handles GET /admin/models/test-results.
//
// @Summary      List recent model probe results
// @Description  Returns the stored verdicts of the most recent probe run per model.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string][]modeltest.Result
// @Failure      401  {object}  core.GatewayError
// @Failure      503  {object}  core.GatewayError
// @Router       /admin/models/test-results [get]
func (h *Handler) ListModelTestResults(c *echo.Context) error {
	if h.modelTestResults == nil {
		return handleError(c, featureUnavailableError("model test feature is unavailable"))
	}
	ordered := h.modelTestResults.order()
	latest := h.modelTestResults.snapshot()
	results := make(map[string][]modeltest.Result, len(ordered))
	for _, key := range ordered {
		results[key] = latest[key]
	}
	return c.JSON(http.StatusOK, results)
}

// confirmCapabilities is the shared confirmation path for the probe and
// observation endpoints: it persists the verdict durably first (so a restart
// never loses an acknowledged confirmation), then merges into the in-memory
// registry. Without a wired confirmer the historical registry-only path
// applies. source must be one of the core.CapSrc* confirmation sources.
func (h *Handler) confirmCapabilities(c *echo.Context, provider, model string, caps map[string]bool, source string) error {
	if h.capabilities != nil {
		if err := h.capabilities.Confirm(c.Request().Context(), provider, model, caps, source); err != nil {
			if errors.Is(err, capability.ErrUnknownModel) {
				return core.NewModelNotFoundError(selectorOf(provider, model))
			}
			return err
		}
	}
	if !h.modelTest.MergeModelCapabilities(provider, model, caps, source) {
		return core.NewModelNotFoundError(selectorOf(provider, model))
	}
	return nil
}

// ConfirmModelCapabilities handles PUT /admin/models/capabilities: an
// operator's explicit confirmation of probe/observation verdicts. Only this
// endpoint (and its observation counterpart) can change model capabilities;
// probes alone never do.
//
// @Summary      Confirm model capabilities
// @Description  Persists operator-confirmed capabilities for one model with the confirmation source. True confirms support; false records an explicitly confirmed unsupported (probes/observations must never produce it silently).
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  confirmCapabilityRequest  true  "Target and confirmed capabilities"
// @Success      200  {object}  map[string]bool
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Failure      404  {object}  core.GatewayError
// @Failure      503  {object}  core.GatewayError
// @Router       /admin/models/capabilities [put]
func (h *Handler) ConfirmModelCapabilities(c *echo.Context) error {
	return h.confirmCapabilitiesRequest(c, core.CapSrcTest)
}

// ConfirmObservedCapabilities handles PUT /admin/models/observed-capabilities:
// operator confirmation of a passive-observation suggestion (W2b). Same
// persistence channel as probe confirmations, but stamped with the
// "observed" source so provenance stays distinguishable.
//
// @Summary      Confirm observed capabilities
// @Description  Persists operator-confirmed capabilities mined from audit-log observations (≥3 distinct sessions), with the observed source.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  confirmCapabilityRequest  true  "Target and confirmed capabilities"
// @Success      200  {object}  map[string]bool
// @Failure      400  {object}  core.GatewayError
// @Failure      401  {object}  core.GatewayError
// @Failure      404  {object}  core.GatewayError
// @Failure      503  {object}  core.GatewayError
// @Router       /admin/models/observed-capabilities [put]
func (h *Handler) ConfirmObservedCapabilities(c *echo.Context) error {
	return h.confirmCapabilitiesRequest(c, core.CapSrcObserved)
}

// confirmCapabilitiesRequest validates one operator confirmation payload and
// persists it under the given provenance source: a probe confirmation and an
// observed-capability confirmation differ only in that source.
func (h *Handler) confirmCapabilitiesRequest(c *echo.Context, source string) error {
	if h.modelTest == nil {
		return handleError(c, featureUnavailableError("model test feature is unavailable"))
	}
	var req confirmCapabilityRequest
	if err := c.Bind(&req); err != nil {
		return handleError(c, core.NewInvalidRequestError("invalid request body: "+err.Error(), err))
	}
	req.Provider = strings.TrimSpace(req.Provider)
	req.Model = strings.TrimSpace(req.Model)
	if req.Provider == "" || req.Model == "" || len(req.Caps) == 0 {
		return handleError(c, core.NewInvalidRequestError("provider, model and capabilities are required", nil))
	}
	for key := range req.Caps {
		if strings.TrimSpace(key) == "" {
			return handleError(c, core.NewInvalidRequestError("capability keys must be non-empty", nil))
		}
	}
	if err := h.confirmCapabilities(c, req.Provider, req.Model, req.Caps, source); err != nil {
		return handleError(c, err)
	}
	return c.JSON(http.StatusOK, req.Caps)
}

func resolveProbes(names []string) ([]modeltest.Probe, error) {
	if len(names) == 0 {
		return []modeltest.Probe{modeltest.ProbeChat, modeltest.ProbeFunctionCalling}, nil
	}
	probes := make([]modeltest.Probe, 0, len(names))
	for _, name := range names {
		probe, ok := validProbes[strings.TrimSpace(name)]
		if !ok {
			return nil, errInvalidProbe(name)
		}
		probes = append(probes, probe)
	}
	return probes, nil
}
