// Package btdby4 is a pure-Go, zero-dependency estimator for whole LLM
// request payloads. It scores system, tools, text, images, visible and
// encrypted thinking, and tool I/O generically — ~90% accuracy against
// any provider or model, with nothing to configure beyond
// Options{IgnoreImages}. Counting is all you need for context windows,
// cost estimation, budget enforcement and automatic compaction — and
// estimating is much cheaper than encoding.
//
// Supported request shapes (each protocol has the same granularities:
// request, request-JSON, message/item, message/item-JSON, part/block,
// part/block-JSON, tool, tool-JSON):
//
//	Anthropic Messages  CountAnthropicRequest / CountAnthropicMessage / CountAnthropicBlock / CountAnthropicTool (+JSON)
//	OpenAI Chat         CountChatRequest / CountChatMessage / CountChatPart / CountChatTool (+JSON)
//	OpenAI Responses    CountResponsesRequest / CountResponsesItem / CountResponsesPart / CountResponsesTool (+JSON)
//
// Text is counted with the real BPE merge table; images follow the
// standard 28px-tile visual-token model. Pass Options{IgnoreImages: true}
// to count images as zero. Encrypted thinking (redacted_thinking,
// encrypted_content, thought_signature — whatever the provider or model
// hides inside any protocol) is detected and estimated automatically by
// EstimateThinkingTokens; there is nothing to configure.
package btdby4
