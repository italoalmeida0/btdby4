# BTDby4 — Better Than Divide-by-4

One generic estimator for any LLM payload. BTDby4 scores a **complete
request** — system, tools, text, images, visible thinking, encrypted
thinking, tool I/O — with ~90% accuracy against **any provider or model**,
in under a millisecond. One pure-Go JSON dependency (goccy/go-json,
no CGO — it also builds for js/wasm). No provider config, no
model tables, no per-API math: drop in any payload shape and get a
breakdown you can enforce budgets, compaction and routing on.

```go
import "github.com/italoalmeida0/btdby4"

n, _ := btdby4.CountText("hello world") // 2 — exact, not "len/4"
```

## How it compares

Three kinds of counters exist: raw BPE encoders (exact text, nothing
else),
and the `/4` guess. BTDby4 is the fourth option — a local,
whole-payload estimator:

| Capability | BTDby4 | tiktoken family (Python / Rust / Go) | JS counters (js-tiktoken / gpt-tokenizer) |
|---|---|---|---|
| Text | ✅ identical counts | ✅ reference | ⚠️ gpt-tokenizer drifts ~1.3% |
| Whole request in one call | ✅ + per-turn `Breakdown` | ❌ manual recipe | ⚠️ chat only (gpt-tokenizer) |
| Protocols | ✅ Anthropic + Chat + Responses | ❌ OpenAI text | ❌ OpenAI text |
| Images | ✅ 28px-tile model | ❌ | ❌ |
| Encrypted thinking | ✅ auto-estimated | ❌ invisible | ❌ invisible |
| Offline, pure Go | ✅ | ⚠️ downloads BPE on first use | ✅ |

### Large-context latency (windows/arm64, p50)

Same realistic payloads per size (EN + code + JSON tool args + SQL +
multilingual, ~4 chars/token). Counts identical across btdby4 /
tiktoken / tiktoken-rs / js-tiktoken (gpt-tokenizer drifts, own table).

| Context | btdby4 | tiktoken (Python) | tiktoken-rs (Rust) | js-tiktoken | gpt-tokenizer | Faster than best rival |
|---|---|---|---|---|---|---|
| 256K (~1.0MB) | **1ms** | 59ms | 70ms | 217ms | 25ms | **~25×** |
| 512K (~2.1MB) | **3ms** | 117ms | 137ms | 1042ms | 128ms | **~39×** |
| 1M (~4.1MB) | **5ms** | 217ms | 799ms | 2354ms | 407ms | **~43×** |
| 1.5M (~6.1MB) | **9ms** | 398ms | 1342ms | 3738ms | 478ms | **~44×** |

Tails stay flat for btdby4 while rivals fan out at 1.5M (p99): btdby4
10ms vs tiktoken 888ms, tiktoken-rs 1435ms, gpt-tokenizer 650ms,
js-tiktoken 3928ms. Full p50/p90/p99 runs: 11 timed iterations each
(7 on Node), same machine.

Whole-request cost (80 tool-use/tool-result turns, 7295 tokens):
`CountAnthropicRequest` p50 **<1µs** (cached), p99 1.05ms — per-turn
attribution for compaction is effectively free.

```sh
go test -run=NONE -bench=. -benchtime=1000000x -count=1 .
```

Why faster: single-pass BPE scan over a pre-merged rank table with a
sharded count cache — no regex split per call, no encode allocation,
no BPE download. tiktoken variants pay the full regex + merge loop
every call; js-tiktoken adds JS/WASM overhead on top.

## Why you'll use it

**The `/4` rule is silently eating your context.** Code tokenizes near
1 char/token, JSON tool payloads blow up, non-English text drifts — teams
either over-truncate (wasting context they paid for) or overflow mid-request
(crashing long agent runs). BTDby4 scores the whole payload generically
(real BPE for text and tool JSON, 28px-tile model for images, calibrated
envelope math for encrypted thinking), so a generic AI harness can pack,
meter and compact any request to the edge safely:

- **Context-window packing** — fit one more tool result instead of leaving
  20% headroom "just in case"
- **Cost metering** — bill per estimated token, not per guess
- **Budget enforcement** — kill runaway agent loops before the invoice does
- **Automatic compaction** — trigger summarization off `Total` / `ByMessage`
  instead of provider-specific counters
- **Pre-flight checks** — reject or compress oversized requests *before*
  paying for the API call

## One call per API

