package modeltest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/enterpilot/gomodel/internal/core"
)

// stubChat answers with scripted responses / errors.
type stubChat struct {
	resp *core.ChatResponse
	err  error
}

func (s *stubChat) ChatCompletion(context.Context, *core.ChatRequest) (*core.ChatResponse, error) {
	return s.resp, s.err
}

type stubEmbed struct {
	resp *core.EmbeddingResponse
	err  error
}

func (s *stubEmbed) Embeddings(context.Context, *core.EmbeddingRequest) (*core.EmbeddingResponse, error) {
	return s.resp, s.err
}

func chatResp(withTool bool) *core.ChatResponse {
	msg := core.ResponseMessage{Content: "pong"}
	if withTool {
		msg.ToolCalls = []core.ToolCall{{ID: "c1", Type: "function", Function: core.FunctionCall{Name: "get_weather"}}}
	}
	return &core.ChatResponse{Choices: []core.Choice{{Message: msg}}}
}

var target = Target{Provider: "local", Model: "my-model"}

func TestProbeChatPass(t *testing.T) {
	t.Parallel()
	tr := New(&stubChat{resp: chatResp(false)}, nil, Config{})
	r := tr.Probe(context.Background(), target, ProbeChat)
	if r.Verdict != Pass {
		t.Fatalf("chat probe = %s (%s), want pass", r.Verdict, r.Detail)
	}
}

func TestProbeTransportErrorIsInconclusive(t *testing.T) {
	t.Parallel()
	tr := New(&stubChat{err: errors.New("connection refused")}, nil, Config{})
	for _, probe := range []Probe{ProbeChat, ProbeFunctionCalling} {
		r := tr.Probe(context.Background(), target, probe)
		if r.Verdict != Inconclusive {
			t.Fatalf("%s probe on transport error = %s, want inconclusive", probe, r.Verdict)
		}
	}
}

func TestProbeFunctionCallingPassAndSilentIgnore(t *testing.T) {
	t.Parallel()
	pass := New(&stubChat{resp: chatResp(true)}, nil, Config{}).
		Probe(context.Background(), target, ProbeFunctionCalling)
	if pass.Verdict != Pass {
		t.Fatalf("tool_calls observed = %s, want pass", pass.Verdict)
	}
	ignore := New(&stubChat{resp: chatResp(false)}, nil, Config{}).
		Probe(context.Background(), target, ProbeFunctionCalling)
	if ignore.Verdict != Inconclusive {
		t.Fatalf("silent ignore = %s, want inconclusive (negative never asserted)", ignore.Verdict)
	}
}

func TestProbeToolRejectionDetail(t *testing.T) {
	t.Parallel()
	reject := core.NewProviderError("local", 400, "The model does not support the tools parameter", nil)
	r := New(&stubChat{err: reject}, nil, Config{}).Probe(context.Background(), target, ProbeFunctionCalling)
	if r.Verdict != Inconclusive {
		t.Fatalf("tool rejection = %s, want inconclusive", r.Verdict)
	}
	if r.Detail == "" {
		t.Fatal("rejection detail lost")
	}
}

func TestProbeEmbeddings(t *testing.T) {
	t.Parallel()
	ok := New(nil, &stubEmbed{resp: &core.EmbeddingResponse{Data: []core.EmbeddingData{{Embedding: json.RawMessage("[0.1,0.2]")}}}}, Config{}).
		Probe(context.Background(), target, ProbeEmbeddings)
	if ok.Verdict != Pass {
		t.Fatalf("embeddings = %s, want pass", ok.Verdict)
	}
	empty := New(nil, &stubEmbed{resp: &core.EmbeddingResponse{}}, Config{}).
		Probe(context.Background(), target, ProbeEmbeddings)
	if empty.Verdict != Inconclusive {
		t.Fatalf("empty vectors = %s, want inconclusive", empty.Verdict)
	}
	missing := New(nil, nil, Config{}).Probe(context.Background(), target, ProbeEmbeddings)
	if missing.Verdict != Inconclusive {
		t.Fatal("missing prober must not panic and must be inconclusive")
	}
}

func TestProbeAllBoundedAndOrdered(t *testing.T) {
	t.Parallel()
	tr := New(slowProber{}, nil, Config{MaxParallel: 2, Timeout: time.Second})
	results := tr.ProbeAll(context.Background(), []Target{
		target, target, target, target, target, target,
	}, []Probe{ProbeChat})
	if len(results) != 6 {
		t.Fatalf("ProbeAll returned %d results, want 6", len(results))
	}
	for _, r := range results {
		if r.Verdict != Pass {
			t.Fatalf("result = %+v, want all pass", r)
		}
	}
}

// slowProber counts peak concurrency to prove MaxParallel is honored.
type slowProber struct{}

func (slowProber) ChatCompletion(ctx context.Context, _ *core.ChatRequest) (*core.ChatResponse, error) {
	select {
	case <-time.After(20 * time.Millisecond):
		return chatResp(false), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestTargetQualified(t *testing.T) {
	if got := (Target{Provider: "openai", Model: "gpt"}).Qualified(); got != "openai/gpt" {
		t.Fatalf("Qualified = %q", got)
	}
}
