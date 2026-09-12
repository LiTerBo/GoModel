package virtualmodels

import "testing"

// T13-C: the lock guards the pointing, not the row. Deciding whether an edit
// changes the pointing has to mirror how the service normalizes a row on
// write, otherwise an unrelated edit would trip the guard.
func TestResolutionConfigChanged(t *testing.T) {
	openaiTarget := Target{Provider: "openai", Model: "gpt-4o"}
	miniTarget := Target{Provider: "openai", Model: "gpt-4o-mini"}
	redirect := func(mutate func(vm *VirtualModel)) VirtualModel {
		vm := VirtualModel{
			Source:   "smart",
			Targets:  []Target{openaiTarget},
			Strategy: StrategyRoundRobin,
			Enabled:  true,
		}
		if mutate != nil {
			mutate(&vm)
		}
		return vm
	}

	tests := []struct {
		name   string
		stored VirtualModel
		next   VirtualModel
		want   bool
	}{
		{
			// The admin request omits strategy; the stored row has the default.
			name:   "same targets and implied default strategy",
			stored: redirect(nil),
			next:   redirect(func(vm *VirtualModel) { vm.Strategy = "" }),
			want:   false,
		},
		{
			name:   "description change only",
			stored: redirect(nil),
			next:   redirect(func(vm *VirtualModel) { vm.Description = "new note" }),
			want:   false,
		},
		{
			name:   "enabled and scope change only",
			stored: redirect(nil),
			next: redirect(func(vm *VirtualModel) {
				vm.Enabled = false
				vm.UserPaths = []string{"/acme"}
				vm.Slowdown = new(0.5)
			}),
			want: false,
		},
		{
			name:   "target replaced",
			stored: redirect(nil),
			next:   redirect(func(vm *VirtualModel) { vm.Targets = []Target{miniTarget} }),
			want:   true,
		},
		{
			name:   "target added",
			stored: redirect(nil),
			next:   redirect(func(vm *VirtualModel) { vm.Targets = []Target{openaiTarget, miniTarget} }),
			want:   true,
		},
		{
			name:   "weight changed",
			stored: redirect(nil),
			next: redirect(func(vm *VirtualModel) {
				weighted := openaiTarget
				weighted.Weight = 3
				vm.Targets = []Target{weighted}
			}),
			want: true,
		},
		{
			name:   "target order changed",
			stored: redirect(func(vm *VirtualModel) { vm.Targets = []Target{openaiTarget, miniTarget} }),
			next:   redirect(func(vm *VirtualModel) { vm.Targets = []Target{miniTarget, openaiTarget} }),
			want:   true,
		},
		{
			// Duplicates collapse on write, so a request that spells the same
			// target twice is not a change.
			name:   "duplicate target collapses to the stored one",
			stored: redirect(nil),
			next:   redirect(func(vm *VirtualModel) { vm.Targets = []Target{openaiTarget, openaiTarget} }),
			want:   false,
		},
		{
			name:   "strategy changed",
			stored: redirect(nil),
			next:   redirect(func(vm *VirtualModel) { vm.Strategy = StrategyCost }),
			want:   true,
		},
		{
			name:   "strategy case only",
			stored: redirect(func(vm *VirtualModel) { vm.Strategy = StrategyRoundRobin }),
			next:   redirect(func(vm *VirtualModel) { vm.Strategy = "ROUND_ROBIN" }),
			want:   false,
		},
		{
			name: "plugin config changed",
			stored: redirect(func(vm *VirtualModel) {
				vm.Strategy = StrategyPlugin
				vm.StrategyPlugin = "router"
				vm.StrategyConfig = map[string]any{"weight": 1}
			}),
			next: redirect(func(vm *VirtualModel) {
				vm.Strategy = StrategyPlugin
				vm.StrategyPlugin = "router"
				vm.StrategyConfig = map[string]any{"weight": 2}
			}),
			want: true,
		},
		{
			name: "plugin names swapped",
			stored: redirect(func(vm *VirtualModel) {
				vm.Strategy = StrategyPlugin
				vm.StrategyPlugin = "router"
			}),
			next: redirect(func(vm *VirtualModel) {
				vm.Strategy = StrategyPlugin
				vm.StrategyPlugin = "other"
			}),
			want: true,
		},
		{
			// Leftover plugin fields on a non-plugin row are dropped on write.
			name: "plugin fields ignored outside the plugin strategy",
			stored: redirect(func(vm *VirtualModel) {
				vm.StrategyPlugin = "stale"
				vm.StrategyConfig = map[string]any{"weight": 1}
			}),
			next: redirect(func(vm *VirtualModel) {
				vm.StrategyPlugin = "stale"
				vm.StrategyConfig = map[string]any{"weight": 2}
			}),
			want: false,
		},
		{
			name: "policy row keeps a policy row",
			stored: VirtualModel{
				Source:       "openai/gpt-4o",
				ProviderName: "openai",
				Model:        "gpt-4o",
				Enabled:      true,
			},
			next: VirtualModel{
				Source:       "openai/gpt-4o",
				ProviderName: "openai",
				Model:        "gpt-4o",
				Description:  "note",
				Strategy:     "round_robin",
				Enabled:      true,
			},
			want: false,
		},
		{
			name: "policy row turns into a redirect",
			stored: VirtualModel{
				Source:       "openai/gpt-4o",
				ProviderName: "openai",
				Model:        "gpt-4o",
				Enabled:      true,
			},
			next: redirect(func(vm *VirtualModel) { vm.Source = "openai/gpt-4o" }),
			want: true,
		},
		{
			name:   "redirect turns into a policy row",
			stored: redirect(nil),
			next: VirtualModel{
				Source:       "smart",
				ProviderName: "openai",
				Model:        "gpt-4o",
				Enabled:      true,
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolutionConfigChanged(tc.stored, tc.next); got != tc.want {
				t.Fatalf("ResolutionConfigChanged(%#v, %#v) = %v, want %v", tc.stored, tc.next, got, tc.want)
			}
		})
	}
}