```go
// Anthropic Messages
out, _ := btdby4.CountAnthropicRequest(btdby4.AnthropicRequest{
    System: "You are a helpful assistant.",
    Messages: []btdby4.AnthropicMessage{
        {Role: "user", Content: "hello world"},
        {Role: "assistant", Content: []any{
            map[string]any{"type": "text", "text": "hi there"},
            map[string]any{"type": "tool_use", "id": "toolu_1",
                "name": "get_weather", "input": map[string]any{"city": "SP"}},
        }},
    },
    Tools: []btdby4.AnthropicTool{
        {Name: "get_weather", Description: "Get the current weather",
            InputSchema: map[string]any{"type": "object"}},
    },
}, btdby4.Options{} /* IgnoreImages */)

fmt.Println(out.Total, out.TextTokens, out.Images, out.ByMessage, out.ByTool)

// OpenAI Chat Completions
cout, _ := btdby4.CountChatRequest(btdby4.ChatRequest{
    Messages: []btdby4.ChatMessage{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "hello world"},
    },
}, btdby4.Options{})

// OpenAI Responses — reasoning + encrypted thinking auto-estimated
rout, _ := btdby4.CountResponsesRequest(btdby4.ResponsesRequest{
    Instructions: "You are a helpful assistant.",
    Input: []any{
        map[string]any{"type": "message", "role": "user", "content": []any{
            map[string]any{"type": "input_text", "text": "hello world"},
        }},
    },
}, btdby4.Options{})
```

Every counter returns a `Breakdown`: per-message and per-tool counts plus
separate `TextTokens`/`Images`/`Total`, so you can attribute spend per turn.
Each protocol exposes the same granularities (plus `*JSON` variants that
take raw bytes):

| Level   | Anthropic Messages | OpenAI Chat       | OpenAI Responses    |
|---------|--------------------|-------------------|---------------------|
| Request | `CountAnthropicRequest` | `CountChatRequest`| `CountResponsesRequest` |
| Message | `CountAnthropicMessage` | `CountChatMessage`| `CountResponsesItem`    |
| Part    | `CountAnthropicBlock`   | `CountChatPart`   | `CountResponsesPart`    |
| Tool    | `CountAnthropicTool`    | `CountChatTool`   | `CountResponsesTool`    |

Supported content: Anthropic `text` / `thinking` (visible) / `image`
(base64, URL, data-URL) / `tool_use` / `tool_result` / `redacted_thinking`;
Chat `text` / `reasoning_content` (visible thinking) / `image_url` /
`input_audio` (+ `tool_calls`); Responses `message` (`input_text`,
`output_text`, `reasoning_text`, `input_image`), `function_call`,
`function_call_output`, `reasoning` (summary + `encrypted_content`). Images use the standard 28px-tile
visual-token model (1568px fit). Encrypted thinking (`redacted_thinking`,
`encrypted_content`, `thought_signature` — any provider, any model, any
protocol) is detected and estimated automatically; there is nothing to
configure.

## API Reference

All counters take an `Options` value (`Tight`, `IgnoreImages`) and
return `(Breakdown, error)` for requests/messages/parts or `(int, error)`
for text/tools. `*JSON` variants accept raw `[]byte` instead of structs.
Images-only helpers never fail and return a plain `int`.

### Text

| Function | Signature | Description |
|---|---|---|
| `CountText` | `CountText(text string) (int, error)` | Count tokens in a plain string. |

```go
n, _ := btdby4.CountText("hello world") // 2
```

### Anthropic Messages

Types: `AnthropicRequest{System any, Messages []AnthropicMessage, Tools []AnthropicTool}`,
`AnthropicMessage{Role string, Content any}`,
`AnthropicTool{Name, Description string, InputSchema any}`.
`Content` accepts a plain string or `[]any` of content blocks (`text`,
`image`, `tool_use`, `tool_result`, `redacted_thinking`).

| Function | Signature | Description |
|---|---|---|
| `CountAnthropicRequest` | `CountAnthropicRequest(req AnthropicRequest, opts Options) (Breakdown, error)` | Full request: system + messages + tools. |
| `CountAnthropicRequestJSON` | `CountAnthropicRequestJSON(raw []byte, opts Options) (Breakdown, error)` | Same, from raw JSON. |
| `CountAnthropicMessage` | `CountAnthropicMessage(msg AnthropicMessage, opts Options) (BlockBreakdown, error)` | Single message (string or blocks). |
| `CountAnthropicMessageJSON` | `CountAnthropicMessageJSON(raw []byte, opts Options) (BlockBreakdown, error)` | Same, from raw JSON. |
| `CountAnthropicBlock` | `CountAnthropicBlock(block map[string]any, opts Options) (BlockBreakdown, error)` | Single content block. |
| `CountAnthropicBlockJSON` | `CountAnthropicBlockJSON(raw []byte, opts Options) (BlockBreakdown, error)` | Same, from raw JSON. |
| `CountAnthropicTool` | `CountAnthropicTool(tool AnthropicTool) (int, error)` | Tool definition (name + description + schema). |
| `CountAnthropicToolJSON` | `CountAnthropicToolJSON(raw []byte) (int, error)` | Same, from raw JSON. |

