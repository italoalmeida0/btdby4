# btdby4-wasm ⚡

> **The most complete, ultra-fast LLM token estimator & KV-cache prefix simulator in the JavaScript & WebAssembly ecosystem.**

[![npm version](https://img.shields.io/npm/v/btdby4-wasm.svg?style=flat-square)](https://www.npmjs.com/package/btdby4-wasm)
[![npm downloads](https://img.shields.io/npm/dm/btdby4-wasm.svg?style=flat-square)](https://www.npmjs.com/package/btdby4-wasm)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)

Universal WebAssembly engine for **BTDby4**. Built specifically for **AI Gateways**, **LLM Proxies** (like OpenRouter, LiteLLM, Cloudflare AI Gateway, Portkey), and **Production Agent Runtimes**.

Works out-of-the-box in **Node.js**, **Bun**, **Deno**, **Cloudflare Workers**, and **Browsers** with **zero external model downloads**, **zero native compilation (`node-gyp`)**, and **zero runtime dependencies**.

---

## 🚀 Why btdby4-wasm?

Other tokenizers on NPM fall into two traps:
1. **Dumb regex/heuristics** (`tokenx`, `token-estimate`) that drift significantly on code, JSON, and non-English text, and cannot parse messages.
2. **Raw text-only BPE engines** (`bpe-lite`, `kitoken`, `hypertok`) that **only take plain strings**, require downloading 15–30 MB external files, break in non-Node runtimes (e.g. `kitoken` crashes in Bun), and have zero knowledge of Chat Completions payloads, message framing tokens, vision, or KV caching.

**btdby4-wasm is the only all-in-one engine:**
- ⚡ **10x to 27x faster** than traditional tokenizers (`kitoken`, `bpe-lite`).
- 📦 **100% Bundled & Offline:** Zero external downloads, zero network calls.
- 🎯 **Direct JSON Stream Processing:** Pass your raw request body string directly — zero double-parsing in JavaScript.
- 🧠 **Full Protocol Framing:** Exact message delimiters (`<|im_start|>`, roles, tool calls, function schemas) for OpenAI Chat, Anthropic Claude, and OpenAI Responses API.
- 🔮 **True KV-Cache Simulation:** Prefix Trie with sliding TTL and LRU eviction for real-time prompt-cache hit prediction and intelligent multi-provider routing (OpenRouter-style).
- 🖼️ **Multimodal Support:** Full 28px-tile image token counting (base64, raw bytes, image URLs).
- 🔒 **Encrypted Reasoning Tokens:** Calibrated estimation for Claude 3.7 Sonnet `redacted_thinking` and Gemini `thought_signature`.

---

## 📊 Speed Benchmark: Real LLM Payloads

Tested on actual chat completion payloads extracted from production dialogues (Node.js v22 on ARM64):

| Payload Context Size | **⚡ btdby4-wasm** | **📦 bpe-lite** | **🦀 kitoken** | **📦 tokenx** | **📦 token-estimate** |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **20K Tokens (~60 KB)** | **0.41 ms** *(137 MB/s)* | 4.09 ms *(10x slower)* | 10.20 ms *(25x slower)* | 4.20 ms *(10x slower)* | 6.81 ms *(16x slower)* |
| **50K Tokens (~150 KB)** | **1.63 ms** *(88 MB/s)* | 10.30 ms *(6.3x slower)* | 24.18 ms *(15x slower)* | 10.57 ms *(6.5x slower)* | 18.32 ms *(11x slower)* |
| **100K Tokens (~310 KB)** | **2.07 ms** *(107 MB/s)* | 28.88 ms *(14x slower)* | 53.94 ms *(26x slower)* | 35.30 ms *(17x slower)* | 51.45 ms *(25x slower)* |
| **200K Tokens (~630 KB)** | **6.32 ms** *(71 MB/s)* | 86.75 ms *(14x slower)* | 173.06 ms *(27x slower)* | 67.84 ms *(11x slower)* | 99.38 ms *(16x slower)* |
| **Full (~2.2 MB / 700K Tokens)** | **16.84 ms** *(76 MB/s)* | 175.29 ms *(10.4x slower)* | 414.84 ms *(25x slower)* | 165.08 ms *(10x slower)* | 231.02 ms *(14x slower)* |

> 🎯 **100% Token Accuracy:** `btdby4-wasm` delivers **exact token counts matching reference BPE** down to the single token, but at 10x–27x the throughput of incumbent WASM tokenizers.

---

## 🏆 Architectural Comparison

| Feature | `btdby4-wasm` | `bpe-lite` | `kitoken` | `hypertok` | `tokenx` |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Accepts Raw Chat JSON Directly** | ✅ **Yes** | ❌ (returns 0) | ❌ (string only) | ❌ (string only) | ❌ (string only) |
| **Message Framing & Delimiters** | ✅ **Yes** | ❌ No | ❌ No | ❌ No | ❌ No |
| **KV-Cache Prefix Simulation** | ✅ **Yes** | ❌ No | ❌ No | ❌ No | ❌ No |
| **Multimodal Vision / Image Tiles** | ✅ **Yes** | ❌ No | ❌ No | ❌ No | ❌ No |
| **Encrypted Thinking Estimation** | ✅ **Yes** | ❌ No | ❌ No | ❌ No | ❌ No |
| **Bundled Offline Vocabularies** | ✅ **Yes** | ✅ Yes | ❌ Requires 15-30MB file | ❌ Requires external package | ⚠️ Heuristic regex |
| **Bun & Edge Runtime Support** | ✅ **100%** | ✅ Yes | ❌ Crashes in Bun | ⚠️ Complex setup | ✅ Yes |

---

## 📦 Installation

```bash
npm install btdby4-wasm
# or
bun add btdby4-wasm
# or
pnpm add btdby4-wasm
```

---

## 🚦 Quick Start

### 1. Initialize Once

```typescript
import initBTDby4, { getBTDby4 } from "btdby4-wasm";

// In Node.js / Bun, initializes automatically:
const btdby4 = await initBTDby4();

// In Cloudflare Workers or Browsers, pass the wasm binary URL:
// const btdby4 = await initBTDby4(new URL("btdby4.wasm", import.meta.url));
```

### 2. Count Whole OpenAI Chat Completions Request

Pass the request JSON string directly from your incoming HTTP request body — no double unmarshaling:

```typescript
const rawRequestBody = JSON.stringify({
  model: "gpt-4o",
  messages: [
    { role: "system", content: "You are a helpful coding assistant." },
    { role: "user", content: "Write a high-performance router in Go." }
  ],
  tools: [
    {
      type: "function",
      function: {
        name: "run_code",
        description: "Executes code in a sandbox",
        parameters: { type: "object", properties: { code: { type: "string" } } }
      }
    }
  ]
});

// Full breakdown in microseconds:
const breakdown = btdby4.countChatRequest(rawRequestBody);
console.log(breakdown);
/* Output:
{
  total: 68,
  textTokens: 68,
  system: 11,
  messages: 23,
  tools: 34,
  images: 0,
  byMessage: [ 11, 12 ],
  byTool: [ 34 ]
}
*/

// Or just the total:
const totalTokens = btdby4.countChatTotal(rawRequestBody);
```

### 3. Anthropic Claude Messages API

```typescript
const anthropicPayload = JSON.stringify({
  model: "claude-3-7-sonnet-20250219",
  system: "You are Claude.",
  messages: [
    { role: "user", content: "Analyze this image." },
    { 
      role: "assistant", 
      content: [
        { type: "thinking", thinking: "Let's inspect the pixels." },
        { type: "text", text: "Here is what I found." }
      ]
    }
  ]
});

const result = btdby4.countAnthropicRequest(anthropicPayload);
console.log(`Anthropic Total: ${result.total}`);
```

---

## 🧠 True KV-Cache Prefix Simulation (For Gateways & Multi-Provider Routing)

Modern LLM inference engines (vLLM, SGLang, Anthropic Prompt Caching, OpenAI Prompt Caching, DeepSeek) reuse Key-Value (KV) attention states for common prompt prefixes.

**`btdby4-wasm` implements a production-grade prefix trie simulator** with configurable sliding TTL and memory limits (LRU eviction).

### Why this is a game-changer for AI Gateways (OpenRouter / LiteLLM / Portkey):
1. **Pre-Flight Cost Prediction:** Know *before sending the request* how many tokens will hit the provider's cache discount (e.g., Anthropic's 90% cache discount or OpenAI's 50% discount).
2. **Smart Multi-Provider Routing:** Route requests to the specific upstream cluster or model replica where the conversation prefix is already cached, slashing latency by up to 80%.
3. **Session State Tracking:** Keep track of cache freshness per user, tenant, or API key.

```typescript
// 1. Configure the simulator at gateway boot
btdby4.kvInit({
  ttlSeconds: 600,       // 10 minutes sliding expiration (matches Anthropic / vLLM)
  maxMB: 400,            // 400 MB RAM memory cap with automatic LRU eviction
  separateProtocol: true // Isolate namespaces by protocol
});

// 2. Namespace your cache like real providers (provider|model|tenant)
const namespace = "anthropic|claude-3-7-sonnet|customer-tenant-42";

// First message in a conversation:
const req1 = JSON.stringify({
  system: "You are an enterprise support bot with 500 lines of guidelines...",
  messages: [{ role: "user", content: "Hello!" }]
});

const sim1 = btdby4.kvCache(req1, "anthropic", namespace);
console.log(sim1);
// -> { cached: 0, fresh: 1250, written: 1250, hit: false, total: 1250 }

// Second message in the conversation (conversation growth):
const req2 = JSON.stringify({
  system: "You are an enterprise support bot with 500 lines of guidelines...",
  messages: [
    { role: "user", content: "Hello!" },
    { role: "assistant", content: "How can I help you?" },
    { role: "user", content: "What is my invoice balance?" }
  ]
});

const sim2 = btdby4.kvCache(req2, "anthropic", namespace);
console.log(sim2);
// -> { cached: 1250, fresh: 38, written: 38, hit: true, total: 1288 }
// 🔥 1,250 tokens hit the cache! You save 90% on input costs!

// 3. Inspect global cache telemetry & dashboard capacity (also cleans up expired entries automatically!):
const stats = btdby4.kvStats();
console.log(`Active Nodes: ${stats.nodes}, Memory Used: ${(stats.bytes / 1024 / 1024).toFixed(2)} MB, Available: ${(stats.available_bytes / 1024 / 1024).toFixed(2)} MB, TTL: ${stats.ttl_seconds}s`);
```

---

## 🖼️ Image Tokens & Multimodal Calculations

Handles images following provider visual tile specs (1568px fit, 28px tiles):

```typescript
// From dimensions:
const tokensFromDimensions = btdby4.countImageSize(1920, 1080); // 1,600 tiles

// From Base64 data:
const tokensFromBase64 = btdby4.countImageBase64("iVBORw0KGgoAAAANSUhEUgAA...");

// From Uint8Array buffer:
const tokensFromBuffer = btdby4.countImageBytes(imageBytes);
```

---

## 🔒 Encrypted Reasoning & Thinking Tokens

Estimates opaque reasoning envelopes (Claude 3.7 Sonnet `redacted_thinking`, Gemini `thought_signature`) using calibrated envelope metrics:

```typescript
const thinkingTokens = btdby4.estimateThinkingTokens("gAAAAABl-...");
```

---

## 📚 Complete API Reference

| Category | Function | Signature | Description |
| :--- | :--- | :--- | :--- |
| **Lifecycle** | `initBTDby4` | `(wasmSource?: string \| URL \| Response \| Buffer): Promise<BTDby4Instance>` | Initializes WASM runtime |
| | `getBTDby4` | `(): BTDby4Instance` | Synchronous getter after boot |
| **Text** | `countText` | `(text: string): number` | Raw BPE token counter |
| **Images** | `countImageSize` | `(width: number, height: number): number` | Visual tiles from dimensions |
| | `countImageBase64` | `(base64: string): number` | Visual tiles from base64 |
| | `countImageBytes` | `(bytes: Uint8Array): number` | Visual tiles from image buffer |
| **OpenAI Chat** | `countChatRequest` | `(rawJson: string, options?: Options): Breakdown` | Full request breakdown |
| | `countChatTotal` | `(rawJson: string, options?: Options): number` | Quick total token count |
| | `countChatMessage` | `(rawJson: string, options?: Options): BlockBreakdown` | Single message tokens |
| | `countChatTool` | `(rawJson: string): number` | Single tool definition |
| **Anthropic** | `countAnthropicRequest` | `(rawJson: string, options?: Options): Breakdown` | Full request breakdown |
| | `countAnthropicTotal` | `(rawJson: string, options?: Options): number` | Quick total token count |
| | `countAnthropicMessage`| `(rawJson: string, options?: Options): BlockBreakdown` | Single message tokens |
| | `countAnthropicBlock` | `(rawJson: string, options?: Options): BlockBreakdown` | Single content block |
| **OpenAI Responses**| `countResponsesRequest`| `(rawJson: string, options?: Options): Breakdown` | Full Responses API request |
| | `countResponsesTotal` | `(rawJson: string, options?: Options): number` | Quick total token count |
| **KV Cache** | `kvInit` | `(options?: { ttlSeconds?: number, maxMB?: number, separateProtocol?: boolean }): KvConfig` | Configures prefix simulator |
| | `kvCache` | `(rawJson: string, protocol: Protocol, namespace: string, options?: Options): KvResult` | Simulates prefix cache hit/miss |
| | `kvStats` | `(): KvStats` | Global cache telemetry |
| | `kvClear` | `(namespace?: string): void` | Invalidate cache entries |

---

## 📄 License

MIT © [italoalmeida0](https://github.com/italoalmeida0)
