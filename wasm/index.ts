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
      if (typeof process !== "undefined" && process.versions && (process.versions.node || process.versions.bun)) {
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
      if (typeof process !== "undefined" && process.versions && (process.versions.node || process.versions.bun) && !urlStr.startsWith("http://") && !urlStr.startsWith("https://")) {
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

      countChatRequest(request: object, options?: Options): Breakdown {
        const str = JSON.stringify(request);
        const res = wasm.countChatRequestJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<Breakdown>(res);
      },

      countChatTotal(request: object, options?: Options): number {
        const str = JSON.stringify(request);
        const res = wasm.countChatTotalQuick(str, !!options?.tight, !!options?.ignoreImages);
        if (res < 0) {
          throw new Error("BTDby4 error: failed to count chat request total");
        }
        return res;
      },

      countChatMessage(message: object, options?: Options): BlockBreakdown {
        const str = JSON.stringify(message);
        const res = wasm.countChatMessageJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countChatPart(part: object, options?: Options): BlockBreakdown {
        const str = JSON.stringify(part);
        const res = wasm.countChatPartJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countChatTool(tool: object): number {
        const str = JSON.stringify(tool);
        const res = wasm.countChatToolJSON(str);
        if (res < 0) throw new Error("BTDby4 error: failed to count chat tool");
        return res;
      },

      countAnthropicRequest(request: object, options?: Options): Breakdown {
        const str = JSON.stringify(request);
        const res = wasm.countAnthropicRequestJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<Breakdown>(res);
      },

      countAnthropicTotal(request: object, options?: Options): number {
        const str = JSON.stringify(request);
        const res = wasm.countAnthropicTotalQuick(str, !!options?.tight, !!options?.ignoreImages);
        if (res < 0) {
          throw new Error("BTDby4 error: failed to count anthropic request total");
        }
        return res;
      },

      countAnthropicMessage(message: object, options?: Options): BlockBreakdown {
        const str = JSON.stringify(message);
        const res = wasm.countAnthropicMessageJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countAnthropicBlock(block: object, options?: Options): BlockBreakdown {
        const str = JSON.stringify(block);
        const res = wasm.countAnthropicBlockJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countAnthropicTool(tool: object): number {
        const str = JSON.stringify(tool);
        const res = wasm.countAnthropicToolJSON(str);
        if (res < 0) throw new Error("BTDby4 error: failed to count anthropic tool");
        return res;
      },

      countRequest(request: object, options?: Options): Breakdown {
        return this.countAnthropicRequest(request, options);
      },

      countMessage(message: object, options?: Options): BlockBreakdown {
        return this.countAnthropicMessage(message, options);
      },

      countBlock(block: object, options?: Options): BlockBreakdown {
        return this.countAnthropicBlock(block, options);
      },

      countTool(tool: object): number {
        return this.countAnthropicTool(tool);
      },

      countResponsesRequest(request: object, options?: Options): Breakdown {
        const str = JSON.stringify(request);
        const res = wasm.countResponsesRequestJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<Breakdown>(res);
      },

      countResponsesTotal(request: object, options?: Options): number {
        const str = JSON.stringify(request);
        const res = wasm.countResponsesTotalQuick(str, !!options?.tight, !!options?.ignoreImages);
        if (res < 0) {
          throw new Error("BTDby4 error: failed to count responses request total");
        }
        return res;
      },

      countResponsesItem(item: object, options?: Options): BlockBreakdown {
        const str = JSON.stringify(item);
        const res = wasm.countResponsesItemJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countResponsesPart(part: object, options?: Options): BlockBreakdown {
        const str = JSON.stringify(part);
        const res = wasm.countResponsesPartJSON(str, !!options?.tight, !!options?.ignoreImages);
        return parseResult<BlockBreakdown>(res);
      },

      countResponsesTool(tool: object): number {
        const str = JSON.stringify(tool);
        const res = wasm.countResponsesToolJSON(str);
        if (res < 0) throw new Error("BTDby4 error: failed to count responses tool");
        return res;
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
