package gateway

import (
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
)

func TestBuildRequiredCapabilities_Chat(t *testing.T) {
	t.Parallel()
	text := &core.ChatRequest{Messages: []core.Message{{Role: "user", Content: "hi"}}}
	if got := BuildRequiredCapabilities(text); got != nil {
		t.Fatalf("plain text request demands %v, want none", got)
	}

	tools := &core.ChatRequest{Messages: text.Messages, Tools: []map[string]any{{"type": "function"}}}
	if got := BuildRequiredCapabilities(tools); len(got) != 1 || got[0] != "function_calling" {
		t.Fatalf("tools request demands %v, want [function_calling]", got)
	}

	image := &core.ChatRequest{Messages: []core.Message{{Role: "user", Content: []core.ContentPart{
		{Type: "text", Text: "what is this?"},
		{Type: "image_url", ImageURL: &core.ImageURLContent{URL: "https://x/y.png"}},
	}}}}
	got := BuildRequiredCapabilities(image)
	if len(got) != 1 || got[0] != "vision" {
		t.Fatalf("image request demands %v, want [vision]", got)
	}

	both := &core.ChatRequest{Messages: image.Messages, Tools: tools.Tools}
	got = BuildRequiredCapabilities(both)
	if len(got) != 2 {
		t.Fatalf("tools+image request demands %v, want both", got)
	}
}

func TestBuildRouteContent_Chat(t *testing.T) {
	t.Parallel()
	if BuildRouteContent(nil) != nil {
		t.Fatal("nil request must summarize to nil")
	}
	req := &core.ChatRequest{Messages: []core.Message{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "1. step\n2. step\n3. step\n\n```go\nfunc main(){}\n```"},
	}, Tools: []map[string]any{{"type": "function"}, {"type": "function"}}}
	content := BuildRouteContent(req)
	if content.MessageCount != 4 || content.UserTurns != 2 {
		t.Fatalf("counts = %d/%d, want 4/2", content.MessageCount, content.UserTurns)
	}
	if content.ToolDefs != 2 {
		t.Fatalf("tool defs = %d, want 2", content.ToolDefs)
	}
	if !content.HasCode || !content.HasMultiStep {
		t.Fatalf("features = code %v multistep %v, want both", content.HasCode, content.HasMultiStep)
	}
	last := req.Messages[3].Content.(string)
	if content.CurrentChars != len(last) {
		t.Fatalf("current chars = %d, want %d", content.CurrentChars, len(last))
	}
	if content.AvgUserChars <= 0 || content.ContextChars <= 0 {
		t.Fatalf("derived lengths = avg %d ctx %d, want positive", content.AvgUserChars, content.ContextChars)
	}
}

func TestBuildRouteContentResponses(t *testing.T) {
	t.Parallel()
	if BuildRouteContentResponses(nil) != nil {
		t.Fatal("nil responses request must summarize to nil")
	}
	req := &core.ResponsesRequest{Input: []core.ResponsesInputElement{
		{Type: "message", Role: "user", Content: "hello there"},
		{Type: "message", Role: "user", Content: []core.ContentPart{
			{Type: "text", Text: "describe"},
			{Type: "input_image", ImageURL: &core.ImageURLContent{URL: "https://x/y.png"}},
		}},
	}, Tools: []map[string]any{{"type": "function"}}}
	content := BuildRouteContentResponses(req)
	if content.MessageCount != 2 || content.UserTurns != 2 || content.ToolDefs != 1 {
		t.Fatalf("counts = %+v, want 2 messages/2 turns/1 tool", content)
	}
	got := BuildRequiredCapabilitiesResponses(req)
	if len(got) != 2 {
		t.Fatalf("responses request demands %v, want function_calling + vision", got)
	}
	// Plain string input still summarizes.
	str := &core.ResponsesRequest{Input: "just a string question"}
	if s := BuildRouteContentResponses(str); s == nil || s.CurrentChars != len("just a string question") {
		t.Fatalf("string input summary = %+v", s)
	}
}
