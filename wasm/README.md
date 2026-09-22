# btdby4-wasm ⚡

Universal WebAssembly port of **BTDby4**, the ultra-fast LLM token estimator for OpenAI and Anthropic Claude payloads.

Works out-of-the-box in **Node.js**, **Bun**, **Deno**, and **Browsers** with **zero native dependencies** and **no OS/architecture constraints**.

[![npm version](https://img.shields.io/npm/v/btdby4-wasm.svg)](https://www.npmjs.com/package/btdby4-wasm)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Highlights

- 🌐 **100% Universal**: 1 single package that runs anywhere WebAssembly is supported.
- ⚡ **11x–16x Faster than pure JS**: Delivers ~100–160 MB/s throughput on large payloads.
- 🔒 **Zero Native Builds**: No `node-gyp`, no C++ compilers, no separate platform packages needed.
- 📐 **Full Feature Parity**: Covers OpenAI Chat, Anthropic Messages, OpenAI Responses API, encrypted thinking tokens, and image token calculations.
- 📦 **Full TypeScript Support**: Shipped with comprehensive `.d.ts` type declarations.

---

## Installation

```bash
npm install btdby4-wasm
# or
bun add btdby4-wasm
# or
pnpm add btdby4-wasm
```

---

## Quick Start

### Node.js (ESM / TypeScript) & Bun

```typescript
import initBTDby4 from "btdby4-wasm";

// Initialize once (in Node.js / Bun, automatically loads btdby4.wasm)
const btdby4 = await initBTDby4();

// 1. Plain Text Counting
const count = btdby4.countText("Hello world!");
console.log(`Tokens: ${count}`); // 3

// 2. OpenAI Chat Completion
const chatBreakdown = btdby4.countChatRequest({
  system: "You are a helpful coding assistant.",
  messages: [
    { role: "user", content: "Write a high-performance token estimator." }
  ]
});
console.log(`Chat Total: ${chatBreakdown.total}`);

// 3. Anthropic Messages
const anthropicBreakdown = btdby4.countAnthropicRequest({
  system: "You are Claude.",
  messages: [
    { role: "user", content: "Count my tokens!" }
  ]
});
console.log(`Anthropic Total: ${anthropicBreakdown.total}`);

// 4. Encrypted Thinking Tokens (Claude 3.7 Sonnet)
const thinkingTokens = btdby4.estimateThinkingTokens("gAAAAABl-...");
console.log(`Estimated Thinking Tokens: ${thinkingTokens}`);
```

### Browser / Cloudflare Workers

In edge or browser environments where the local filesystem is not available, pass a URL or `fetch` response:

```typescript
import initBTDby4 from "btdby4-wasm";

// Provide the URL or fetch response to btdby4.wasm
const btdby4 = await initBTDby4(new URL("btdby4.wasm", import.meta.url));
console.log(btdby4.countText("Browser token estimation!"));
```

---

## Synchronous Usage After Initialization

If you have initialized the WASM engine at application startup:

```typescript
import { initBTDby4, getBTDby4 } from "btdby4-wasm";

// At server startup
await initBTDby4();

// Later in request handlers (synchronous!)
export function handleRequest(prompt: string) {
  const btdby4 = getBTDby4();
  return btdby4.countText(prompt);
}
```

---

## API Reference

### Text & Reasoning
- `countText(text: string): number`
- `estimateThinkingTokens(encryptedPayload: string): number`

### Image Token Helpers
- `countImageSize(width: number, height: number): number`
- `countImageBase64(base64Str: string): number`
- `countImageBytes(bytes: Uint8Array | ArrayBuffer): number`

> All payload APIs take a **raw JSON string** (the request body as-is).
> Single `JSON.stringify` at the gateway, zero double conversion.

### OpenAI Chat API
- `countChatRequest(request: string, options?: Options): Breakdown`
- `countChatTotal(request: string, options?: Options): number`
- `countChatMessage(message: string, options?: Options): BlockBreakdown`
- `countChatPart(part: string, options?: Options): BlockBreakdown`
- `countChatTool(tool: string): number`

### Anthropic Messages API
- `countAnthropicRequest(request: string, options?: Options): Breakdown`
- `countAnthropicTotal(request: string, options?: Options): number`
- `countAnthropicMessage(message: string, options?: Options): BlockBreakdown`
- `countAnthropicBlock(block: string, options?: Options): BlockBreakdown`
- `countAnthropicTool(tool: string): number`

### OpenAI Responses API
- `countResponsesRequest(request: string, options?: Options): Breakdown`
- `countResponsesTotal(request: string, options?: Options): number`
- `countResponsesItem(item: string, options?: Options): BlockBreakdown`
- `countResponsesPart(part: string, options?: Options): BlockBreakdown`
- `countResponsesTool(tool: string): number`

### KV-Cache Provider (prefix simulation)
- `kvInit(options?: { ttlSeconds?: number; maxMB?: number; separateProtocol?: boolean }): KvConfig`
- `kvCache(request: string, protocol: "anthropic" | "chat" | "responses", namespace: string, options?: Options): KvResult`
- `kvStats(): KvStats`
- `kvClear(namespace?: string): void`

```typescript
// Configure once at boot (defaults: 600s / 400MB / separateProtocol: true).
btdby4.kvInit({ ttlSeconds: 600, maxMB: 400 });

// namespace isolates like a real provider: provider|model|api-key
// (the real key is protocol + namespace, unless separateProtocol: false)
const ns = "openai|gpt-5|sk-123";

const first = btdby4.kvCache(chatPayload, "chat", ns);
// first.cached === 0, first.fresh === first.total

const second = btdby4.kvCache(chatPayload, "chat", ns);
// second.cached === second.total, second.fresh === 0

// Conversation growth: prefix hits, suffix is fresh.
// Sliding TTL (default 10min), memory cap with automatic LRU.
// Token counts stay in stats for observability only.
const stats = btdby4.kvStats();
```

---

## Performance Benchmark

Measured on Apple Silicon / ARM64:

| Benchmark | BTDby4 WASM | pure JS (`tokenx`) | Speedup |
| :--- | :--- | :--- | :--- |
| **Medium Text (~100K tokens / 400 KB)** | **156 MB/s** | ~9.6 MB/s | **16.2x faster** |
| **Large Text (~3M tokens / 8.3 MB)** | **94 MB/s** | ~8.4 MB/s | **11.2x faster** |

---

## License

MIT © [italoalmeida0](https://github.com/italoalmeida0)
