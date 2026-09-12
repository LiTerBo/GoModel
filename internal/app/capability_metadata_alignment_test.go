package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/enterpilot/gomodel/internal/cache/modelcache"
	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/providers"
)

func intPtr(v int) *int { return &v }

// reportedModels is what a local deployment says about itself: real limits,
// chat mode, and two capabilities it claims to support.
func reportedModels() *core.ModelsResponse {
	return &core.ModelsResponse{
		Object: "list",
		Data: []core.Model{{
			ID: "my-llm", Object: "model", OwnedBy: "local", Created: 1700000000,
			Metadata: &core.ModelMetadata{
				ContextWindow:   intPtr(32000),
				MaxOutputTokens: intPtr(4096),
				Modes:           []string{"chat"},
				Capabilities:    map[string]bool{"function_calling": true, "vision": true},
			},
		}},
	}
}

// TestProviderReportedMetadataAlignsWithConfirmationsAfterRestart is the F3
// alignment check between the upstream model cache (the provider's own report
// is persisted and restored as discovered metadata) and this fork's capability
// confirmation channel (operator verdicts persisted separately and replayed at
// startup). Neither side's unit tests can tell whether the two agree, because
// the agreement is a property of the pair:
//
//  1. what the provider reported survives a restart with the provider gone;
//  2. a confirmed verdict still wins over the restored provider report;
//  3. the cache holds the provider report only - a confirmed verdict must not
//     leak into it, or a stale verdict would outlive the capability store that
//     is its real home.
func TestProviderReportedMetadataAlignsWithConfirmationsAfterRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "models.json")
	dbPath := filepath.Join(dir, "capability.db")

	// Generation one: a live provider, the cache armed, and an operator
	// confirming that the provider's self-report is wrong about vision.
	firstRegistry := newTestRegistry(t, reportedModels(), nil)
	firstRegistry.SetCache(modelcache.NewLocalCache(cachePath))
	if err := firstRegistry.Initialize(ctx); err != nil {
		t.Fatalf("registry Initialize: %v", err)
	}
	first := openCapabilityGeneration(t, ctx, dbPath, firstRegistry)
	if err := first.bootstrap.app.capabilities.Service.Confirm(
		ctx, "local", "my-llm", map[string]bool{"vision": false}, core.CapSrcTest,
	); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	assertCapabilityState(t, firstRegistry, "vision", false, core.CapSrcTest)
	if err := firstRegistry.SaveToCache(ctx); err != nil {
		t.Fatalf("SaveToCache: %v", err)
	}

	cached := readCachedMetadata(t, cachePath, "local", "my-llm")
	if cached == nil || !cached.Capabilities["vision"] {
		t.Fatalf("cache metadata = %+v, want the provider report (vision true)", cached)
	}
	if got := cached.CapabilitySources["vision"]; got == core.CapSrcTest {
		t.Fatalf("confirmed verdict leaked into the model cache: %+v", cached.CapabilitySources)
	}

	// Generation two: a restart with the provider unreachable, so the
	// inventory can only come from the cache.
	secondRegistry := newTestRegistry(t, nil, errors.New("provider unreachable"))
	secondRegistry.SetCache(modelcache.NewLocalCache(cachePath))
	loaded, err := secondRegistry.LoadFromCache(ctx)
	if err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}
	if loaded == 0 {
		t.Fatal("cache restored no models")
	}
	second := openCapabilityGeneration(t, ctx, dbPath, secondRegistry)

	assertProviderReport(t, second.registry)
	assertCapabilityState(t, second.registry, "vision", false, core.CapSrcTest)
}

// TestDiscoveredProvenanceNeedsACatalog pins the other half of the discovery
// story: capabilities the provider reports are stamped core.CapSrcDiscovered by
// the catalog enrichment step, so the provenance appears only when a catalog is
// loaded. Without one the same capabilities publish with an empty
// CapabilitySources map - recorded as the open F3 gap.
func TestDiscoveredProvenanceNeedsACatalog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cache := modelcache.NewLocalCache(filepath.Join(t.TempDir(), "models.json"))
	if err := cache.Set(ctx, &modelcache.ModelCache{
		Providers: map[string]modelcache.CachedProvider{
			"local": {
				OwnedBy: "local",
				Models: []modelcache.CachedModel{{
					ID: "my-llm", Created: 1700000000, Metadata: reportedModels().Data[0].Metadata,
				}},
			},
		},
		ModelListData: []byte(`{
			"version": 1,
			"updated_at": "2026-04-11T00:00:00Z",
			"providers": {"local": {"display_name": "Local", "api_type": "openai", "supported_modes": ["chat"]}},
			"models": {"my-llm": {"display_name": "My LLM", "modes": ["chat"], "context_window": 8000}},
			"provider_models": {}
		}`),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	registry := newTestRegistry(t, nil, nil)
	registry.SetCache(cache)
	if _, err := registry.LoadFromCache(ctx); err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}

	meta := lookupMetadata(t, registry)
	for _, capability := range []string{"function_calling", "vision"} {
		if !meta.Capabilities[capability] {
			t.Errorf("%s lost across the cache round trip: %v", capability, meta.Capabilities)
		}
		if got := meta.CapabilitySources[capability]; got != core.CapSrcDiscovered {
			t.Errorf("%s source = %q, want %q", capability, got, core.CapSrcDiscovered)
		}
	}
	if meta.ContextWindow == nil || *meta.ContextWindow != 32000 {
		t.Errorf("context window = %v, want the provider's 32000 over the catalog's 8000", meta.ContextWindow)
	}
}

