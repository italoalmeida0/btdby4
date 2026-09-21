import { dlopen, FFIType, CString, ptr } from "bun:ffi";
import { resolve } from "path";
import { existsSync } from "fs";

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

export interface LoadOptions {
  /**
   * Force using the musl libc build on Linux (e.g. Alpine Linux).
   * If omitted, it will be detected automatically.
   */
  musl?: boolean;
  /**
   * Custom path to the native library file.
   */
  customPath?: string;
}

/**
 * Detects if the current Linux system is running on musl libc (e.g. Alpine Linux).
 */
export function isMuslLinux(): boolean {
  if (process.platform !== "linux") return false;
  try {
    return (
      existsSync("/etc/alpine-release") ||
      existsSync("/lib/ld-musl-x86_64.so.1") ||
      existsSync("/lib/ld-musl-aarch64.so.1") ||
      existsSync("/usr/lib/ld-musl-x86_64.so.1") ||
      existsSync("/usr/lib/ld-musl-aarch64.so.1")
    );
  } catch {
    return false;
  }
}

/**
 * Resolves the appropriate pre-compiled native library based on current OS, CPU architecture, and libc.
 */
export function getDefaultLibraryPath(preferMusl?: boolean): string {
  const platform = process.platform;
  const arch = process.arch === "ia32" ? "amd64" : process.arch === "x64" ? "amd64" : process.arch;
  const musl = preferMusl ?? isMuslLinux();

  let filename = "";

  if (platform === "win32") {
    filename = arch === "arm64" ? "libbtdby4-windows-arm64.dll" : "libbtdby4-windows-amd64.dll";
  } else if (platform === "linux") {
    if (musl) {
      filename = arch === "arm64" ? "libbtdby4-linux-arm64-musl.so" : "libbtdby4-linux-amd64-musl.so";
    } else {
      filename = arch === "arm64" ? "libbtdby4-linux-arm64.so" : "libbtdby4-linux-amd64.so";
    }
  } else if (platform === "darwin") {
    filename = arch === "arm64" ? "libbtdby4-darwin-arm64.dylib" : "libbtdby4-darwin-amd64.dylib";
  } else {
    throw new Error(`Unsupported platform for BTDby4: ${platform}-${arch}`);
  }

  return resolve(import.meta.dir, "lib", filename);
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

/**
 * Loads the BTDby4 native shared library into Bun FFI.
 * @param options Optional custom path or musl preference.
 */
export function loadBTDby4(options?: string | LoadOptions): BTDby4Instance {
  const customPath = typeof options === "string" ? options : options?.customPath;
  const musl = typeof options === "object" ? options.musl : undefined;
  const libPath = customPath ?? getDefaultLibraryPath(musl);

  const { symbols } = dlopen(libPath, {
    CountText: { args: [FFIType.cstring], returns: FFIType.i32 },
    EstimateThinkingTokens: { args: [FFIType.cstring], returns: FFIType.i32 },
    CountImageSize: { args: [FFIType.i32, FFIType.i32], returns: FFIType.i32 },
    CountImageBase64: { args: [FFIType.cstring], returns: FFIType.i32 },
    CountImageBytes: { args: [FFIType.ptr, FFIType.i32], returns: FFIType.i32 },

    // OpenAI Chat
    CountChatRequestJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountChatTotalQuick: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.i32 },
    CountChatMessageJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountChatPartJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountChatToolJSON: { args: [FFIType.cstring], returns: FFIType.i32 },

    // Anthropic Messages
    CountAnthropicRequestJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountAnthropicTotalQuick: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.i32 },
    CountAnthropicMessageJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountAnthropicBlockJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountAnthropicToolJSON: { args: [FFIType.cstring], returns: FFIType.i32 },

    // OpenAI Responses
    CountResponsesRequestJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountResponsesTotalQuick: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.i32 },
    CountResponsesItemJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountResponsesPartJSON: { args: [FFIType.cstring, FFIType.i32, FFIType.i32], returns: FFIType.ptr },
    CountResponsesToolJSON: { args: [FFIType.cstring], returns: FFIType.i32 },

    FreeCString: { args: [FFIType.ptr], returns: FFIType.void },
  });

  const toCString = (str: string) =>
    Buffer.from(
      (str.indexOf("\0") !== -1 ? str.replace(/\0/g, "") : str) + "\0",
      "utf8"
    );

  function parsePtr<T>(ptrVal: number): T {
    if (!ptrVal) throw new Error("BTDby4 FFI returned null pointer");
    const jsonStr = new CString(ptrVal);
    symbols.FreeCString(ptrVal);
    const parsed = JSON.parse(jsonStr);
    if (parsed.error) {
      throw new Error(`BTDby4 error: ${parsed.error}`);
    }
    return parsed as T;
  }

  const instance: BTDby4Instance = {
    countText(text: string): number {
      return symbols.CountText(toCString(text));
    },

    estimateThinkingTokens(encryptedPayload: string): number {
      return symbols.EstimateThinkingTokens(toCString(encryptedPayload));
    },

    countImageSize(width: number, height: number): number {
      return symbols.CountImageSize(width, height);
    },

    countImageBase64(base64Str: string): number {
      return symbols.CountImageBase64(toCString(base64Str));
    },

    countImageBytes(bytes: Uint8Array | ArrayBuffer): number {
      const u8 = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
      return symbols.CountImageBytes(ptr(u8), u8.byteLength);
    },

    // OpenAI Chat
    countChatRequest(request: object, options?: Options): Breakdown {
      const json = toCString(JSON.stringify(request));
      const resPtr = symbols.CountChatRequestJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<Breakdown>(resPtr);
    },

    countChatTotal(request: object, options?: Options): number {
      const json = toCString(JSON.stringify(request));
      const res = symbols.CountChatTotalQuick(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      if (res < 0) throw new Error("Failed to count Chat request total");
      return res;
    },

    countChatMessage(message: object, options?: Options): BlockBreakdown {
      const json = toCString(JSON.stringify(message));
      const resPtr = symbols.CountChatMessageJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<BlockBreakdown>(resPtr);
    },

    countChatPart(part: object, options?: Options): BlockBreakdown {
      const json = toCString(JSON.stringify(part));
      const resPtr = symbols.CountChatPartJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<BlockBreakdown>(resPtr);
    },

    countChatTool(tool: object): number {
      const json = toCString(JSON.stringify(tool));
      const res = symbols.CountChatToolJSON(json);
      if (res < 0) throw new Error("Failed to count Chat tool");
      return res;
    },

    // Anthropic Messages
    countAnthropicRequest(request: object, options?: Options): Breakdown {
      const json = toCString(JSON.stringify(request));
      const resPtr = symbols.CountAnthropicRequestJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<Breakdown>(resPtr);
    },

    countAnthropicTotal(request: object, options?: Options): number {
      const json = toCString(JSON.stringify(request));
      const res = symbols.CountAnthropicTotalQuick(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      if (res < 0) throw new Error("Failed to count Anthropic request total");
      return res;
    },

    countAnthropicMessage(message: object, options?: Options): BlockBreakdown {
      const json = toCString(JSON.stringify(message));
      const resPtr = symbols.CountAnthropicMessageJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<BlockBreakdown>(resPtr);
    },

    countAnthropicBlock(block: object, options?: Options): BlockBreakdown {
      const json = toCString(JSON.stringify(block));
      const resPtr = symbols.CountAnthropicBlockJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<BlockBreakdown>(resPtr);
    },

    countAnthropicTool(tool: object): number {
      const json = toCString(JSON.stringify(tool));
      const res = symbols.CountAnthropicToolJSON(json);
      if (res < 0) throw new Error("Failed to count Anthropic tool");
      return res;
    },

    // Aliases
    countRequest(req, opts) { return instance.countAnthropicRequest(req, opts); },
    countMessage(msg, opts) { return instance.countAnthropicMessage(msg, opts); },
    countBlock(blk, opts) { return instance.countAnthropicBlock(blk, opts); },
    countTool(tl) { return instance.countAnthropicTool(tl); },

    // OpenAI Responses
    countResponsesRequest(request: object, options?: Options): Breakdown {
      const json = toCString(JSON.stringify(request));
      const resPtr = symbols.CountResponsesRequestJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<Breakdown>(resPtr);
    },

    countResponsesTotal(request: object, options?: Options): number {
      const json = toCString(JSON.stringify(request));
      const res = symbols.CountResponsesTotalQuick(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      if (res < 0) throw new Error("Failed to count Responses request total");
      return res;
    },

    countResponsesItem(item: object, options?: Options): BlockBreakdown {
      const json = toCString(JSON.stringify(item));
      const resPtr = symbols.CountResponsesItemJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<BlockBreakdown>(resPtr);
    },

    countResponsesPart(part: object, options?: Options): BlockBreakdown {
      const json = toCString(JSON.stringify(part));
      const resPtr = symbols.CountResponsesPartJSON(
        json,
        options?.tight ? 1 : 0,
        options?.ignoreImages ? 1 : 0
      );
      return parsePtr<BlockBreakdown>(resPtr);
    },

    countResponsesTool(tool: object): number {
      const json = toCString(JSON.stringify(tool));
      const res = symbols.CountResponsesToolJSON(json);
      if (res < 0) throw new Error("Failed to count Responses tool");
      return res;
    },
  };

  return instance;
}

// Default singleton export
let defaultInstance: BTDby4Instance | null = null;
export default function btdby4(options?: string | LoadOptions): BTDby4Instance {
  if (options) {
    return loadBTDby4(options);
  }
  if (!defaultInstance) {
    defaultInstance = loadBTDby4();
  }
  return defaultInstance;
}
