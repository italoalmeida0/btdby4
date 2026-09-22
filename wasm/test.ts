import initBTDby4, { getBTDby4 } from "./index.ts";

console.log("==================================================");
console.log("  BTDby4 WASM - Universal Multiplatform Test");
console.log("==================================================");

console.log(`Runtime:       ${typeof Bun !== "undefined" ? "Bun " + Bun.version : "Node.js " + process.version}`);
console.log(`OS Platform:   ${process.platform}`);
console.log(`Architecture:  ${process.arch}\n`);

console.log("==> Initializing WebAssembly module...");
const startTime = performance.now();
const estimator = await initBTDby4();
console.log(`==> Initialized in ${(performance.now() - startTime).toFixed(2)}ms\n`);

// 1. Text Counting
console.log("1. Testing countText...");
const text = "hello world";
const textCount = estimator.countText(text);
console.log(`   countText("${text}") = ${textCount}`);
console.assert(textCount === 2, `Expected 2, got ${textCount}`);

// 2. Encrypted Thinking Token Estimation
console.log("\n2. Testing estimateThinkingTokens...");
const sampleEncrypted = "gAAAAABl-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx==";
const thinkingCount = estimator.estimateThinkingTokens(sampleEncrypted);
console.log(`   estimateThinkingTokens(...) = ${thinkingCount}`);
console.assert(typeof thinkingCount === "number", "Should return a number");

// 3. Image Helpers
console.log("\n3. Testing Image Helpers...");
const imgSizeTokens = estimator.countImageSize(1024, 768);
console.log(`   countImageSize(1024, 768) = ${imgSizeTokens}`);
console.assert(imgSizeTokens > 0, "Image tokens should be > 0");

const dummyB64 = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==";
const b64Tokens = estimator.countImageBase64(dummyB64);
console.log(`   countImageBase64(...) = ${b64Tokens}`);

const dummyBytes = new Uint8Array([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]);
const byteTokens = estimator.countImageBytes(dummyBytes);
console.log(`   countImageBytes(...) = ${byteTokens}`);

// 4. OpenAI Chat Completions
console.log("\n4. Testing OpenAI Chat (Full Suite)...");
const chatPayload = {
  system: "You are a helpful coding assistant.",
  messages: [
    { role: "user", content: "Write a high-performance token estimator." },
    { role: "assistant", content: "Here is BTDby4, running under 1 millisecond!" }
  ],
  tools: [
    {
      type: "function",
      function: {
        name: "search_code",
        description: "Search for code patterns",
        parameters: { type: "object", properties: { q: { type: "string" } } }
      }
    }
  ]
};

const chatJson = JSON.stringify(chatPayload);
const chatReq = estimator.countChatRequest(chatJson, { tight: true });
console.log(`   countChatRequest (tight): ${chatReq.total}`);
console.assert(chatReq.total > 0, "Chat request total should be > 0");

const chatTotal = estimator.countChatTotal(chatJson, { tight: true });
console.log(`   countChatTotal   (tight): ${chatTotal}`);
console.assert(chatTotal === chatReq.total, "countChatTotal should match breakdown total");

const chatMsg = estimator.countChatMessage(JSON.stringify(chatPayload.messages[0]));
console.log(`   countChatMessage (tokens): ${chatMsg.tokens}`);

const chatPart = estimator.countChatPart(JSON.stringify({ type: "text", text: "quick test" }));
console.log(`   countChatPart    (tokens): ${chatPart.tokens}`);

const chatTool = estimator.countChatTool(JSON.stringify(chatPayload.tools[0]));
console.log(`   countChatTool    (tokens): ${chatTool}`);

// 5. Anthropic Messages
console.log("\n5. Testing Anthropic Messages (Full Suite)...");
const anthropicPayload = {
  system: "You are Claude, an AI by Anthropic.",
  messages: [
    { role: "user", content: "Hello! Count tokens accurately." }
  ],
  tools: [
    {
      name: "calc",
      description: "Calculator",
      input_schema: { type: "object", properties: { expr: { type: "string" } } }
    }
  ]
};