// assertProviderReport fails unless the values the provider reported are
// published after a cache-only start. Provenance is asserted separately: see
// TestDiscoveredProvenanceNeedsACatalog and the merge plan's F3 record.
func assertProviderReport(t *testing.T, registry *providers.ModelRegistry) {
	t.Helper()

	meta := lookupMetadata(t, registry)
	if meta.ContextWindow == nil || *meta.ContextWindow != 32000 {
		t.Errorf("context window = %v, want 32000 (provider report lost across restart)", meta.ContextWindow)
	}
	if meta.MaxOutputTokens == nil || *meta.MaxOutputTokens != 4096 {
		t.Errorf("max output tokens = %v, want 4096", meta.MaxOutputTokens)
	}
	if !slices.Contains(meta.Modes, "chat") {
		t.Errorf("modes = %v, want chat", meta.Modes)
	}
	if !meta.Capabilities["function_calling"] {
		t.Errorf("provider-reported function_calling lost: %v", meta.Capabilities)
	}
}

// assertCapabilityState fails unless the published capability holds the wanted
// value and carries the operator's provenance.
func assertCapabilityState(t *testing.T, registry *providers.ModelRegistry, capability string, want bool, wantSource string) {
	t.Helper()

	meta := lookupMetadata(t, registry)
	if got := meta.Capabilities[capability]; got != want {
		t.Fatalf("%s = %v, want %v (sources %v)", capability, got, want, meta.CapabilitySources)
	}
	if got := meta.CapabilitySources[capability]; got != wantSource {
		t.Fatalf("%s source = %q, want %q", capability, got, wantSource)
	}
}

func lookupMetadata(t *testing.T, registry *providers.ModelRegistry) *core.ModelMetadata {
	t.Helper()

	model, ok := registry.LookupModel("local/my-llm")
	if !ok {
		t.Fatal("local/my-llm missing from registry")
	}
	if model.Metadata == nil {
		t.Fatal("published model metadata is nil")
	}
	return model.Metadata
}

// readCachedMetadata reads one model's persisted metadata straight from the
// cache file, independently of the registry that wrote it.
func readCachedMetadata(t *testing.T, cachePath, providerName, modelID string) *core.ModelMetadata {
	t.Helper()

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	var cache modelcache.ModelCache
	if err := json.Unmarshal(data, &cache); err != nil {
		t.Fatalf("unmarshal cache: %v", err)
	}
	provider, ok := cache.Providers[providerName]
	if !ok {
		t.Fatalf("provider %q missing from cache", providerName)
	}
	for _, model := range provider.Models {
		if model.ID == modelID {
			return model.Metadata
		}
	}
	t.Fatalf("model %q missing from cache", modelID)
	return nil
}

// TestProviderReportedCapabilitiesCarryDiscoveredProvenance covers the case the
// catalog test cannot reach: a deployment with no model list configured, where
// nothing enriches the provider's report. The origin of a capability a provider
// reports has to be recorded where that report becomes registry state, on both
// the live fetch and the cache restore path - otherwise /v1/models publishes
// capabilities with an empty source map and no consumer can tell where the
// value came from.
func TestProviderReportedCapabilitiesCarryDiscoveredProvenance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	live := newTestRegistry(t, reportedModels(), nil)
	if err := live.Initialize(ctx); err != nil {
		t.Fatalf("registry Initialize: %v", err)
	}

	cache := modelcache.NewLocalCache(filepath.Join(t.TempDir(), "models.json"))
	writer := newTestRegistry(t, reportedModels(), nil)
	writer.SetCache(cache)
	if err := writer.Initialize(ctx); err != nil {
		t.Fatalf("registry Initialize: %v", err)
	}
	if err := writer.SaveToCache(ctx); err != nil {
		t.Fatalf("SaveToCache: %v", err)
	}

	restored := newTestRegistry(t, nil, errors.New("provider unreachable"))
	restored.SetCache(cache)
	if _, err := restored.LoadFromCache(ctx); err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}

	for name, registry := range map[string]*providers.ModelRegistry{
		"live fetch":    live,
		"cache restore": restored,
	} {
		meta := lookupMetadata(t, registry)
		for _, capability := range []string{"function_calling", "vision"} {
			if !meta.Capabilities[capability] {
				t.Errorf("%s: %s missing: %v", name, capability, meta.Capabilities)
				continue
			}
			if got := meta.CapabilitySources[capability]; got != core.CapSrcDiscovered {
				t.Errorf("%s: %s source = %q, want %q", name, capability, got, core.CapSrcDiscovered)
			}
		}
	}
}
