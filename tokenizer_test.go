package btdby4_test

import (
	json "github.com/goccy/go-json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/italoalmeida0/btdby4"
)

func TestCountText(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"'RE", 1},
		{"hello world", 2},
		{"hello  world", 3},
		{"hello   world", 3},
		{"supercalifragilistic", 7},
		{"We know what we are, but know not what we may be.", 14},
		{"don't", 2},
		{"1234567", 3},
		{"", 0},
		{" ", 1},
		{`{"city":"SP"}`, 5},
		{"get_weather", 2},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, err := btdby4.CountText(tt.text)
			if err != nil {
				t.Fatalf("CountText error: %v", err)
			}
			if got != tt.want {
				t.Errorf("CountText(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestCountRequest(t *testing.T) {
	req := btdby4.AnthropicRequest{
		System: "You are a helpful assistant.",
		Messages: []btdby4.AnthropicMessage{
			{Role: "user", Content: "hello world"},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "text", "text": "hi there"},
				map[string]any{"type": "tool_use", "id": "toolu_1", "name": "get_weather", "input": map[string]any{"city": "SP"}},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "sunny"},
			}},
		},
		Tools: []btdby4.AnthropicTool{
			{Name: "get_weather", Description: "Get the current weather", InputSchema: map[string]any{"type": "object"}},
		},
	}
	out, err := btdby4.CountAnthropicRequest(req, btdby4.Options{Tight: true})
	if err != nil {
		t.Fatalf("CountAnthropicRequest error: %v", err)
	}
	sys, _ := btdby4.CountText("You are a helpful assistant.")
	m0, _ := btdby4.CountText("hello world")
	m1a, _ := btdby4.CountText("hi there")
	m1b, _ := btdby4.CountText("get_weather")
	m1c, _ := btdby4.CountText(`{"city":"SP"}`)
	m2, _ := btdby4.CountText("sunny")
	tn, _ := btdby4.CountText("get_weather")
	td, _ := btdby4.CountText("Get the current weather")
	ts, _ := btdby4.CountText(`{"type":"object"}`)
	wantSys := sys
	wantMsgs := m0 + m1a + m1b + m1c + m2
	wantTools := tn + td + ts
	if out.System != wantSys {
		t.Errorf("System = %d, want %d", out.System, wantSys)
	}
	if out.Messages != wantMsgs {
		t.Errorf("Messages = %d, want %d", out.Messages, wantMsgs)
	}
	if out.Tools != wantTools {
		t.Errorf("Tools = %d, want %d", out.Tools, wantTools)
	}
	if out.Total != wantSys+wantMsgs+wantTools {
		t.Errorf("Total = %d, want %d", out.Total, wantSys+wantMsgs+wantTools)
	}
	if len(out.ByMessage) != 3 || len(out.ByTool) != 1 {
		t.Errorf("breakdown lens: %+v", out)
	}
	raw := []byte(`{"system":"You are a helpful assistant.","messages":[{"role":"user","content":"hello world"}]}`)
	out3, err := btdby4.CountAnthropicRequestJSON(raw, btdby4.Options{})
	if err != nil {
		t.Fatalf("CountAnthropicRequestJSON error: %v", err)
	}
	if out3.Messages != m0 || out3.System != sys {
		t.Errorf("JSON: got %+v", out3)
	}
}

func TestCountImage(t *testing.T) {
	if got := btdby4.CountImageSize(1920, 1080); got != 1560 {
		t.Errorf("CountImageSize(1920,1080) = %d, want 1560", got)
	}
	if got := btdby4.CountImageSize(100, 100); got != 16 {
		t.Errorf("CountImageSize(100,100) = %d, want 16", got)
	}
	if got := btdby4.CountImageBytes([]byte("not an image")); got != 0 {
		t.Errorf("CountImageBytes(invalid) = %d, want 0", got)
	}
	if got := btdby4.CountImageBase64("!!!"); got != 0 {
		t.Errorf("CountImageBase64(invalid) = %d, want 0", got)
	}
}

