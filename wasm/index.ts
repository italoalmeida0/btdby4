import "./crypto_polyfill.js";
import "./wasm_exec.js";

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
  /**
   * Disables the built-in provider framing safety margin and returns the
   * closest point estimate. Default (false) keeps the safety margin.
   */
  tight?: boolean;
  /**
   * If true, counts images as zero tokens.
   */
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

let cachedInstance: BTDby4Instance | null = null;
let initPromise: Promise<BTDby4Instance> | null = null;

export type WasmSource = BufferSource | Response | URL | string;

/**
 * Initializes the universal WebAssembly BTDby4 estimator.
 * If wasmSource is omitted in Node/Bun, it loads btdby4.wasm from the package folder.
 */
export async function initBTDby4(wasmSource?: WasmSource): Promise<BTDby4Instance> {
  if (cachedInstance) {
    return cachedInstance;
  }
  if (initPromise) {
    return initPromise;
  }

  initPromise = (async () => {
    // 1. Obtain bytes
    let bytes: BufferSource;

    if (!wasmSource) {
      if (typeof process !== "undefined" && process.versions && (process.versions.node || (process.versions as any).bun)) {
        const { readFileSync } = await import("fs");
        const wasmUrl = new URL("btdby4.wasm", import.meta.url);
        bytes = readFileSync(wasmUrl);
      } else if (typeof fetch !== "undefined") {
        const res = await fetch("btdby4.wasm");
        bytes = await res.arrayBuffer();
      } else {
        throw new Error("Cannot automatically resolve btdby4.wasm in this environment. Please pass wasmSource to initBTDby4().");
      }
    } else if (wasmSource instanceof ArrayBuffer || ArrayBuffer.isView(wasmSource)) {
      bytes = wasmSource;
    } else if (typeof Response !== "undefined" && wasmSource instanceof Response) {
      bytes = await wasmSource.arrayBuffer();
    } else if (typeof wasmSource === "string" || wasmSource instanceof URL) {
      const urlStr = wasmSource.toString();
      if (typeof process !== "undefined" && process.versions && (process.versions.node || (process.versions as any).bun) && !urlStr.startsWith("http://") && !urlStr.startsWith("https://")) {
        const { readFileSync } = await import("fs");
        bytes = readFileSync(urlStr);
      } else {
        const res = await fetch(urlStr);
        bytes = await res.arrayBuffer();
      }
    } else {
      throw new Error("Unsupported wasmSource type passed to initBTDby4()");
    }

    // 2. Instantiate Go WASM
    const GoClass = (globalThis as any).Go;
    if (!GoClass) {
      throw new Error("Go WebAssembly runtime (wasm_exec.js) not found in global scope.");
    }

    const go = new GoClass();
    const wasmModule = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(wasmModule.instance);

    const wasm = (globalThis as any).__btdby4_wasm_instance;
    if (!wasm) {
      throw new Error("Failed to initialize Go WebAssembly BTDby4 instance.");
    }

    function parseResult<T>(jsonStr: string): T {
      const parsed = JSON.parse(jsonStr);
      if (parsed && parsed.error) {
        throw new Error(`BTDby4 error: ${parsed.error}`);
      }
      return parsed as T;
    }

    const instance: BTDby4Instance = {
      countText(text: string): number {
        return wasm.countText(text);
      },

      estimateThinkingTokens(encryptedPayload: string): number {
        return wasm.estimateThinkingTokens(encryptedPayload);
      },

      countImageSize(width: number, height: number): number {
        return wasm.countImageSize(width, height);
      },

      countImageBase64(base64Str: string): number {
        return wasm.countImageBase64(base64Str);
      },

      countImageBytes(bytes: Uint8Array | ArrayBuffer): number {
        const u8 = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
        return wasm.countImageBytes(u8);
      },

      countChatRequest(request: string, options?: Options): Breakdown {
        const res = wasm.countChatRequestJSON(request, !!options?.tight, !!options?.ignoreImages);
        return parseResult<Breakdown>(res);
      },

      countChatTotal(request: string, options?: Options): number {
        const res = wasm.countChatTotalQuick(request, !!options?.tight, !!options?.ignoreImages);
        if (res < 0) {
          throw new Error("BTDby4 error: failed to count chat request total");
        }
        return res;
      },

      countChatMessage(message: string, options?: Options): BlockBreakdown {
        const res = wasm.countChatMessageJSON(message, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countChatPart(part: string, options?: Options): BlockBreakdown {
        const res = wasm.countChatPartJSON(part, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countChatTool(tool: string): number {
        const res = wasm.countChatToolJSON(tool);
        if (res < 0) throw new Error("BTDby4 error: failed to count chat tool");
        return res;
      },

      countAnthropicRequest(request: string, options?: Options): Breakdown {
        const res = wasm.countAnthropicRequestJSON(request, !!options?.tight, !!options?.ignoreImages);
        return parseResult<Breakdown>(res);
      },

      countAnthropicTotal(request: string, options?: Options): number {
        const res = wasm.countAnthropicTotalQuick(request, !!options?.tight, !!options?.ignoreImages);
        if (res < 0) {
          throw new Error("BTDby4 error: failed to count anthropic request total");
        }
        return res;
      },

      countAnthropicMessage(message: string, options?: Options): BlockBreakdown {
        const res = wasm.countAnthropicMessageJSON(message, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countAnthropicBlock(block: string, options?: Options): BlockBreakdown {
        const res = wasm.countAnthropicBlockJSON(block, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countAnthropicTool(tool: string): number {
        const res = wasm.countAnthropicToolJSON(tool);
        if (res < 0) throw new Error("BTDby4 error: failed to count anthropic tool");
        return res;
      },

      countResponsesRequest(request: string, options?: Options): Breakdown {
        const res = wasm.countResponsesRequestJSON(request, !!options?.tight, !!options?.ignoreImages);
        return parseResult<Breakdown>(res);
      },

      countResponsesTotal(request: string, options?: Options): number {
        const res = wasm.countResponsesTotalQuick(request, !!options?.tight, !!options?.ignoreImages);
        if (res < 0) {
          throw new Error("BTDby4 error: failed to count responses request total");
        }
        return res;
      },

      countResponsesItem(item: string, options?: Options): BlockBreakdown {
        const res = wasm.countResponsesItemJSON(item, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countResponsesPart(part: string, options?: Options): BlockBreakdown {
        const res = wasm.countResponsesPartJSON(part, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countResponsesTool(tool: string): number {
        const res = wasm.countResponsesToolJSON(tool);
        if (res < 0) throw new Error("BTDby4 error: failed to count responses tool");
        return res;
      },

      kvCache(request: string, protocol: KvProtocol, namespace: string, options?: Options): KvResult {
        const res = wasm.kvCacheJSON(request, protocol, namespace, !!options?.tight, !!options?.ignoreImages);
        return parseResult<KvResult>(res);
      },

      kvStats(): KvStats {
        const res = wasm.kvStatsJSON();
        return parseResult<KvStats>(res);
      },

      kvInit(options?: KvInitOptions): KvConfig {
        const res = wasm.kvInit(options?.ttlSeconds ?? 0, options?.maxMB ?? 0, options?.separateProtocol ?? true);
        return parseResult<KvConfig>(res);
      },

      kvClear(namespace?: string): void {
        wasm.kvClear(namespace ?? "");
      },
    };

    cachedInstance = instance;
    return instance;
  })();

  return initPromise;
}

/**
 * Returns the synchronously initialized instance, throwing if initBTDby4() has not completed.
 */
export function getBTDby4(): BTDby4Instance {
  if (!cachedInstance) {
    throw new Error("BTDby4 WebAssembly is not initialized yet. Please call and await initBTDby4() first.");
  }
  return cachedInstance;
}

export default initBTDby4;
