package app

import (
	"context"
	"time"

	"github.com/enterpilot/gomodel/internal/modeldata/modeltest"
)

// adminModelTestProber adapts modeltest.Tester's batch signature to the
// admin surface's single-target signature.
type adminModelTestProber struct {
	tester *modeltest.Tester
}

func (a adminModelTestProber) Probe(provider, model string, probes []modeltest.Probe) []modeltest.Result {
	ctx, cancel := context.WithTimeout(context.Background(), probeRunTimeout)
	defer cancel()
	return a.tester.ProbeAll(ctx, []modeltest.Target{{Provider: provider, Model: model}}, probes)
}

// probeRunTimeout bounds one admin-triggered probe run server-side; the
// tester applies its own per-probe timeout on top.
const probeRunTimeout = 2 * time.Minute
