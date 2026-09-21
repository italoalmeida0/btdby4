if (typeof globalThis !== "undefined" && !globalThis.crypto) {
  try {
    const nodeCrypto = await import("node:crypto");
    globalThis.crypto = nodeCrypto.webcrypto || nodeCrypto.default?.webcrypto || nodeCrypto.default;
  } catch {}
}
