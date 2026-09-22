// Package btdby4 is a pure-Go estimator for whole LLM request payloads.
// It scores system, tools, text, images, visible and encrypted thinking,
// and tool I/O generically — ~90% accuracy against any provider or model,
// with nothing to configure beyond Options{Tight, IgnoreImages}. Counting
// is all you need for context windows, cost estimation, budget enforcement
// and automatic compaction — and estimating is much cheaper than encoding.
// JSON parsing uses goccy/go-json (pure Go, no CGO), which also builds
// for js/wasm.
//
// Supported request shapes (each protocol has the same granularities:
// request, request-JSON, message/item, message/item-JSON, part/block,
// part/block-JSON, tool, tool-JSON):
//
//	Anthropic Messages  CountAnthropicRequest / CountAnthropicMessage / CountAnthropicBlock / CountAnthropicTool (+JSON)
//	OpenAI Chat         CountChatRequest / CountChatMessage / CountChatPart / CountChatTool (+JSON)
//	OpenAI Responses    CountResponsesRequest / CountResponsesItem / CountResponsesPart / CountResponsesTool (+JSON)
//
// The *JSON request entry points decode once and then count + linearize
// the prefix blocks in a single fused walk shared with the KV cache,
// so gateway calls pay one parse instead of two.
//
// KV-cache simulation (KvLookup / KvInit / KvStatsSnapshot / KvClear): a
// high-performance prefix-cache simulator for gateways. Input is the raw
// request payload + protocol + namespace ("provider|model|api-key",
// assembled by the caller); output is cached/fresh/written token counts
// for billing cached input even when the provider does not report it.
// One prefix trie per protocol+namespace (or namespace only with
// KvSetSeparateProtocol(false)), 10-minute sliding TTL and a memory cap
// (default 400MB) with automatic LRU eviction — all tunable via KvInit.
// Token totals stay in stats for observability only.
//
// Text is counted with the real BPE merge table; images follow the
// standard 28px-tile visual-token model. Pass Options{IgnoreImages: true}
// to count images as zero. Encrypted thinking (redacted_thinking,
// encrypted_content, thought_signature — whatever the provider or model
// hides inside any protocol) is detected and estimated automatically by
// EstimateThinkingTokens; there is nothing to configure.
package btdby4
