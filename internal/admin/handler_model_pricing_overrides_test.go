package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/modelselectors"
	"github.com/enterpilot/gomodel/internal/pricingoverrides"
)

type modelPricingOverrideTestStore struct {
	items map[string]pricingoverrides.Override
}

func newModelPricingOverrideTestStore(items ...pricingoverrides.Override) *modelPricingOverrideTestStore {
	store := &modelPricingOverrideTestStore{items: make(map[string]pricingoverrides.Override, len(items))}
	for _, item := range items {
		store.items[item.Selector] = item
	}
	return store
}

func (s *modelPricingOverrideTestStore) List(_ context.Context) ([]pricingoverrides.Override, error) {
	result := make([]pricingoverrides.Override, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	return result, nil
}

func (s *modelPricingOverrideTestStore) Upsert(_ context.Context, override pricingoverrides.Override) error {
	s.items[override.Selector] = override
	return nil
}

func (s *modelPricingOverrideTestStore) Delete(_ context.Context, selector string) error {
	if _, ok := s.items[selector]; !ok {
		return pricingoverrides.ErrNotFound
	}
	delete(s.items, selector)
	return nil
}

func (s *modelPricingOverrideTestStore) Close() error { return nil }

type modelPricingOverrideTestCatalog struct {
	providerNames []string
}

func (c modelPricingOverrideTestCatalog) ProviderNames() []string {
	return append([]string(nil), c.providerNames...)
}

func newModelPricingOverrideService(t *testing.T, store pricingoverrides.Store, providerNames ...string) *pricingoverrides.Service {
	t.Helper()
	if len(providerNames) == 0 {
		providerNames = []string{"openai"}
	}
	service, err := pricingoverrides.NewService(store, modelPricingOverrideTestCatalog{providerNames: providerNames}, nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	return service
}

func TestModelPricingOverrideLifecycle(t *testing.T) {
	tests := []struct {
		name         string
		providers    []string
		selector     string
		price        float64
		wantProvider string
		wantModel    string
		wantScope    modelselectors.ScopeKind
	}{
		{
			name:         "simple provider model selector",
			providers:    []string{"openai"},
			selector:     "openai/gpt-4o",
			price:        1.25,
			wantProvider: "openai",
			wantModel:    "gpt-4o",
			wantScope:    modelselectors.ScopeProviderModel,
		},
		{
			name:         "provider model selector with slash-shaped model id",
			providers:    []string{"openrouter"},
			selector:     "openrouter/meta-llama/llama-3.1-8b-instruct",
			price:        0.18,
			wantProvider: "openrouter",
			wantModel:    "meta-llama/llama-3.1-8b-instruct",
			wantScope:    modelselectors.ScopeProviderModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newModelPricingOverrideService(t, newModelPricingOverrideTestStore(), tt.providers...)
			h := NewHandler(nil, nil, WithPricingOverrides(service))
			e := echo.New()
			h.RegisterRoutes(e.Group("/admin"))

			bodyJSON := `{"selector":"` + tt.selector + `","pricing":{"input_per_mtok":` + strconv.FormatFloat(tt.price, 'f', -1, 64) + `}}`
			putReq := httptest.NewRequest(http.MethodPut, "/admin/model-pricing-overrides", bytes.NewBufferString(bodyJSON))
			putReq.Header.Set("Content-Type", "application/json")
			putRec := httptest.NewRecorder()
			e.ServeHTTP(putRec, putReq)
			if putRec.Code != http.StatusOK {
				t.Fatalf("put status = %d, want 200 body=%s", putRec.Code, putRec.Body.String())
			}

			var body pricingoverrides.View
			if err := json.Unmarshal(putRec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode upsert response: %v", err)
			}
			assertPricingOverrideView(t, body, tt.selector, tt.wantProvider, tt.wantModel, tt.wantScope, tt.price)

			listReq := httptest.NewRequest(http.MethodGet, "/admin/model-pricing-overrides", nil)
			listRec := httptest.NewRecorder()
			e.ServeHTTP(listRec, listReq)
			if listRec.Code != http.StatusOK {
				t.Fatalf("list status = %d, want 200", listRec.Code)
			}
			var listBody []pricingoverrides.View
			if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
				t.Fatalf("decode list response: %v", err)
			}
			if len(listBody) != 1 {
				t.Fatalf("list length = %d, want 1: %+v", len(listBody), listBody)
			}
			assertPricingOverrideView(t, listBody[0], tt.selector, tt.wantProvider, tt.wantModel, tt.wantScope, tt.price)

			deleteReq := httptest.NewRequest(http.MethodDelete, "/admin/model-pricing-overrides", bytes.NewBufferString(`{"selector":"`+tt.selector+`"}`))
			deleteReq.Header.Set("Content-Type", "application/json")
			deleteRec := httptest.NewRecorder()
			e.ServeHTTP(deleteRec, deleteReq)
			if deleteRec.Code != http.StatusNoContent {
				t.Fatalf("delete status = %d, want 204", deleteRec.Code)
			}
		})
	}
}

