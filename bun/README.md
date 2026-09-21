# BTDby4 — Bun FFI Bindings

Ultra-fast, native [BTDby4](https://github.com/italoalmeida0/btdby4) token estimation bindings for **Bun** via `bun:ffi`.

Score plain text, visual tokens (images), whole-request payloads, and sub-block granularities (OpenAI Chat, Anthropic Messages, and OpenAI Responses) with per-turn / per-tool attribution, visible/encrypted thinking estimation, running natively at **~1.2µs per call (>770,000 ops/second on Linux)** with **zero external dependencies**.

---

## 📦 Pre-compiled Native Binaries (8 Variants)

This folder includes pre-compiled native dynamic libraries in [`lib/`](./lib) for Windows, macOS, and Linux (with dedicated builds for both **glibc** and **musl/Alpine**):

| Operating System | Architecture | Libc / Environment | Shared Library File |
|---|---|---|---|
| **Windows** | `x64` (amd64) | MSVC / MinGW | [`lib/libbtdby4-windows-amd64.dll`](./lib/libbtdby4-windows-amd64.dll) |
| **Windows** | `arm64` | MSVC / Clang | [`lib/libbtdby4-windows-arm64.dll`](./lib/libbtdby4-windows-arm64.dll) |
| **Linux** | `x64` (amd64) | **glibc** (Ubuntu, Debian, Fedora, Arch) | [`lib/libbtdby4-linux-amd64.so`](./lib/libbtdby4-linux-amd64.so) |
| **Linux** | `arm64` | **glibc** (Ubuntu, Debian, Fedora, Arch) | [`lib/libbtdby4-linux-arm64.so`](./lib/libbtdby4-linux-arm64.so) |
| **Linux** | `x64` (amd64) | **musl** (Alpine Linux, minimal containers) | [`lib/libbtdby4-linux-amd64-musl.so`](./lib/libbtdby4-linux-amd64-musl.so) |
| **Linux** | `arm64` | **musl** (Alpine Linux, minimal containers) | [`lib/libbtdby4-linux-arm64-musl.so`](./lib/libbtdby4-linux-arm64-musl.so) |
| **macOS** | `Apple Silicon` (arm64) | Mach-O / Darwin | [`lib/libbtdby4-darwin-arm64.dylib`](./lib/libbtdby4-darwin-arm64.dylib) |
| **macOS** | `Intel` (x64) | Mach-O / Darwin | [`lib/libbtdby4-darwin-amd64.dylib`](./lib/libbtdby4-darwin-amd64.dylib) |

The loader ([`index.ts`](./index.ts)) automatically detects the current operating system (`process.platform`), processor architecture (`process.arch`), and whether the environment uses **musl** (e.g., Alpine Linux) or **glibc**, loading the exact binary at runtime with zero manual configuration.

---

## 🚀 Quickstart

### Basic Usage

```typescript
import btdby4 from "./bun/index.ts";

const estimator = btdby4();

// Fast text token counting
const count = estimator.countText("Hello, world!");
console.log(`Tokens: ${count}`); // 2
```

To force a specific variant (such as musl inside Docker) or supply a custom library path:

```typescript
import { loadBTDby4 } from "./bun/index.ts";

// Force musl build:
const estimator = loadBTDby4({ musl: true });

// Or specify an explicit library path:
const estimatorCustom = loadBTDby4("/path/to/libbtdby4.so");
```

---

## 📚 Complete API Reference (All 18 Functions)

### 1. Plain Text & Reasoning

#### `countText(text: string): number`
Counts tokens in an arbitrary string using the real BPE merge table.
```typescript
const tokens = estimator.countText("Hello world!"); // 2
```

#### `estimateThinkingTokens(encryptedPayload: string): number`
Estimates billed thinking tokens hidden inside opaque encrypted envelopes or thought signatures (`redacted_thinking`, `encrypted_content`, `thought_signature`).
```typescript
const thinking = estimator.estimateThinkingTokens("gAAAAABl-...");
```

---

### 2. Image Helpers

#### `countImageSize(width: number, height: number): number`
Calculates visual tokens based on pixel dimensions using the standard 28px-tile visual token model (1568px fit).
```typescript
const tokens = estimator.countImageSize(1024, 768); // 1036
```

#### `countImageBase64(base64Str: string): number`
Decodes an image header from a base64 string (or data-URL) and estimates its visual tokens.
```typescript
const tokens = estimator.countImageBase64("data:image/png;base64,...");
```

#### `countImageBytes(bytes: Uint8Array | ArrayBuffer): number`
Inspects raw image bytes directly and calculates visual tokens.
```typescript
const tokens = estimator.countImageBytes(buffer);
```

---

### 3. OpenAI Chat Completions

#### `countChatRequest(request: object, options?: Options): Breakdown`
Scores an entire Chat Completions request (`system`, `messages`, `tools`).
```typescript
const breakdown = estimator.countChatRequest({
  system: "You are a helpful assistant.",
  messages: [{ role: "user", content: "Hello!" }],
  tools: [...]
}, { tight: true });

console.log(breakdown.total);
console.log(breakdown.by_message); // [tokens_msg_0, ...]
console.log(breakdown.by_tool);    // [tokens_tool_0, ...]
```

#### `countChatTotal(request: object, options?: Options): number`
**Fast-path**: Returns the total token count directly as an integer without JSON serialization overhead.
```typescript
const total = estimator.countChatTotal(chatPayload, { tight: true });
```

#### `countChatMessage(message: object, options?: Options): BlockBreakdown`
Scores a single individual chat message (`role`, `content`, `tool_calls`).
```typescript
const msgBreakdown = estimator.countChatMessage({
  role: "user",
  content: "Analyze this code snippet."
});
console.log(msgBreakdown.tokens, msgBreakdown.images);
```

#### `countChatPart(part: object, options?: Options): BlockBreakdown`
Scores an individual content part (`text`, `image_url`, `input_audio`).
```typescript
const part = estimator.countChatPart({ type: "text", text: "code snippet" });
```

#### `countChatTool(tool: object): number`
Scores an OpenAI tool definition (`function`, `name`, `description`, `parameters`).
```typescript
const toolTokens = estimator.countChatTool({
  type: "function",
  function: { name: "get_weather", parameters: { type: "object" } }
});
```

---

### 4. Anthropic Messages

#### `countAnthropicRequest(request: object, options?: Options): Breakdown`
Scores a full Anthropic Messages request (`system`, `messages`, `tools`, images, tool use/results, thinking).
```typescript
const breakdown = estimator.countAnthropicRequest({
  system: "You are Claude.",
  messages: [{ role: "user", content: "Hi!" }]
});
```

#### `countAnthropicTotal(request: object, options?: Options): number`
**Fast-path**: Returns the total token count directly as an integer for Anthropic requests.
```typescript
const total = estimator.countAnthropicTotal(anthropicPayload);
```

#### `countAnthropicMessage(message: object, options?: Options): BlockBreakdown`
Scores an individual Anthropic message.
```typescript
const msg = estimator.countAnthropicMessage({ role: "user", content: "Hello" });
```

#### `countAnthropicBlock(block: object, options?: Options): BlockBreakdown`
Scores a content block (`text`, `image`, `tool_use`, `tool_result`, `thinking`).
```typescript
const block = estimator.countAnthropicBlock({ type: "text", text: "Hello" });
```

#### `countAnthropicTool(tool: object): number`
Scores an Anthropic tool definition (`name`, `description`, `input_schema`).
```typescript
const toolTokens = estimator.countAnthropicTool({
  name: "calc",
  input_schema: { type: "object" }
});
```

*(Legacy aliases available: `countRequest`, `countMessage`, `countBlock`, `countTool`).*

---

### 5. OpenAI Responses API

#### `countResponsesRequest(request: object, options?: Options): Breakdown`
Scores a complete request for the OpenAI Responses API (`instructions`, `input`, `tools`, `reasoning`).
```typescript
const breakdown = estimator.countResponsesRequest({
  instructions: "Respond concisely.",
  input: [{ type: "message", role: "user", content: [{ type: "input_text", text: "Query" }] }]
});
```

#### `countResponsesTotal(request: object, options?: Options): number`
**Fast-path**: Returns the total token count directly as an integer for Responses requests.
```typescript
const total = estimator.countResponsesTotal(responsesPayload);
```

#### `countResponsesItem(item: object, options?: Options): BlockBreakdown`
Scores an individual input item (`message`, `function_call`, `function_call_output`, `reasoning`).
```typescript
const item = estimator.countResponsesItem(responsesPayload.input[0]);
```

#### `countResponsesPart(part: object, options?: Options): BlockBreakdown`
Scores an individual part of an item (`input_text`, `output_text`, `input_image`).
```typescript
const part = estimator.countResponsesPart({ type: "input_text", text: "hello" });
```

#### `countResponsesTool(tool: object): number`
Scores a Responses API tool definition.
```typescript
const toolTokens = estimator.countResponsesTool({
  type: "function",
  name: "execute_sql"
});
```

---

## 📊 Data Structures & TypeScript Interfaces

### `Breakdown`
Returned by `count*Request` methods:
```typescript
export interface Breakdown {
  system: number;        // Tokens in system instructions
  messages: number;      // Sum of tokens across all messages/items
  tools: number;         // Sum of tokens across all registered tools
  images: number;        // Tokens consumed by images
  by_message?: number[]; // Per-message token counts
  by_tool?: number[];    // Per-tool token counts
  text_tokens: number;   // Pure text content tokens (excluding images)
  total: number;         // Grand total (includes safety margin unless tight: true)
  image_count: number;   // Number of images in payload
}
```

### `BlockBreakdown`
Returned by `count*Message`, `count*Block`, `count*Item`, and `count*Part`:
```typescript
export interface BlockBreakdown {
  tokens: number;      // Total tokens in block/message
  images: number;      // Image tokens in block
  text_tokens: number; // Text tokens in block
  image_count: number; // Number of images in block
}
```

### `Options`
```typescript
export interface Options {
  tight?: boolean;        // If true, removes safety margin and returns exact point estimate
  ignoreImages?: boolean; // If true, counts images as zero tokens
}
```

---

## ⚡ Tests & Benchmark

### 1. Functional Suite & Micro-Benchmark

Run the complete 18-function verification test and micro-benchmark:

```bash
cd bun
bun test.ts
```

- **Windows (x64)**: ~190,000 ops/second (~5.2µs per call).
- **Linux (WSL2 ARM64)**: **~875,000 ops/second (~1.14µs per call)**.

### 2. Real-World ~700K Conversation Session Benchmark (vs `tokenx`)

In real-world LLM agent workloads, context windows frequently exceed hundreds of thousands of tokens. Below is a real performance comparison between native `btdby4` (via `bun:ffi`) and the pure JavaScript library [`tokenx`](https://www.npmjs.com/package/tokenx) executed on a genuine 8.3 MB conversation session containing **~680K – 700K tokens across 2,626 messages** (including multi-step tool calls, code inspections, and git diffs):

#### Benchmark Results (Windows x64 / Linux ARM64):

| Benchmark Scenario | Payload / Size | `tokenx` (JS) | `btdby4` (Bun FFI) | Speedup Multiplier |
|---|---|---|---|---|
| **Structured Chat Request** | 2,626 messages (~650k tokens) | ~220 ms (11.9k msg/s) | **~59 ms (44.4k msg/s)** | **~2.9x – 4.4x faster** |
| **Pure Conversation Text** | 1.75 MB pure text (~613k tokens) | ~182 ms (9.6 MB/s) | **~29 ms (59.8 MB/s)** | **~5.8x – 6.2x faster** |
| **Full Session JSONL File** | 8.3 MB raw file (~2.98M tokens) | ~825 ms (9.6 MB/s) | **~144 ms (54.8 MB/s)** | **~5.7x – 6.0x faster** |

*Note: Internal session debug metadata (e.g. `details`, `summary`, timestamps) is stripped before API estimation, yielding ~614k - 646k tokens, accurately mirroring the session's recorded 679,019 tokens.*

---

## 🛠️ Rebuilding Native Binaries

To rebuild all 8 target binaries in a single pass:

```powershell
cd bun
.\build.ps1
```

---

## 🔒 Memory Safety & Zero Leaks

- **Input (JS ➔ C)**: Strings are passed as UTF-8 null-terminated buffers (`Buffer.from(str + "\0", "utf8")`).
- **Output (C ➔ JS)**: For functions returning dynamic JSON strings, Bun clones the C string with `new CString(ptr)` and immediately invokes the exported `FreeCString(ptr)` to release the Go C-heap allocation, guaranteeing zero memory leaks even under millions of continuous invocations.