func TestImageInRequest(t *testing.T) {
	png1x1 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	imgBlock := func() map[string]any {
		return map[string]any{"type": "image", "source": map[string]any{
			"type": "base64", "media_type": "image/png", "data": png1x1,
		}}
	}
	req := btdby4.AnthropicRequest{
		Messages: []btdby4.AnthropicMessage{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "what is this?"},
				imgBlock(),
			}},
		},
	}
	txt, _ := btdby4.CountText("what is this?")
	img := btdby4.CountImageBase64(png1x1)
	out, err := btdby4.CountAnthropicRequest(req, btdby4.Options{Tight: true})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out.Images != img || out.ImageCount != 1 {
		t.Errorf("images: got %+v want %d", out, img)
	}
	if out.Total != txt+img {
		t.Errorf("total = %d, want %d", out.Total, txt+img)
	}
	out2, _ := btdby4.CountAnthropicRequest(req, btdby4.Options{IgnoreImages: true, Tight: true})
	if out2.Images != 0 || out2.Total != txt {
		t.Errorf("excluded: got %+v", out2)
	}
}

func TestCountBlockAndMessage(t *testing.T) {
	blk := map[string]any{"type": "text", "text": "hello world"}
	bb, err := btdby4.CountAnthropicBlock(blk, btdby4.Options{})
	if err != nil {
		t.Fatalf("CountAnthropicBlock error: %v", err)
	}
	if bb.Tokens != 2 || bb.Images != 0 || bb.TextTokens != 2 || bb.ImageCount != 0 {
		t.Errorf("block: got %+v", bb)
	}
	msg := btdby4.AnthropicMessage{Role: "user", Content: "hello world"}
	mb, err := btdby4.CountAnthropicMessage(msg, btdby4.Options{})
	if err != nil {
		t.Fatalf("CountAnthropicMessage error: %v", err)
	}
	if mb.Tokens != 2 {
		t.Errorf("message: got %+v", mb)
	}
	tool := btdby4.AnthropicTool{Name: "get_weather", Description: "Get the current weather", InputSchema: map[string]any{"type": "object"}}
	tn, err := btdby4.CountAnthropicTool(tool)
	if err != nil {
		t.Fatalf("CountAnthropicTool error: %v", err)
	}
	if tn <= 0 {
		t.Errorf("tool: got %d", tn)
	}
}

func TestLargeContextScalesLinearly(t *testing.T) {
	base := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 200)
	n1, _ := btdby4.CountText(base)
	n2, _ := btdby4.CountText(base + base)
	if n2 < 2*n1-2 || n2 > 2*n1+2 {
		t.Errorf("expected near-linear scaling: 1x=%d 2x=%d", n1, n2)
	}
}

func TestCountChatRequest(t *testing.T) {
	req := btdby4.ChatRequest{
		Messages: []btdby4.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "hello world"},
			{Role: "assistant", Content: "hi there", ToolCalls: []btdby4.ChatToolCall{
				{ID: "call_1", Type: "function", Function: btdby4.ChatFunction{Name: "get_weather", Arguments: `{"city":"SP"}`}},
			}},
			{Role: "tool", ToolCallID: "call_1", Content: "sunny"},
		},
		Tools: []btdby4.ChatTool{
			{Type: "function", Function: btdby4.ChatFunctionDef{Name: "get_weather", Description: "Get the current weather", Parameters: map[string]any{"type": "object"}}},
		},
	}
	out, err := btdby4.CountChatRequest(req, btdby4.Options{Tight: true})
	if err != nil {
		t.Fatalf("CountChatRequest error: %v", err)
	}
	sy, _ := btdby4.CountText("You are a helpful assistant.")
	m0, _ := btdby4.CountText("hello world")
	m1a, _ := btdby4.CountText("hi there")
	m1b, _ := btdby4.CountText("get_weather")
	m1c, _ := btdby4.CountText(`{"city":"SP"}`)
	m2, _ := btdby4.CountText("sunny")
	tn, _ := btdby4.CountText("get_weather")
	td, _ := btdby4.CountText("Get the current weather")
	ts, _ := btdby4.CountText(`{"type":"object"}`)
	if out.System != 0 || out.Messages != sy+m0+m1a+m1b+m1c+m2 || out.Tools != tn+td+ts {
		t.Errorf("chat: got %+v", out)
	}
	if len(out.ByMessage) != 4 || len(out.ByTool) != 1 {
		t.Errorf("breakdown lens: %+v", out)
	}
}