func assertPricingOverrideView(t *testing.T, view pricingoverrides.View, selector, provider, model string, scope modelselectors.ScopeKind, price float64) {
	t.Helper()
	if view.Selector != selector || view.ProviderName != provider || view.Model != model {
		t.Fatalf("selector parts = (%q, %q, %q), want (%q, %q, %q)", view.Selector, view.ProviderName, view.Model, selector, provider, model)
	}
	if view.ScopeKind != scope {
		t.Fatalf("ScopeKind = %q, want %q", view.ScopeKind, scope)
	}
	if view.Pricing.InputPerMtok == nil || *view.Pricing.InputPerMtok != price {
		t.Fatalf("InputPerMtok = %#v, want %v", view.Pricing.InputPerMtok, price)
	}
}

func TestUpsertModelPricingOverrideReturnsBadRequestForValidationErrors(t *testing.T) {
	service := newModelPricingOverrideService(t, newModelPricingOverrideTestStore())
	h := NewHandler(nil, nil, WithPricingOverrides(service))
	e := echo.New()

	req := httptest.NewRequest(http.MethodPut, "/admin/model-pricing-overrides", bytes.NewBufferString(`{"selector":"openai/gpt-4o","pricing":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.UpsertModelPricingOverride(c); err != nil {
		t.Fatalf("UpsertModelPricingOverride() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUpsertModelPricingOverrideWithTimeWindows(t *testing.T) {
	service := newModelPricingOverrideService(t, newModelPricingOverrideTestStore())
	h := NewHandler(nil, nil, WithPricingOverrides(service))
	e := echo.New()
	h.RegisterRoutes(e.Group("/admin"))

	payload := `{
		"selector":"deepseek/deepseek-v4-flash",
		"pricing":{
			"input_per_mtok":0.44,
			"output_per_mtok":1.32,
			"time_windows":[
				{
					"label":"off_peak",
					"utc_ranges":[
						{"days":["mon","tue","wed","thu","fri"],"start":"00:00","end":"01:00"},
						{"days":["sat","sun"],"start":"00:00","end":"24:00"}
					],
					"pricing":{
						"input_per_mtok":0.22,
						"output_per_mtok":0.66
					}
				}
			]
		}
	}`
	putReq := httptest.NewRequest(http.MethodPut, "/admin/model-pricing-overrides", bytes.NewBufferString(payload))
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	e.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200 body=%s", putRec.Code, putRec.Body.String())
	}

	var putBody pricingoverrides.View
	if err := json.Unmarshal(putRec.Body.Bytes(), &putBody); err != nil {
		t.Fatalf("decode upsert response: %v", err)
	}
	windows := putBody.Pricing.TimeWindows
	if len(windows) != 1 {
		t.Fatalf("put TimeWindows = %d, want 1", len(windows))
	}
	w := windows[0]
	if w.Label != "off_peak" || len(w.UTCRanges) != 2 {
		t.Fatalf("put window = %+v", w)
	}
	if w.UTCRanges[0].Start != "00:00" || w.UTCRanges[1].End != "24:00" {
		t.Fatalf("put ranges = %+v", w.UTCRanges)
	}
	if w.UTCRanges[0].Days[0] != "mon" {
		t.Fatalf("put days = %v", w.UTCRanges[0].Days)
	}
	if w.Pricing.InputPerMtok == nil || *w.Pricing.InputPerMtok != 0.22 ||
		w.Pricing.OutputPerMtok == nil || *w.Pricing.OutputPerMtok != 0.66 {
		t.Fatalf("put window rates = %+v", w.Pricing)
	}

	// List endpoint round-trips windows too.
	listReq := httptest.NewRequest(http.MethodGet, "/admin/model-pricing-overrides", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listRec.Code)
	}
	var listBody []pricingoverrides.View
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listBody) != 1 {
		t.Fatalf("list length = %d, want 1", len(listBody))
	}
	lw := listBody[0].Pricing.TimeWindows
	if len(lw) != 1 || lw[0].Label != "off_peak" || len(lw[0].UTCRanges) != 2 {
		t.Fatalf("list TimeWindows = %+v", lw)
	}
}

func TestUpsertModelPricingOverrideRejectsInvalidTimeWindows(t *testing.T) {
	service := newModelPricingOverrideService(t, newModelPricingOverrideTestStore())
	e := echo.New()
	h := NewHandler(nil, nil, WithPricingOverrides(service))
	h.RegisterRoutes(e.Group("/admin"))

	for name, bad := range map[string]string{
		"missing label": `{
			"selector":"deepseek/deepseek-v4-flash",
			"pricing":{"time_windows":[{"utc_ranges":[{"start":"00:00","end":"24:00"}],"pricing":{"input_per_mtok":0.1}}]}
		}`,
		"bad clock": `{
			"selector":"deepseek/deepseek-v4-flash",
			"pricing":{"time_windows":[{"label":"off_peak","utc_ranges":[{"start":"25:00","end":"24:00"}],"pricing":{"input_per_mtok":0.1}}]}
		}`,
		"foreign weekday": `{
			"selector":"deepseek/deepseek-v4-flash",
			"pricing":{"time_windows":[{"label":"off_peak","utc_ranges":[{"days":["tues"],"start":"00:00","end":"24:00"}],"pricing":{"input_per_mtok":0.1}}]}
		}`,
		"empty rates": `{
			"selector":"deepseek/deepseek-v4-flash",
			"pricing":{"time_windows":[{"label":"off_peak","utc_ranges":[{"start":"00:00","end":"24:00"}],"pricing":{}}]}
		}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/admin/model-pricing-overrides", bytes.NewBufferString(bad))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
