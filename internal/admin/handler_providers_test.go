package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/providers"
)

// TestClassifyProviderStatus_HealthyForAllowlistInventory locks in the
// admin-endpoint behavior fixed alongside the registry change that makes
// allowlist mode set LastModelFetchSuccessAt. Before the fix, an allowlist
// provider serving real traffic appeared as status=degraded / label=Starting
// because the classifier treated LastModelFetchSuccessAt==nil as "still
// loading cached models". Now the classifier correctly reports healthy.
func TestClassifyProviderStatus_HealthyForAllowlistInventory(t *testing.T) {
	now := time.Now().UTC()
	cfg := providers.SanitizedProviderConfig{Name: "bedrock", Type: "bedrock"}
	runtime := providers.ProviderRuntimeSnapshot{
		Name:                    "bedrock",
		Type:                    "bedrock",
		Registered:              true,
		RegistryInitialized:     true,
		DiscoveredModelCount:    1,
		LastModelFetchAt:        &now,
		LastModelFetchSuccessAt: &now,
	}

	status, label, _, _ := classifyProviderStatus(cfg, runtime)
	if status != "healthy" {
		t.Fatalf("status = %q, want healthy", status)
	}
	if label != "Healthy" {
		t.Fatalf("label = %q, want Healthy", label)
	}
}

// A provider retired from load balancing by a failed availability probe has a
// clean model-fetch record but must not be reported healthy: the routing layer
// is actively skipping it and its models are hidden from the model list.
func TestClassifyProviderStatus_StaleInventoryIsUnhealthy(t *testing.T) {
	now := time.Now().UTC()
	cfg := providers.SanitizedProviderConfig{Name: "openai", Type: "openai"}
	runtime := providers.ProviderRuntimeSnapshot{
		Name:                    "openai",
		Type:                    "openai",
		Registered:              true,
		RegistryInitialized:     true,
		DiscoveredModelCount:    3,
		LastModelFetchAt:        &now,
		LastModelFetchSuccessAt: &now,
		LastAvailabilityCheckAt: &now,
		LastAvailabilityError:   "connection refused",
		InventoryStale:          true,
	}

	status, label, reason, lastError := classifyProviderStatus(cfg, runtime)
	if status != "unhealthy" {
		t.Fatalf("status = %q, want unhealthy", status)
	}
	if label != "Offline" {
		t.Fatalf("label = %q, want Offline", label)
	}
	if reason == "" {
		t.Fatal("reason empty, want stale-inventory explanation")
	}
	if lastError != "connection refused" {
		t.Fatalf("lastError = %q, want availability error surfaced", lastError)
	}
}

// --- Manual per-provider model refresh (POST /admin/providers/:name/models/refresh) ---

// refreshModelsContext builds the request context for the refresh route, with
// the path parameter echo's router would have filled in.
func refreshModelsContext(name string, withParam bool) (*echo.Context, *httptest.ResponseRecorder) {
	c, rec := newHandlerContextWithBody("/admin/providers/"+name+"/models/refresh", http.MethodPost, "")
	if withParam {
		c.SetPathValues(echo.PathValues{{Name: "name", Value: name}})
	}
	return c, rec
}

// initializedProviderRegistry registers one provider under its type name and
// initializes the registry against the mock's current listing.
func initializedProviderRegistry(t *testing.T, mock *handlerMockProvider) *providers.ModelRegistry {
	t.Helper()
	registry := providers.NewModelRegistry()
	registry.RegisterProviderWithType(mock, "openai")
	if err := registry.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	return registry
}

func modelsResponse(ids ...string) *core.ModelsResponse {
	resp := &core.ModelsResponse{Object: "list", Data: make([]core.Model, 0, len(ids))}
	for _, id := range ids {
		resp.Data = append(resp.Data, core.Model{ID: id, Object: "model", OwnedBy: "openai"})
	}
	return resp
}