func TestCountChatImageURL(t *testing.T) {
	req := btdby4.ChatRequest{
		Messages: []btdby4.ChatMessage{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "what is this?"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="}},
			}},
		},
	}
	txt, _ := btdby4.CountText("what is this?")
	img := btdby4.CountImageBase64("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	out, err := btdby4.CountChatRequest(req, btdby4.Options{Tight: true})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out.Images != img || out.ImageCount != 1 || out.Total != txt+img {
		t.Errorf("chat img: got %+v want img %d", out, img)
	}
	out2, _ := btdby4.CountChatRequest(req, btdby4.Options{IgnoreImages: true, Tight: true})
	if out2.Images != 0 || out2.Total != txt {
		t.Errorf("excluded: got %+v", out2)
	}
}

func TestCountResponsesRequest(t *testing.T) {
	req := btdby4.ResponsesRequest{
		Instructions: "You are a helpful assistant.",
		Input: []any{
			map[string]any{"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "hello world"},
			}},
			map[string]any{"type": "function_call", "name": "get_weather", "arguments": `{"city":"SP"}`, "call_id": "call_1"},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "sunny"},
			map[string]any{"type": "reasoning", "summary": []any{
				map[string]any{"type": "summary_text", "text": "thinking out loud"},
			}},
		},
		Tools: []btdby4.ResponsesTool{
			{Type: "function", Name: "get_weather", Description: "Get the current weather", Parameters: map[string]any{"type": "object"}},
		},
	}
	out, err := btdby4.CountResponsesRequest(req, btdby4.Options{Tight: true})
	if err != nil {
		t.Fatalf("CountResponsesRequest error: %v", err)
	}
	sys, _ := btdby4.CountText("You are a helpful assistant.")
	m0, _ := btdby4.CountText("hello world")
	m1, _ := btdby4.CountText("get_weather")
	m2, _ := btdby4.CountText(`{"city":"SP"}`)
	m3, _ := btdby4.CountText("sunny")
	m4, _ := btdby4.CountText("thinking out loud")
	tn, _ := btdby4.CountText("get_weather")
	td, _ := btdby4.CountText("Get the current weather")
	ts, _ := btdby4.CountText(`{"type":"object"}`)
	if out.System != sys || out.Messages != m0+m1+m2+m3+m4 || out.Tools != tn+td+ts {
		t.Errorf("responses: got %+v", out)
	}
	// Deterministic Fernet-style envelope: raw = 752 + 4.75*T bytes.
	// T=100 -> raw=1227 -> 1636 chars.
	makeEnvelope := func(rawBytes int) string {
		alpha := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
		n := rawBytes * 4 / 3
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = alpha[i%len(alpha)]
		}
		return string(buf)
	}
	encPayload := makeEnvelope(1227)
	req2 := btdby4.ResponsesRequest{
		Input: []any{
			map[string]any{"type": "reasoning", "encrypted_content": encPayload},
		},
	}
	out2, _ := btdby4.CountResponsesRequest(req2, btdby4.Options{})
	if out2.Messages != 100 {
		t.Errorf("encrypted thinking auto: got %+v want 100", out2)
	}
	// Empty payload counts zero.
	req3 := btdby4.ResponsesRequest{
		Input: []any{
			map[string]any{"type": "reasoning", "encrypted_content": ""},
		},
	}
	out3, _ := btdby4.CountResponsesRequest(req3, btdby4.Options{})
	if out3.Messages != 0 {
		t.Errorf("thinking empty: got %+v", out3)
	}
}

