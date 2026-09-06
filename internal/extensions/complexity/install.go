package complexity

import (
	"context"
	"log/slog"

	"github.com/enterpilot/gomodel/config"
	"github.com/enterpilot/gomodel/ext"
)

// ConfigKey names this extension's section under `extensions:` in
// config.yaml: extensions.complexity_routing.
const ConfigKey = "complexity_routing"

// InstallToDefault adapts Install to run.Options.SetupConfig, registering on
// the shared ext.Default registry the standard gateway consumes.
func InstallToDefault(_ context.Context, result *config.LoadResult) error {
	return Install(ext.Default, result)
}

// Install decodes extensions.complexity_routing and, when enabled, registers
// the complexity selector and its thresholds setting on the registry. A
// missing or disabled section leaves the gateway untouched (adaptive
// redirects keep falling back to weighted round robin).
func Install(reg *ext.Registry, result *config.LoadResult) error {
	if reg == nil || result == nil {
		return nil
	}
	var cfg Config
	ok, err := result.DecodeExtension(ConfigKey, &cfg)
	if err != nil {
		return err
	}
	if !ok || !cfg.Enabled {
		return nil
	}
	selector := NewSelector(cfg)
	reg.RegisterRouteSelector(selector)
	reg.RegisterSetting(NewThresholdSetting(selector))
	slog.Info("complexity routing enabled", "virtual_model", cfg.VirtualModel)
	return nil
}