func TestRefreshProviderModels_ReportsNewlyAdvertisedModels(t *testing.T) {
	mock := &handlerMockProvider{models: modelsResponse("model-a")}
	registry := initializedProviderRegistry(t, mock)

	// The upstream listing grows between the two fetches; picking that up
	// without waiting for the background interval is what the endpoint is for.
	mock.models = modelsResponse("model-a", "model-b")

	h := NewHandler(nil, registry)
	c, rec := refreshModelsContext("openai", true)

	if err := h.RefreshProviderModels(c); err != nil {
		t.Fatalf("RefreshProviderModels() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var got providerModelRefreshResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Provider != "openai" {
		t.Errorf("provider = %q, want openai", got.Provider)
	}
	if got.ModelCount != 2 {
		t.Errorf("model_count = %d, want 2", got.ModelCount)
	}
	if want := []string{"model-b"}; !slices.Equal(got.Added, want) {
		t.Errorf("added = %v, want %v", got.Added, want)
	}
	if len(got.Removed) != 0 {
		t.Errorf("removed = %v, want none", got.Removed)
	}
}

func TestRefreshProviderModels_ReportsWithdrawnModels(t *testing.T) {
	mock := &handlerMockProvider{models: modelsResponse("model-a", "model-b")}
	registry := initializedProviderRegistry(t, mock)

	mock.models = modelsResponse("model-a")

	h := NewHandler(nil, registry)
	c, rec := refreshModelsContext("openai", true)

	if err := h.RefreshProviderModels(c); err != nil {
		t.Fatalf("RefreshProviderModels() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var got providerModelRefreshResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ModelCount != 1 {
		t.Errorf("model_count = %d, want 1", got.ModelCount)
	}
	if len(got.Added) != 0 {
		t.Errorf("added = %v, want none", got.Added)
	}
	if want := []string{"model-b"}; !slices.Equal(got.Removed, want) {
		t.Errorf("removed = %v, want %v", got.Removed, want)
	}
}

func TestRefreshProviderModels_UnavailableWithoutRegistry(t *testing.T) {
	h := NewHandler(nil, nil)
	c, rec := refreshModelsContext("openai", true)

	if err := h.RefreshProviderModels(c); err != nil {
		t.Fatalf("RefreshProviderModels() error = %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshProviderModels_RequiresAProviderName(t *testing.T) {
	h := NewHandler(nil, providers.NewModelRegistry())
	c, rec := refreshModelsContext("", false)

	if err := h.RefreshProviderModels(c); err != nil {
		t.Fatalf("RefreshProviderModels() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshProviderModels_UnknownProviderIsRejected(t *testing.T) {
	registry := initializedProviderRegistry(t, &handlerMockProvider{models: modelsResponse("model-a")})

	h := NewHandler(nil, registry)
	c, rec := refreshModelsContext("missing", true)

	if err := h.RefreshProviderModels(c); err != nil {
		t.Fatalf("RefreshProviderModels() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshProviderModels_UpstreamFailureIsReportedAsSuch(t *testing.T) {
	mock := &handlerMockProvider{models: modelsResponse("model-a")}
	registry := initializedProviderRegistry(t, mock)
	mock.err = errors.New("upstream unavailable")

	h := NewHandler(nil, registry)
	c, rec := refreshModelsContext("openai", true)

	if err := h.RefreshProviderModels(c); err != nil {
		t.Fatalf("RefreshProviderModels() error = %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body = %s", rec.Code, rec.Body.String())
	}
}

// TestRefreshProviderModels_ThroughTheRouter exercises the refresh through the
// real echo router: the provider name is a path parameter, so a route pattern
// that never matched would fail before the handler ever saw a name.
func TestRefreshProviderModels_ThroughTheRouter(t *testing.T) {
	registry := providers.NewModelRegistry()
	mock := &handlerMockProvider{models: &core.ModelsResponse{
		Object: "list",
		Data:   []core.Model{{ID: "model-a", Object: "model"}},
	}}
	registry.RegisterProviderWithType(mock, "openai")
	if err := registry.Initialize(context.Background()); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	mock.models = &core.ModelsResponse{
		Object: "list",
		Data: []core.Model{
			{ID: "model-a", Object: "model"},
			{ID: "model-b", Object: "model"},
		},
	}

	h := NewHandler(nil, registry)
	e := echo.New()
	h.RegisterRoutes(e.Group("/admin"))

	req := httptest.NewRequest(http.MethodPost, "/admin/providers/openai/models/refresh", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got providerModelRefreshResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Provider != "openai" {
		t.Errorf("provider = %q, want openai (path parameter resolution)", got.Provider)
	}
	if !slices.Equal(got.Added, []string{"model-b"}) {
		t.Errorf("added = %v, want [model-b]", got.Added)
	}
}