func TestEstimateThinkingTokens(t *testing.T) {
	single := []struct {
		name string
		file string
		want int
	}{
		{"openai_resp1", "openai_resp1", 127},
		{"openai_medium", "openai_medium", 109},
		{"openai_hard", "openai_hard", 195},
		{"openai_eff_low", "openai_eff_low", 102},
		{"meta_b1024", "meta_b1024", 1486},
		{"google_chat1", "google_chat1", 767},
		{"google_easy", "google_easy", 102},
		{"google_hard", "google_hard", 1442},
	}
	for _, tc := range single {
		t.Run(tc.name, func(t *testing.T) {
			encs := extractThinkingPayloads(t, tc.file)
			if len(encs) != 1 {
				t.Fatalf("expected 1 payload in %s, got %d", tc.file, len(encs))
			}
			got := btdby4.EstimateThinkingTokens(encs[0])
			lo := tc.want * 80 / 100
			hi := tc.want * 120 / 100
			if got < lo || got > hi {
				t.Errorf("EstimateThinkingTokens(%s) = %d, want ~%d (±20%%)", tc.name, got, tc.want)
			}
		})
	}
	multi := []struct {
		name string
		file string
		want int
	}{
		{"openai_eff_high", "openai_eff_high", 800},
		{"openai_high2", "openai_high2", 3000},
	}
	for _, tc := range multi {
		t.Run(tc.name, func(t *testing.T) {
			encs := extractThinkingPayloads(t, tc.file)
			if len(encs) < 2 {
				t.Fatalf("expected multi payload in %s", tc.file)
			}
			items := make([]any, len(encs))
			for i, e := range encs {
				items[i] = map[string]any{"type": "reasoning", "encrypted_content": e}
			}
			out, err := btdby4.CountResponsesRequest(
				btdby4.ResponsesRequest{Input: items}, btdby4.Options{})
			if err != nil {
				t.Fatal(err)
			}
			lo := tc.want * 80 / 100
			hi := tc.want * 120 / 100
			if out.Messages < lo || out.Messages > hi {
				t.Errorf("multi thinking(%s) = %d, want ~%d (±20%%)", tc.name, out.Messages, tc.want)
			}
		})
	}
	// Anthropic jumbo envelopes via the request path.
	jumbo := []struct {
		name string
		file string
		want int
	}{
		{"meta_msg2", "meta_msg2", 1939},
		{"meta_b2000", "meta_b2000", 2340},
	}
	for _, tc := range jumbo {
		t.Run(tc.name, func(t *testing.T) {
			encs := extractThinkingPayloads(t, tc.file)
			if len(encs) != 1 {
				t.Fatalf("expected 1 payload in %s", tc.file)
			}
			msg := btdby4.AnthropicMessage{Role: "assistant", Content: []any{
				map[string]any{"type": "redacted_thinking", "data": encs[0]},
			}}
			out, err := btdby4.CountAnthropicRequest(
				btdby4.AnthropicRequest{Messages: []btdby4.AnthropicMessage{msg}}, btdby4.Options{})
			if err != nil {
				t.Fatal(err)
			}
			lo := tc.want * 80 / 100
			hi := tc.want * 120 / 100
			if out.Messages < lo || out.Messages > hi {
				t.Errorf("jumbo thinking(%s) = %d, want ~%d (±20%%)", tc.name, out.Messages, tc.want)
			}
		})
	}
}

func extractThinkingPayloads(t *testing.T, name string) []string {
	t.Helper()
	dir := os.Getenv("BTDBY4_THINKING_FIXTURES")
	if dir == "" {
		t.Skip("BTDBY4_THINKING_FIXTURES not set (private provider payloads)")
	}
	raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		t.Skipf("fixture %s not found", name)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("bad fixture %s: %v", name, err)
	}
	var out []string
	if items, ok := v["output"].([]any); ok {
		for _, o := range items {
			if m, ok := o.(map[string]any); ok && m["type"] == "reasoning" {
				if e, ok := m["encrypted_content"].(string); ok && e != "" {
					out = append(out, e)
				}
			}
		}
	}
	if content, ok := v["content"].([]any); ok {
		for _, c := range content {
			if m, ok := c.(map[string]any); ok && m["type"] == "redacted_thinking" {
				if e, ok := m["data"].(string); ok && e != "" {
					out = append(out, e)
				}
			}
		}
	}
	if choices, ok := v["choices"].([]any); ok && len(choices) > 0 {
		if m, ok := choices[0].(map[string]any)["message"].(map[string]any); ok {
			if ex, ok := m["extra_content"].(map[string]any); ok {
				if g, ok := ex["google"].(map[string]any); ok {
					if s, ok := g["thought_signature"].(string); ok && s != "" {
						out = append(out, s)
					}
				}
			}
		}
	}
	return out
}

