package modeltest

import (
	"context"
	"sync"
	"time"

	"github.com/enterpilot/gomodel/internal/core"
)

// Target names one provider/model pair to probe.
type Target struct {
	Provider string
	Model    string
}

// Qualified is the "provider/model" key used in requests and results.
func (t Target) Qualified() string { return t.Provider + "/" + t.Model }

// Config bounds the tester's blast radius.
type Config struct {
	// MaxParallel caps concurrent upstream probe calls (default 4).
	MaxParallel int
	// Timeout bounds one probe call (default 30s).
	Timeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.MaxParallel <= 0 {
		c.MaxParallel = 4
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	return c
}

// Tester runs offline probes against live providers. It is safe for
// concurrent use; one semaphore bounds total upstream pressure regardless of
// how many callers queue results.
type Tester struct {
	chat      ChatProber
	embedding EmbeddingProber
	cfg       Config
}

// New builds a tester over the provider router (either interface may be nil
// when the deployment has no such endpoint surface).
func New(chat ChatProber, embedding EmbeddingProber, cfg Config) *Tester {
	return &Tester{chat: chat, embedding: embedding, cfg: cfg.withDefaults()}
}

// ProbeAll runs the requested probes over targets with bounded concurrency
// and returns results in completion order.
func (t *Tester) ProbeAll(ctx context.Context, targets []Target, probes []Probe) []Result {
	sem := make(chan struct{}, t.cfg.MaxParallel)
	out := make(chan Result, len(targets)*len(probes))
	var wg sync.WaitGroup
	for _, target := range targets {
		for _, probe := range probes {
			wg.Add(1)
			go func(target Target, probe Probe) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				out <- t.Probe(ctx, target, probe)
			}(target, probe)
		}
	}
	wg.Wait()
	close(out)
	results := make([]Result, 0, len(targets)*len(probes))
	for r := range out {
		results = append(results, r)
	}
	return results
}

// Probe executes a single probe against a single target.
func (t *Tester) Probe(ctx context.Context, target Target, probe Probe) Result {
	result := Result{Provider: target.Provider, Model: target.Model, Probe: probe, At: time.Now()}
	ctx, cancel := context.WithTimeout(ctx, t.cfg.Timeout)
	defer cancel()
	switch probe {
	case ProbeChat:
		t.probeChat(ctx, target, &result)
	case ProbeEmbeddings:
		t.probeEmbeddings(ctx, target, &result)
	case ProbeFunctionCalling:
		t.probeFunctionCalling(ctx, target, &result)
	default:
		result.Verdict, result.Detail = Inconclusive, "unknown probe"
	}
	return result
}

func (t *Tester) probeChat(ctx context.Context, target Target, result *Result) {
	if t.chat == nil {
		result.Verdict, result.Detail = Inconclusive, "chat prober unavailable"
		return
	}
	resp, elapsed, err := runChatProbe(ctx, t.chat, target.Model, false)
	result.LatencyMs = elapsed.Milliseconds()
	if err != nil {
		result.Verdict, result.Detail = Inconclusive, errorText(err)
		return
	}
	if resp == nil || len(resp.Choices) == 0 {
		result.Verdict, result.Detail = Inconclusive, "empty choices"
		return
	}
	result.Verdict = Pass
}

func (t *Tester) probeEmbeddings(ctx context.Context, target Target, result *Result) {
	if t.embedding == nil {
		result.Verdict, result.Detail = Inconclusive, "embedding prober unavailable"
		return
	}
	start := time.Now()
	resp, err := t.embedding.Embeddings(ctx, &core.EmbeddingRequest{Model: target.Model, Input: "ping"})
	result.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		result.Verdict, result.Detail = Inconclusive, errorText(err)
		return
	}
	if resp == nil || len(resp.Data) == 0 {
		result.Verdict, result.Detail = Inconclusive, "no embedding vectors returned"
		return
	}
	result.Verdict = Pass
}

func (t *Tester) probeFunctionCalling(ctx context.Context, target Target, result *Result) {
	if t.chat == nil {
		result.Verdict, result.Detail = Inconclusive, "chat prober unavailable"
		return
	}
	resp, elapsed, err := runChatProbe(ctx, t.chat, target.Model, true)
	result.LatencyMs = elapsed.Milliseconds()
	if err != nil {
		if ToolRejectionErr(err) {
			// The only negative this probe may assert: an explicit 4xx
			// rejection of the tools parameter. Advisory until confirmed.
			result.Verdict = Inconclusive
			result.Detail = "upstream rejected tools parameter: " + errorText(err)
			return
		}
		result.Verdict, result.Detail = Inconclusive, errorText(err)
		return
	}
	if ToolCallObserved(resp) {
		result.Verdict = Pass
		return
	}
	// Silent ignore or prose answer: no verdict either way.
	result.Verdict, result.Detail = Inconclusive, "no tool_calls observed"
}