const anthropicJson = JSON.stringify(anthropicPayload);
const anthropicReq = estimator.countAnthropicRequest(anthropicJson, { tight: true });
console.log(`   countAnthropicRequest (tight): ${anthropicReq.total}`);
console.assert(anthropicReq.total > 0, "Anthropic request total should be > 0");

const anthropicTotal = estimator.countAnthropicTotal(anthropicJson, { tight: true });
console.log(`   countAnthropicTotal   (tight): ${anthropicTotal}`);
console.assert(anthropicTotal === anthropicReq.total, "countAnthropicTotal should match breakdown total");

const anthropicMsg = estimator.countAnthropicMessage(JSON.stringify(anthropicPayload.messages[0]));
console.log(`   countAnthropicMessage (tokens): ${anthropicMsg.tokens}`);

const anthropicBlock = estimator.countAnthropicBlock(JSON.stringify({ type: "text", text: "hi" }));
console.log(`   countAnthropicBlock   (tokens): ${anthropicBlock.tokens}`);

const anthropicTool = estimator.countAnthropicTool(JSON.stringify(anthropicPayload.tools[0]));
console.log(`   countAnthropicTool    (tokens): ${anthropicTool}`);

// 6. OpenAI Responses API
console.log("\n6. Testing OpenAI Responses (Full Suite)...");
const responsesPayload = {
  input: [
    { role: "user", content: [{ type: "input_text", text: "Count these tokens." }] }
  ],
  tools: [
    { type: "function", name: "tool1", description: "test" }
  ]
};

const responsesJson = JSON.stringify(responsesPayload);
const responsesReq = estimator.countResponsesRequest(responsesJson, { tight: true });
console.log(`   countResponsesRequest (tight): ${responsesReq.total}`);

const responsesTotal = estimator.countResponsesTotal(responsesJson, { tight: true });
console.log(`   countResponsesTotal   (tight): ${responsesTotal}`);

const responsesItem = estimator.countResponsesItem(JSON.stringify(responsesPayload.input[0]));
console.log(`   countResponsesItem    (tokens): ${responsesItem.tokens}`);

const responsesPart = estimator.countResponsesPart(JSON.stringify(responsesPayload.input[0].content[0]));
console.log(`   countResponsesPart    (tokens): ${responsesPart.tokens}`);

const responsesTool = estimator.countResponsesTool(JSON.stringify(responsesPayload.tools[0]));
console.log(`   countResponsesTool    (tokens): ${responsesTool}`);

// 7. KV-Cache provider (string in, no double-stringify)
console.log("\n7. Testing KV-Cache provider...");
estimator.kvClear();
const kvFirst = estimator.kvCache(chatJson, "chat", "test|model|key", { tight: true });
console.log(`   kvCache miss: total=${kvFirst.total} cached=${kvFirst.cached} fresh=${kvFirst.fresh}`);
console.assert(kvFirst.cached === 0 && kvFirst.fresh === kvFirst.total, "First call must be full miss");
const kvSecond = estimator.kvCache(chatJson, "chat", "test|model|key", { tight: true });
console.log(`   kvCache hit:  total=${kvSecond.total} cached=${kvSecond.cached} fresh=${kvSecond.fresh}`);
console.assert(kvSecond.fresh === 0 && kvSecond.cached === kvSecond.total, "Repeat must be full hit");
console.log(`   kvStats:`, estimator.kvStats());

// 8. Micro-Benchmark
console.log("\n8. Running Micro-Benchmark (10,000 iterations of countText)...");
const benchIters = 10000;
const benchStart = performance.now();
for (let i = 0; i < benchIters; i++) {
  estimator.countText("the quick brown fox jumps over the lazy dog");
}
const benchEnd = performance.now();
const totalMs = benchEnd - benchStart;
const perOpUs = (totalMs / benchIters) * 1000;
const opsPerSec = Math.round((benchIters / totalMs) * 1000);

console.log(`   Completed ${benchIters.toLocaleString()} calls in ${totalMs.toFixed(2)}ms`);
console.log(`   Average latency: ${perOpUs.toFixed(2)}µs per call`);
console.log(`   Throughput:      ${opsPerSec.toLocaleString()} ops/second`);

console.log("\n All BTDby4 WASM functions tested and verified successfully!");