func TestVisibleThinking(t *testing.T) {
	if bb, err := btdby4.CountAnthropicBlock(
		map[string]any{"type": "thinking", "thinking": "hello world"}, btdby4.Options{}); err != nil || bb.Tokens != 2 {
		t.Errorf("anthropic thinking: got %+v err %v", bb, err)
	}
	if pb, err := btdby4.CountChatPart(
		map[string]any{"type": "reasoning_content", "text": "hello world"}, btdby4.Options{}); err != nil || pb.Tokens != 2 {
		t.Errorf("chat reasoning_content: got %+v err %v", pb, err)
	}
	if pb, err := btdby4.CountResponsesPart(
		map[string]any{"type": "reasoning_text", "text": "hello world"}, btdby4.Options{}); err != nil || pb.Tokens != 2 {
		t.Errorf("responses reasoning_text: got %+v err %v", pb, err)
	}
}

func TestFullPayloadParity(t *testing.T) {
	mkEnv := func(raw int) string {
		alpha := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
		n := raw * 4 / 3
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = alpha[i%len(alpha)]
		}
		return string(buf)
	}
	enc := mkEnv(1227) // ~100 thinking tokens

	anth := btdby4.AnthropicRequest{
		System: "You are a helpful assistant.",
		Tools: []btdby4.AnthropicTool{
			{Name: "get_weather", Description: "Get weather", InputSchema: map[string]any{"type": "object"}},
		},
		Messages: []btdby4.AnthropicMessage{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "weather in SP?"},
				map[string]any{"type": "image", "source": map[string]any{"type": "url", "url": "https://x/y.png"}},
			}},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "thinking", "thinking": "let me think step by step"},
				map[string]any{"type": "redacted_thinking", "data": enc},
				map[string]any{"type": "tool_use", "id": "t1", "name": "get_weather", "input": map[string]any{"city": "SP"}},
			}},
			{Role: "user", Content: []any{
				map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "sunny 25C"},
			}},
		},
	}
	a, err := btdby4.CountAnthropicRequest(anth, btdby4.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if a.System != 6 || len(a.ByMessage) != 3 || a.Tools != 9 || a.ImageCount != 1 {
		t.Errorf("anthropic full: got %+v", a)
	}
	if a.ByMessage[1] < 100 {
		t.Errorf("anthropic full: thinking missing in %+v", a)
	}

	chat := btdby4.ChatRequest{
		System: "You are a helpful assistant.",
		Tools:  []btdby4.ChatTool{{Type: "function", Function: btdby4.ChatFunctionDef{Name: "get_weather"}}},
		Messages: []btdby4.ChatMessage{
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "weather in SP?"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x/y.png"}},
			}},
			{Role: "assistant", Content: []any{
				map[string]any{"type": "reasoning_content", "text": "let me think step by step"},
				map[string]any{"type": "text", "text": "checking"},
			}, ToolCalls: []btdby4.ChatToolCall{{ID: "c1", Type: "function", Function: btdby4.ChatFunction{Name: "get_weather", Arguments: `{"city":"SP"}`}}}},
			{Role: "tool", ToolCallID: "c1", Content: "sunny 25C"},
		},
	}
	c, err := btdby4.CountChatRequest(chat, btdby4.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if c.System != 6 || len(c.ByMessage) != 3 || c.Tools != 2 || c.ImageCount != 1 {
		t.Errorf("chat full: got %+v", c)
	}
	if c.ByMessage[1] < 10 {
		t.Errorf("chat full: thinking/tool calls missing in %+v", c)
	}

	resp := btdby4.ResponsesRequest{
		Instructions: "You are a helpful assistant.",
		Tools:        []btdby4.ResponsesTool{{Type: "function", Name: "get_weather"}},
		Input: []any{
			map[string]any{"type": "message", "role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "weather in SP?"},
				map[string]any{"type": "input_image", "image_url": "https://x/y.png"},
			}},
			map[string]any{"type": "reasoning", "summary": []any{
				map[string]any{"type": "summary_text", "text": "thinking about weather"},
			}, "encrypted_content": enc},
			map[string]any{"type": "function_call", "name": "get_weather", "arguments": `{"city":"SP"}`},
			map[string]any{"type": "function_call_output", "output": "sunny 25C"},
		},
	}
	r, err := btdby4.CountResponsesRequest(resp, btdby4.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.System != 6 || len(r.ByMessage) != 4 || r.Tools != 2 || r.ImageCount != 1 {
		t.Errorf("responses full: got %+v", r)
	}
	if r.ByMessage[1] < 100 {
		t.Errorf("responses full: thinking missing in %+v", r)
	}

	// Anthropic granularities.
	msg := btdby4.AnthropicMessage{Role: "user", Content: "hello world"}
	if mb, err := btdby4.CountAnthropicMessage(msg, btdby4.Options{}); err != nil || mb.Tokens != 2 {
		t.Errorf("CountAnthropicMessage: got %+v err %v", mb, err)
	}
	if _, err := btdby4.CountAnthropicMessageJSON([]byte(`{"role":"user","content":"hello world"}`), btdby4.Options{}); err != nil {
		t.Errorf("CountAnthropicMessageJSON: %v", err)
	}
	if _, err := btdby4.CountAnthropicBlockJSON([]byte(`{"type":"text","text":"hello world"}`), btdby4.Options{}); err != nil {
		t.Errorf("CountAnthropicBlockJSON: %v", err)
	}
	if _, err := btdby4.CountAnthropicToolJSON([]byte(`{"name":"get_weather"}`)); err != nil {
		t.Errorf("CountAnthropicToolJSON: %v", err)
	}

	// Chat granularities.
	cmsg := btdby4.ChatMessage{Role: "user", Content: "hello world"}
	if mb, err := btdby4.CountChatMessage(cmsg, btdby4.Options{}); err != nil || mb.Tokens != 2 {
		t.Errorf("CountChatMessage: got %+v err %v", mb, err)
	}
	if _, err := btdby4.CountChatMessageJSON([]byte(`{"role":"user","content":"hello world"}`), btdby4.Options{}); err != nil {
		t.Errorf("CountChatMessageJSON: %v", err)
	}
	if pb, err := btdby4.CountChatPart(map[string]any{"type": "text", "text": "hello world"}, btdby4.Options{}); err != nil || pb.Tokens != 2 {
		t.Errorf("CountChatPart: got %+v err %v", pb, err)
	}
	if _, err := btdby4.CountChatPartJSON([]byte(`{"type":"text","text":"hello world"}`), btdby4.Options{}); err != nil {
		t.Errorf("CountChatPartJSON: %v", err)
	}
	if _, err := btdby4.CountChatTool(btdby4.ChatTool{Function: btdby4.ChatFunctionDef{Name: "get_weather"}}); err != nil {
		t.Errorf("CountChatTool: %v", err)
	}
	if _, err := btdby4.CountChatToolJSON([]byte(`{"function":{"name":"get_weather"}}`)); err != nil {
		t.Errorf("CountChatToolJSON: %v", err)
	}
	if _, err := btdby4.CountChatRequestJSON([]byte(`{"messages":[{"role":"user","content":"hello world"}]}`), btdby4.Options{}); err != nil {
		t.Errorf("CountChatRequestJSON: %v", err)
	}

	// Responses granularities.
	item := map[string]any{"type": "function_call", "name": "get_weather", "arguments": `{"city":"SP"}`}
	if ib, err := btdby4.CountResponsesItem(item, btdby4.Options{}); err != nil || ib.Tokens <= 0 {
		t.Errorf("CountResponsesItem: got %+v err %v", ib, err)
	}
	if _, err := btdby4.CountResponsesItemJSON([]byte(`{"type":"function_call","name":"get_weather","arguments":"{}"} `), btdby4.Options{}); err != nil {
		t.Errorf("CountResponsesItemJSON: %v", err)
	}
	part := map[string]any{"type": "input_text", "text": "hello world"}
	if pb, err := btdby4.CountResponsesPart(part, btdby4.Options{}); err != nil || pb.Tokens != 2 {
		t.Errorf("CountResponsesPart: got %+v err %v", pb, err)
	}
	if _, err := btdby4.CountResponsesPartJSON([]byte(`{"type":"input_text","text":"hello world"}`), btdby4.Options{}); err != nil {
		t.Errorf("CountResponsesPartJSON: %v", err)
	}
	if _, err := btdby4.CountResponsesTool(btdby4.ResponsesTool{Type: "function", Name: "get_weather"}); err != nil {
		t.Errorf("CountResponsesTool: %v", err)
	}
	if _, err := btdby4.CountResponsesToolJSON([]byte(`{"type":"function","name":"get_weather"}`)); err != nil {
		t.Errorf("CountResponsesToolJSON: %v", err)
	}
	if _, err := btdby4.CountResponsesRequestJSON([]byte(`{"instructions":"hi"}`), btdby4.Options{}); err != nil {
		t.Errorf("CountResponsesRequestJSON: %v", err)
	}
}

