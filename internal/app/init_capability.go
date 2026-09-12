package app

import (
	"fmt"
	"log/slog"

	"github.com/enterpilot/gomodel/internal/capability"
)

// initCapabilities loads persisted operator capability confirmations and
// replays them into the model registry, so confirmed verdicts survive
// restarts. It needs only storage and the registry (built by initProviders),
// and must run before initAdmin wires the confirmation endpoint.
//
// This phase is why confirmations also survive configuration reloads: run.Run
// rebuilds the whole application per configuration generation, so every
// generation runs the phase again and replays into the registry it just built.
// Keep it in phases() and keep it after initProviders.
func (b *bootstrap) initCapabilities() error {
	app := b.app

	if app.storage == nil {
		return fmt.Errorf("storage is required for capability confirmations")
	}
	registry := app.providers.Registry
	result, err := capability.New(b.ctx, app.storage, registry)
	if err != nil {
		return fmt.Errorf("failed to initialize capability confirmations: %w", err)
	}
	app.capabilities = result
	app.register(subsystemCapabilityConfirmations, ownedByShutdown, app.capabilities.Close)

	loaded, loadErr := result.Store.List(b.ctx)
	if loadErr != nil {
		// Refresh already validated the store, so a failed count only means
		// no reliable number for the log line — not a broken subsystem.
		slog.Info("capability confirmations loaded into registry", "count", "unknown")
		return nil
	}
	slog.Info("capability confirmations loaded into registry", "count", len(loaded))
	return nil
}
