package virtualmodels

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

// T: the request pipeline resolves a model twice — once before the body is
// summarized, once after — so content-aware selectors see the request content.
// That second resolution only earns its cost for strategies that choose from
// content: for the others it rotates the round-robin cursor a second time for a
// single request, which pins a two-target alias to one target and strands the
// other until a failover.
func TestRouteContentNeeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strategy string
		scoped   string
		request  string
		userPath string
		want     bool
	}{
		{name: "round robin chooses from the target list alone", strategy: StrategyRoundRobin, request: "chat", want: false},
		{name: "an empty strategy means round robin", strategy: "", request: "chat", want: false},
		{name: "cost ranks by price, not content", strategy: StrategyCost, request: "chat", want: false},
		{name: "failover serves declared order", strategy: StrategyFailover, request: "chat", want: false},
		{name: "adaptive selects on request content", strategy: StrategyAdaptive, request: "chat", want: true},
		{name: "plugin strategies select on request content", strategy: StrategyPlugin, request: "chat", want: true},
		{name: "a model that is not an alias needs nothing", strategy: StrategyAdaptive, request: "gpt-4o", want: false},
		{name: "an alias scoped to another user path is not this caller's", strategy: StrategyAdaptive, scoped: "/other", request: "chat", userPath: "/team", want: false},
		{name: "an alias scoped to this user path applies", strategy: StrategyAdaptive, scoped: "/team", request: "chat", userPath: "/team", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			row := VirtualModel{
				Source:   "chat",
				Strategy: tc.strategy,
				Targets: []Target{
					{Provider: "openai", Model: "gpt-4o"},
					{Provider: "groq", Model: "llama"},
				},
				Enabled: true,
			}
			if tc.strategy == StrategyPlugin {
				// A plugin strategy names the plugin it delegates to.
				row.StrategyPlugin = "route-plugin"
			}
			if tc.scoped != "" {
				row.UserPaths = []string{tc.scoped}
			}
			// The row goes in through the store: the service refuses a plugin
			// strategy unless the plugin engine is enabled, and this test is
			// about how a stored row is classified, not about that gate.
			store := newSQLVMStore(t)
			if err := store.Upsert(context.Background(), row); err != nil {
				t.Fatalf("store.Upsert() error = %v", err)
			}
			svc, err := NewService(store, balancingCatalog(), true)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			if err := svc.Refresh(context.Background()); err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}

			ctx := context.Background()
			if tc.userPath != "" {
				ctx = core.WithEffectiveUserPath(ctx, tc.userPath)
			}
			if got := svc.RouteContentNeeded(ctx, core.NewRequestedModelSelector(tc.request, "")); got != tc.want {
				t.Fatalf("RouteContentNeeded(%q) = %v, want %v", tc.request, got, tc.want)
			}
		})
	}
}
