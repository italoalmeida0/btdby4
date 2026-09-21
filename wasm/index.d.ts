export interface Breakdown {
  system: number;
  messages: number;
  tools: number;
  images: number;
  by_message?: number[];
  by_tool?: number[];
  text_tokens: number;
  total: number;
  image_count: number;
}

export interface BlockBreakdown {
  tokens: number;
  images: number;
  text_tokens: number;
  image_count: number;
}

export interface Options {
  tight?: boolean;
  ignoreImages?: boolean;
}

export interface BTDby4Instance {
  // --- Text & Reasoning ---
  countText(text: string): number;
  estimateThinkingTokens(encryptedPayload: string): number;

  // --- Image Helpers ---
  countImageSize(width: number, height: number): number;
  countImageBase64(base64Str: string): number;
  countImageBytes(bytes: Uint8Array | ArrayBuffer): number;

  // --- OpenAI Chat Completions ---
  countChatRequest(request: object, options?: Options): Breakdown;
  countChatTotal(request: object, options?: Options): number;
  countChatMessage(message: object, options?: Options): BlockBreakdown;
  countChatPart(part: object, options?: Options): BlockBreakdown;
  countChatTool(tool: object): number;

  // --- Anthropic Messages ---
  countAnthropicRequest(request: object, options?: Options): Breakdown;
  countAnthropicTotal(request: object, options?: Options): number;
  countAnthropicMessage(message: object, options?: Options): BlockBreakdown;
  countAnthropicBlock(block: object, options?: Options): BlockBreakdown;
  countAnthropicTool(tool: object): number;

  // --- Legacy Aliases for Anthropic ---
  countRequest(request: object, options?: Options): Breakdown;
  countMessage(message: object, options?: Options): BlockBreakdown;
  countBlock(block: object, options?: Options): BlockBreakdown;
  countTool(tool: object): number;

  // --- OpenAI Responses ---
  countResponsesRequest(request: object, options?: Options): Breakdown;
  countResponsesTotal(request: object, options?: Options): number;
  countResponsesItem(item: object, options?: Options): BlockBreakdown;
  countResponsesPart(part: object, options?: Options): BlockBreakdown;
  countResponsesTool(tool: object): number;
}

export type WasmSource = BufferSource | Response | URL | string;

export function initBTDby4(wasmSource?: WasmSource): Promise<BTDby4Instance>;
export function getBTDby4(): BTDby4Instance;
export default initBTDby4;