```go
mb, _ := btdby4.CountAnthropicMessage(
    btdby4.AnthropicMessage{Role: "user", Content: "hello world"},
    btdby4.Options{},
)
fmt.Println(mb.Tokens) // 2
```

### OpenAI Chat Completions

Types: `ChatRequest{System any, Messages []ChatMessage, Tools []ChatTool}`,
`ChatMessage{Role string, Content any, ToolCalls []ChatToolCall, ToolCallID, Name string}`,
`ChatToolCall{ID, Type string, Function ChatFunction}`,
`ChatFunction{Name, Arguments string}`,
`ChatTool{Type string, Function ChatFunctionDef}`,
`ChatFunctionDef{Name, Description string, Parameters any, Strict bool}`.
`Content` accepts a plain string or `[]any` of parts (`text`, `image_url`,
`input_audio`).

| Function | Signature | Description |
|---|---|---|
| `CountChatRequest` | `CountChatRequest(req ChatRequest, opts Options) (Breakdown, error)` | Full request: system + messages + tools. |
| `CountChatRequestJSON` | `CountChatRequestJSON(raw []byte, opts Options) (Breakdown, error)` | Same, from raw JSON. |
| `CountChatMessage` | `CountChatMessage(msg ChatMessage, opts Options) (BlockBreakdown, error)` | Single message (string, parts, or tool calls). |
| `CountChatMessageJSON` | `CountChatMessageJSON(raw []byte, opts Options) (BlockBreakdown, error)` | Same, from raw JSON. |
| `CountChatPart` | `CountChatPart(part map[string]any, opts Options) (BlockBreakdown, error)` | Single content part. |
| `CountChatPartJSON` | `CountChatPartJSON(raw []byte, opts Options) (BlockBreakdown, error)` | Same, from raw JSON. |
| `CountChatTool` | `CountChatTool(tool ChatTool) (int, error)` | Tool definition (name + description + parameters). |
| `CountChatToolJSON` | `CountChatToolJSON(raw []byte) (int, error)` | Same, from raw JSON. |

```go
pb, _ := btdby4.CountChatPart(
    map[string]any{"type": "text", "text": "hello world"},
    btdby4.Options{},
)
fmt.Println(pb.Tokens) // 2
```

### OpenAI Responses

Types: `ResponsesRequest{Instructions any, Input any, Tools []ResponsesTool}`,
`ResponsesTool{Type, Name, Description string, Parameters any, Strict bool}`.
`Input` accepts a plain value or `[]any` of items (`message`,
`function_call`, `function_call_output`, `reasoning`). Item content parts:
`input_text`, `output_text`, `text`, `input_image`, `summary_text`.
Multi-item `reasoning` shares one envelope overhead per request (the
fitted per-call model); unit-level `CountResponsesItem` estimates each
envelope in isolation.

| Function | Signature | Description |
|---|---|---|
| `CountResponsesRequest` | `CountResponsesRequest(req ResponsesRequest, opts Options) (Breakdown, error)` | Full request: instructions + input items + tools. |
| `CountResponsesRequestJSON` | `CountResponsesRequestJSON(raw []byte, opts Options) (Breakdown, error)` | Same, from raw JSON. |
| `CountResponsesItem` | `CountResponsesItem(item map[string]any, opts Options) (BlockBreakdown, error)` | Single input item. |
| `CountResponsesItemJSON` | `CountResponsesItemJSON(raw []byte, opts Options) (BlockBreakdown, error)` | Same, from raw JSON. |
| `CountResponsesPart` | `CountResponsesPart(part map[string]any, opts Options) (BlockBreakdown, error)` | Single content part. |
| `CountResponsesPartJSON` | `CountResponsesPartJSON(raw []byte, opts Options) (BlockBreakdown, error)` | Same, from raw JSON. |
| `CountResponsesTool` | `CountResponsesTool(tool ResponsesTool) (int, error)` | Tool definition (non-function tools counted as JSON). |
| `CountResponsesToolJSON` | `CountResponsesToolJSON(raw []byte) (int, error)` | Same, from raw JSON. |

```go
ib, _ := btdby4.CountResponsesItem(map[string]any{
    "type": "function_call", "name": "get_weather",
    "arguments": `{"city":"SP"}`,
}, btdby4.Options{})
fmt.Println(ib.Tokens) // name + arguments
```

### Images

| Function | Signature | Description |
|---|---|---|
| `CountImageSize` | `CountImageSize(width, height int) int` | Visual tokens for W×H (1568px fit, 28px tiles). |
| `CountImageBytes` | `CountImageBytes(b []byte) int` | Decode (PNG/JPEG/GIF) then count; `0` if undecodable. |
| `CountImageBase64` | `CountImageBase64(s string) int` | Strip optional `data:` prefix, base64-decode, then count; `0` on error. |

