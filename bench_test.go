package btdby4_test

import (
	"strings"
	"testing"

	"github.com/italoalmeida0/btdby4"
)

var benchMsg = btdby4.AnthropicRequest{
	System: "You are a helpful assistant.",
	Tools: []btdby4.AnthropicTool{
		{Name: "get_weather", Description: "Get the current weather", InputSchema: map[string]any{"type": "object"}},
	},
	Messages: []btdby4.AnthropicMessage{
		{Role: "user", Content: "weather in SP?"},
		{Role: "assistant", Content: []any{
			map[string]any{"type": "text", "text": "checking now"},
			map[string]any{"type": "tool_use", "id": "t1", "name": "get_weather", "input": map[string]any{"city": "SP"}},
		}},
		{Role: "user", Content: []any{
			map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "sunny 25C"},
		}},
	},
}

func BenchmarkCountText(b *testing.B) {
	text := strings.Repeat("We know what we are, but know not what we may be. ", 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := btdby4.CountText(text); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCountAnthropicRequest(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := btdby4.CountAnthropicRequest(benchMsg, btdby4.Options{}); err != nil {
			b.Fatal(err)
		}
	}
}
