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

export type KvProtocol = "anthropic" | "chat" | "responses";

export interface KvResult {
  total: number;
  cached: number;
  fresh: number;
  written: number;
  hit: boolean;
  hit_ratio: number;
  prefix_blocks: number;
  total_blocks: number;
  breakdown: Breakdown;
}

export interface KvStats {
  namespaces: number;
  nodes: number;
  branches: number;
  tokens: number;
  bytes: number;
  max_bytes: number;
  /** Free cache memory before LRU eviction kicks in. */
  available_bytes: number;
  /** Configured sliding TTL in seconds. */
  ttl_seconds: number;
}

export interface KvConfig {
  ttl_seconds: number;
  max_mb: number;
  separate_protocol: boolean;
}

export interface KvInitOptions {
  /** TTL in seconds. Default 600 (10min). 0/omitted = default. */
  ttlSeconds?: number;
  /** Memory limit in MB. Default 400. 0/omitted = default. */
  maxMB?: number;
  /** Key by protocol+namespace (default true). false = namespace only. */
  separateProtocol?: boolean;
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
  countChatRequest(request: string, options?: Options): Breakdown;
  countChatTotal(request: string, options?: Options): number;
  countChatMessage(message: string, options?: Options): BlockBreakdown;
  countChatPart(part: string, options?: Options): BlockBreakdown;
  countChatTool(tool: string): number;

  // --- Anthropic Messages ---
  countAnthropicRequest(request: string, options?: Options): Breakdown;
  countAnthropicTotal(request: string, options?: Options): number;
  countAnthropicMessage(message: string, options?: Options): BlockBreakdown;
  countAnthropicBlock(block: string, options?: Options): BlockBreakdown;
  countAnthropicTool(tool: string): number;

  // --- OpenAI Responses ---
  countResponsesRequest(request: string, options?: Options): Breakdown;
  countResponsesTotal(request: string, options?: Options): number;
  countResponsesItem(item: string, options?: Options): BlockBreakdown;
  countResponsesPart(part: string, options?: Options): BlockBreakdown;
  countResponsesTool(tool: string): number;

  // --- KV-Cache provider (prefix simulation) ---
  kvCache(request: string, protocol: KvProtocol, namespace: string, options?: Options): KvResult;
  kvStats(): KvStats;
  kvInit(options?: KvInitOptions): KvConfig;
  kvClear(namespace?: string): void;
}

export type WasmSource = BufferSource | Response | URL | string;

export function initBTDby4(wasmSource?: WasmSource): Promise<BTDby4Instance>;
export function getBTDby4(): BTDby4Instance;
export default initBTDby4;
