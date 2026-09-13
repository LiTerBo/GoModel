//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/enterpilot/gomodel/internal/admin"
	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/virtualmodels"
)

// — in-memory store and catalog (no DB needed) ------------------------------

type inMemoryVMStore struct {
	mu  sync.RWMutex
	vms map[string]virtualmodels.VirtualModel
}

func newInMemoryVMStore() *inMemoryVMStore {
	return &inMemoryVMStore{vms: map[string]virtualmodels.VirtualModel{}}
}

func (s *inMemoryVMStore) List(_ context.Context) ([]virtualmodels.VirtualModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]virtualmodels.VirtualModel, 0, len(s.vms))
	for _, vm := range s.vms {
		out = append(out, vm)
	}
	return out, nil
}

func (s *inMemoryVMStore) Get(_ context.Context, source string) (*virtualmodels.VirtualModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vm, ok := s.vms[source]
	if !ok {
		return nil, virtualmodels.ErrNotFound
	}
	return &vm, nil
}

func (s *inMemoryVMStore) Upsert(_ context.Context, vm virtualmodels.VirtualModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	vm.UpdatedAt = time.Now()
	if existing, ok := s.vms[vm.Source]; ok {
		vm.CreatedAt = existing.CreatedAt
	} else {
		vm.CreatedAt = time.Now()
	}
	s.vms[vm.Source] = vm
	return nil
}

func (s *inMemoryVMStore) Delete(_ context.Context, source string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vms, source)
	return nil
}

func (s *inMemoryVMStore) Close() error { return nil }

type mockCatalog struct{}

func (m *mockCatalog) Supports(model string) bool          { return strings.HasPrefix(model, "test/") }
func (m *mockCatalog) ModelAvailable(model string) bool     { return strings.HasPrefix(model, "test/") }
func (m *mockCatalog) GetProviderType(_ string) string      { return "test" }
func (m *mockCatalog) LookupModel(_ string) (*core.Model, bool) { return &core.Model{ID: "m1"}, true }
func (m *mockCatalog) ProviderNames() []string              { return []string{"test"} }

// — helpers ------------------------------------------------------------------

func doReq(t *testing.T, url, method, bearer string, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, url, rdr)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// — test ---------------------------------------------------------------------

func TestAliasCRUD_E2E(t *testing.T) {
	vmStore := newInMemoryVMStore()
	vmSvc, err := virtualmodels.NewService(vmStore, &mockCatalog{}, true)
	require.NoError(t, err)

	opts := e2eServerOptions{
		adminEndpointsEnabled: true,
		adminOptions: []admin.Option{
			admin.WithVirtualModels(vmSvc),
		},
	}
	srv := setupE2EAdminServer(t, opts)
	defer srv.Close()

	base := srv.URL + "/admin/virtual-models"
	put := func(t *testing.T, body any) *http.Response {
		return doReq(t, base, http.MethodPut, "", body)
	}
	del := func(t *testing.T, body any) *http.Response {
		return doReq(t, base, http.MethodDelete, "", body)
	}
	get := func(t *testing.T, path string) *http.Response {
		return doReq(t, base+path, http.MethodGet, "", nil)
	}
	body := func(t *testing.T, resp *http.Response) string {
		t.Helper()
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return string(b)
	}

	// 1. Create alias
	t.Log("Step 1: Create alias via PUT")
	r := put(t, map[string]any{"source": "smart", "target_model": "test/gpt-4"})
	require.Equal(t, http.StatusOK, r.StatusCode, "create: %s", body(t, r))

	// 2. List includes the alias
	t.Log("Step 2: GET list includes alias")
	r = get(t, "")
	require.Equal(t, http.StatusOK, r.StatusCode)
	b := body(t, r)
	assert.Contains(t, b, `"source":"smart"`)

	// 3. Authorized-by endpoint works
	t.Log("Step 3: Authorized-by returns grants list")
	r = get(t, "/authorized-by?source=smart")
	require.Equal(t, http.StatusOK, r.StatusCode)
	raw := body(t, r)
	var ab struct {
		Source string `json:"source"`
		Grants []any  `json:"grants"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &ab))
	assert.Equal(t, "smart", ab.Source)
	assert.Empty(t, ab.Grants)

	// 4. Lock alias (metadata-only change → the alias survives it)
	t.Log("Step 4: Lock alias via PUT locked:true")
	r = put(t, map[string]any{"source": "smart", "locked": true})
	// body() consumes the response, so it is read once per reply.
	lockBody := body(t, r)
	require.Equal(t, http.StatusOK, r.StatusCode, "lock: %s", lockBody)
	assert.Contains(t, lockBody, `"locked":true`, "lock not applied: %s", lockBody)
	// A metadata-only write keeps the stored pointing; it used to replace the
	// alias with a pointing-less policy and silently drop it.
	assert.Contains(t, lockBody, `"kind":"redirect"`, "lock dropped the redirect: %s", lockBody)
	assert.Contains(t, lockBody, "test/gpt-4", "lock dropped the target: %s", lockBody)

	// 5. Unlock and retarget in one request
	t.Log("Step 5: Unlock (locked:false) and change target")
	r = put(t, map[string]any{
		"source": "smart", "target_model": "test/gpt-4-turbo", "locked": false,
	})
	retargetBody := body(t, r)
	require.Equal(t, http.StatusOK, r.StatusCode, "unlock+retarget: %s", retargetBody)
	assert.Contains(t, retargetBody, "test/gpt-4-turbo", "retarget did not apply: %s", retargetBody)

	// 6. Delete alias (no auth refs → 204)
	t.Log("Step 6: Delete alias")
	r = del(t, map[string]string{"source": "smart"})
	require.Equal(t, http.StatusNoContent, r.StatusCode, "delete: %s", body(t, r))

	// 7. Verify deletion via GET
	t.Log("Step 7: List no longer contains deleted alias")
	r = get(t, "")
	b = body(t, r)
	assert.NotContains(t, b, `"source":"smart"`)

	t.Log("T13-H E2E: all steps passed.")
}