func TestConservativeIsUpperBound(t *testing.T) {
	chat := btdby4.ChatRequest{
		Messages: []btdby4.ChatMessage{{Role: "user", Content: "hello world"}},
		Tools: []btdby4.ChatTool{
			{Type: "function", Function: btdby4.ChatFunctionDef{Name: "get_weather", Description: "Get weather"}},
		},
	}
	tight, err := btdby4.CountChatRequest(chat, btdby4.Options{Tight: true})
	if err != nil {
		t.Fatalf("tight: %v", err)
	}
	safe, err := btdby4.CountChatRequest(chat, btdby4.Options{})
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	// Default carries the margin: 1 message * 12 + tools fixed 500 +
	// 1 tool * 45 = 557.
	if want := tight.Total + 557; safe.Total != want {
		t.Errorf("default total = %d, want %d (tight %d)", safe.Total, want, tight.Total)
	}
	if safe.TextTokens != tight.TextTokens {
		t.Errorf("margin must not change TextTokens: %d vs %d", safe.TextTokens, tight.TextTokens)
	}
	if safe.Total <= tight.Total {
		t.Errorf("default total %d should exceed tight %d", safe.Total, tight.Total)
	}

	// No tools, no images: margin is just per-message.
	plain := btdby4.ChatRequest{
		Messages: []btdby4.ChatMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
	}
	pb, _ := btdby4.CountChatRequest(plain, btdby4.Options{Tight: true})
	ps, _ := btdby4.CountChatRequest(plain, btdby4.Options{})
	if want := pb.Total + 2*12; ps.Total != want {
		t.Errorf("plain default = %d, want %d", ps.Total, want)
	}

	// Anthropic and Responses apply the same margin shape.
	areq := btdby4.AnthropicRequest{
		Messages: []btdby4.AnthropicMessage{{Role: "user", Content: "hi"}},
	}
	ab, _ := btdby4.CountAnthropicRequest(areq, btdby4.Options{Tight: true})
	as, _ := btdby4.CountAnthropicRequest(areq, btdby4.Options{})
	if want := ab.Total + 12; as.Total != want {
		t.Errorf("anthropic default = %d, want %d", as.Total, want)
	}
	rreq := btdby4.ResponsesRequest{Instructions: "hi", Input: "hello"}
	rb, _ := btdby4.CountResponsesRequest(rreq, btdby4.Options{Tight: true})
	rs, _ := btdby4.CountResponsesRequest(rreq, btdby4.Options{})
	if want := rb.Total + 12; rs.Total != want {
		t.Errorf("responses default = %d, want %d", rs.Total, want)
	}
}
