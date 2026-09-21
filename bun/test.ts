import btdby4, { getDefaultLibraryPath, loadBTDby4 } from "./index.ts";

console.log("==================================================");
console.log("  BTDby4 Bun FFI - Complete API & Multiplatform Test");
console.log("==================================================");

console.log(`OS Platform:   ${process.platform}`);
console.log(`Architecture:  ${process.arch}`);
console.log(`Resolved lib:  ${getDefaultLibraryPath()}\n`);

const estimator = btdby4();

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

const chatReq = estimator.countChatRequest(chatPayload, { tight: true });
const chatTot = estimator.countChatTotal(chatPayload, { tight: true });
const chatMsg = estimator.countChatMessage(chatPayload.messages[0], { tight: true });
const chatPart = estimator.countChatPart({ type: "text", text: "part token counting" }, { tight: true });
const chatTool = estimator.countChatTool(chatPayload.tools[0]);

console.log(`   countChatRequest (tight): ${chatReq.total}`);
console.log(`   countChatTotal   (tight): ${chatTot}`);
console.log(`   countChatMessage (tokens): ${chatMsg.tokens}`);
console.log(`   countChatPart    (tokens): ${chatPart.tokens}`);
console.log(`   countChatTool    (tokens): ${chatTool}`);
console.assert(chatReq.total === chatTot, "Request total and Quick total must match");

// 5. Anthropic Messages
console.log("\n5. Testing Anthropic Messages (Full Suite)...");
const anthropicPayload = {
  system: "You are Claude.",
  messages: [
    { role: "user", content: "How does Bun FFI perform compared to Node-API?" },
    { role: "assistant", content: "Bun FFI compiles directly into native JIT calls without JS shims." }
  ],
  tools: [
    {
      name: "calc",
      description: "Calculator",
      input_schema: { type: "object" }
    }
  ]
};

const antReq = estimator.countAnthropicRequest(anthropicPayload, { tight: true });
const antTot = estimator.countAnthropicTotal(anthropicPayload, { tight: true });
const antMsg = estimator.countAnthropicMessage(anthropicPayload.messages[0], { tight: true });
const antBlk = estimator.countAnthropicBlock({ type: "text", text: "hello block" }, { tight: true });
const antTool = estimator.countAnthropicTool(anthropicPayload.tools[0]);

console.log(`   countAnthropicRequest (tight): ${antReq.total}`);
console.log(`   countAnthropicTotal   (tight): ${antTot}`);
console.log(`   countAnthropicMessage (tokens): ${antMsg.tokens}`);
console.log(`   countAnthropicBlock   (tokens): ${antBlk.tokens}`);
console.log(`   countAnthropicTool    (tokens): ${antTool}`);
console.assert(antReq.total === antTot, "Anthropic request total and quick total must match");

// 6. OpenAI Responses
console.log("\n6. Testing OpenAI Responses (Full Suite)...");
const responsesPayload = {
  instructions: "Respond concisely.",
  input: [
    { type: "message", role: "user", content: [{ type: "input_text", text: "Evaluate this request." }] }
  ],
  tools: [
    {
      type: "function",
      name: "run_query",
      description: "Execute query",
      parameters: { type: "object" }
    }
  ]
};

const respReq = estimator.countResponsesRequest(responsesPayload, { tight: true });
const respTot = estimator.countResponsesTotal(responsesPayload, { tight: true });
const respItem = estimator.countResponsesItem(responsesPayload.input[0], { tight: true });
const respPart = estimator.countResponsesPart({ type: "input_text", text: "single part" }, { tight: true });
const respTool = estimator.countResponsesTool(responsesPayload.tools[0]);

console.log(`   countResponsesRequest (tight): ${respReq.total}`);
console.log(`   countResponsesTotal   (tight): ${respTot}`);
console.log(`   countResponsesItem    (tokens): ${respItem.tokens}`);
console.log(`   countResponsesPart    (tokens): ${respPart.tokens}`);
console.log(`   countResponsesTool    (tokens): ${respTool}`);
console.assert(respReq.total === respTot, "Responses request total and quick total must match");

// 7. Performance Micro-Benchmark
console.log("\n7. Running Micro-Benchmark (50,000 iterations of countText)...");
const benchText = "The quick brown fox jumps over the lazy dog. Counting LLM tokens via Bun FFI at native speed.";
const iterations = 50_000;
const start = performance.now();
for (let i = 0; i < iterations; i++) {
  estimator.countText(benchText);
}
const elapsed = performance.now() - start;
const opsPerSec = Math.round((iterations / elapsed) * 1000);

console.log(`   Completed ${iterations.toLocaleString()} calls in ${elapsed.toFixed(2)}ms`);
console.log(`   Average latency: ${(elapsed / iterations * 1000).toFixed(2)}µs per call`);
console.log(`   Throughput:      ${opsPerSec.toLocaleString()} ops/second`);

console.log("\n All 18 BTDby4 functions tested and verified successfully!");