```go
btdby4.CountImageSize(1024, 768) // tiles after fit
```

### Result types

`Breakdown` (requests) — `System`, `Messages`, `Tools`, `Images`
(image-token subtotal), `ByMessage []int`, `ByTool []int`,
`TextTokens` (`System + Messages + Tools - Images`), `Total`
(`System + Messages + Tools`), `ImageCount` (number of image blocks).

`BlockBreakdown` (single message/item/part) — `Tokens`, `Images`,
`TextTokens` (`Tokens - Images`), `ImageCount`.

### Options

```go
type Options struct {
    IgnoreImages bool // count images as zero
    Tight        bool // opt out of the safety margin: closest point estimate
}
```

### Bias: over or under?

`Total` ships with a small generic safety margin on top of the content
count (BPE text + tile area + encrypted-thinking estimate), so against
billed `prompt_tokens` / `input_tokens` it lands **at or above** the real
total in the common cases — install-and-use safe for budget enforcement
and automatic compaction: compacting at 90% of the window fires *before*
the real context fills up instead of after. The margin is generic (no
per-provider or per-model tables): +12 per message, +500 with tools plus
+45 per tool, +1100 per image — sized to cover the worst provider
measured (tool-harness preamble ~500 fixed + ~40/tool; minimum billable
cost per small image up to ~1000), while staying negligible at long
context (a dozen tokens against 200k+). `TextTokens` always keeps the
pure content count with no margin; pass `Options{Tight: true}` to get
the closest point estimate in `Total` as well (display/cost paths).
Without the margin the estimator would sit **at or below** billed usage
by that same fixed overhead — fine for estimates, late for compaction
triggers.

Encrypted thinking (`redacted_thinking` data,
Responses `encrypted_content`, Google `thought_signature` — any provider,
any model, any protocol, even a Gemini payload inside Responses or a GPT
payload inside Anthropic blocks) is opaque ciphertext, so it is estimated,
not decoded, and always on: the base64 envelope is unwrapped to raw bytes
and mapped per envelope family (Fernet-style ~4.75 raw bytes/token past a
~752-byte overhead shared per request; Google-style compact signature
~2.76 raw bytes/token; jumbo single envelopes ≥ 8000 chars at
`966 + raw/16.9`). Calibrated on 14 measured payloads across three
provider shapes (median error ~5%).

| Helper | Signature | Description |
|---|---|---|
| `EstimateThinkingTokens` | `EstimateThinkingTokens(enc string) int` | Automatic estimate for one envelope (empty → 0). |

## 🌐 WebAssembly Bindings (`btdby4-wasm`)

BTDby4 ships a universal WebAssembly package in [`wasm/`](./wasm) (`btdby4-wasm`): the same Go estimator compiled to WASM, running in Node.js, Deno and browsers with zero native dependencies.

All payload APIs take the raw JSON string (the request body as-is) — no object round-trip:

```typescript
import initBTDby4 from "btdby4-wasm";

const btdby4 = await initBTDby4();

// 1. Text token counting
console.log(btdby4.countText("Hello, world!")); // 2

// 2. Whole OpenAI Chat completions request — pass the body straight through
const total = btdby4.countChatTotal(JSON.stringify({
  model: "gpt-4o",
  messages: [
    { role: "system", content: "You are a helpful assistant." },
    { role: "user", content: "Hello!" }
  ]
}));
```

For complete documentation, see [`wasm/README.md`](./wasm/README.md).

### KV-cache simulation (`kvInit` / `kvCache` / `kvStats` / `kvClear`)

The WASM package also ships a prefix-cache simulator for gateways:
pass the raw request body + protocol + namespace
(`"provider|model|api-key"`) and get `cached` / `fresh` / `written`
token counts even when the provider does not report cached input.
One prefix trie per protocol+namespace, sliding TTL (default 600s)
and a memory cap (default 400MB) with automatic LRU — all tunable
via `kvInit`. See the KV-Cache Provider section in
[`wasm/README.md`](./wasm/README.md).

## Install

```bash
go get github.com/italoalmeida0/btdby4
```

Requires Go 1.18+. One dependency (goccy/go-json, pure Go). Builds on
windows/linux/darwin × amd64/arm64.

## How it works

- **Counter, not encoder.** BPE merges are evaluated only to learn *how many*
  tokens a piece produces, never *which* ones. Zero allocations on hot paths.
- **Hand-written splitter.** The pre-tokenizer is a direct scanner —
  no regex engine at runtime.
- **Daemon-safe caches.** Bounded sharded caches for pieces and texts; built
  for background services handling ever-changing requests for hours.
- **Parallel requests.** Large requests fan out per message across all CPUs.

## License

MIT — see [LICENSE](LICENSE).
