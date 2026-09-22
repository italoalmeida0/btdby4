package main

import (
	json "github.com/goccy/go-json"
	"syscall/js"

	"github.com/italoalmeida0/btdby4"
)

func countText(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	str := args[0].String()
	n, _ := btdby4.CountText(str)
	return n
}

func estimateThinkingTokens(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	str := args[0].String()
	return btdby4.EstimateThinkingTokens(str)
}

func countImageSize(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return 0
	}
	w := args[0].Int()
	h := args[1].Int()
	return btdby4.CountImageSize(w, h)
}

func countImageBase64(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	str := args[0].String()
	return btdby4.CountImageBase64(str)
}

func countImageBytes(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	src := args[0]
	length := src.Get("byteLength").Int()
	if length <= 0 {
		return 0
	}
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, src)
	return btdby4.CountImageBytes(buf)
}

// --- OpenAI Chat ---

func countChatRequestJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountChatRequestJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countChatTotalQuick(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountChatRequestJSON([]byte(jsonStr), opts)
	if err != nil {
		return -1
	}
	return b.Total
}

func countChatMessageJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountChatMessageJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countChatPartJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountChatPartJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countChatToolJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	jsonStr := args[0].String()
	n, err := btdby4.CountChatToolJSON([]byte(jsonStr))
	if err != nil {
		return -1
	}
	return n
}

// --- Anthropic Messages ---

func countAnthropicRequestJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountAnthropicRequestJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countAnthropicTotalQuick(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountAnthropicRequestJSON([]byte(jsonStr), opts)
	if err != nil {
		return -1
	}
	return b.Total
}

func countAnthropicMessageJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountAnthropicMessageJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countAnthropicBlockJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountAnthropicBlockJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countAnthropicToolJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	jsonStr := args[0].String()
	n, err := btdby4.CountAnthropicToolJSON([]byte(jsonStr))
	if err != nil {
		return -1
	}
	return n
}

// --- OpenAI Responses API ---

func countResponsesRequestJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountResponsesRequestJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countResponsesTotalQuick(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountResponsesRequestJSON([]byte(jsonStr), opts)
	if err != nil {
		return -1
	}
	return b.Total
}

func countResponsesItemJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountResponsesItemJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countResponsesPartJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload"}`
	}
	jsonStr := args[0].String()
	tight := false
	ignoreImages := false
	if len(args) > 1 {
		tight = args[1].Bool()
	}
	if len(args) > 2 {
		ignoreImages = args[2].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	b, err := btdby4.CountResponsesPartJSON([]byte(jsonStr), opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	res, _ := json.Marshal(b)
	return string(res)
}

func countResponsesToolJSON(this js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsNull() || args[0].IsUndefined() {
		return 0
	}
	jsonStr := args[0].String()
	n, err := btdby4.CountResponsesToolJSON([]byte(jsonStr))
	if err != nil {
		return -1
	}
	return n
}

// --- KV-Cache provider (prefix simulation) ---
//
// kvCacheJSON(payload, protocol, namespace, tight, ignoreImages).
func kvCacheJSON(this js.Value, args []js.Value) any {
	if len(args) < 3 || args[0].IsNull() || args[0].IsUndefined() {
		return `{"error":"missing json payload, protocol and namespace"}`
	}
	jsonStr := args[0].String()
	protocol := args[1].String()
	namespace := args[2].String()
	tight := false
	ignoreImages := false
	if len(args) > 3 && !args[3].IsNull() && !args[3].IsUndefined() {
		tight = args[3].Bool()
	}
	if len(args) > 4 && !args[4].IsNull() && !args[4].IsUndefined() {
		ignoreImages = args[4].Bool()
	}
	opts := btdby4.Options{Tight: tight, IgnoreImages: ignoreImages}
	res, err := btdby4.KvLookup([]byte(jsonStr), protocol, namespace, opts)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errBytes)
	}
	out, _ := json.Marshal(res)
	return string(out)
}

func kvStatsJSON(this js.Value, args []js.Value) any {
	res, _ := json.Marshal(btdby4.KvStatsSnapshot())
	return string(res)
}

// kvInit(ttlSeconds, maxMB, separateProtocol?). Defaults 600s / 400MB / true.
// Clears existing entries. Returns the effective config as JSON.
func kvInit(this js.Value, args []js.Value) any {
	var ttl, mb int64
	separate := true
	if len(args) > 0 && !args[0].IsNull() && !args[0].IsUndefined() {
		ttl = int64(args[0].Int())
	}
	if len(args) > 1 && !args[1].IsNull() && !args[1].IsUndefined() {
		mb = int64(args[1].Int())
	}
	if len(args) > 2 && !args[2].IsNull() && !args[2].IsUndefined() {
		separate = args[2].Bool()
	}
	btdby4.KvInit(ttl, mb)
	btdby4.KvSetSeparateProtocol(separate)
	res, _ := json.Marshal(btdby4.KvConfigSnapshot())
	return string(res)
}

func kvClear(this js.Value, args []js.Value) any {
	ns := ""
	if len(args) > 0 && !args[0].IsNull() && !args[0].IsUndefined() {
		ns = args[0].String()
	}
	btdby4.KvClear(ns)
	return true
}

func main() {
	obj := js.Global().Get("Object").New()
	obj.Set("countText", js.FuncOf(countText))
	obj.Set("estimateThinkingTokens", js.FuncOf(estimateThinkingTokens))
	obj.Set("countImageSize", js.FuncOf(countImageSize))
	obj.Set("countImageBase64", js.FuncOf(countImageBase64))
	obj.Set("countImageBytes", js.FuncOf(countImageBytes))

	obj.Set("countChatRequestJSON", js.FuncOf(countChatRequestJSON))
	obj.Set("countChatTotalQuick", js.FuncOf(countChatTotalQuick))
	obj.Set("countChatMessageJSON", js.FuncOf(countChatMessageJSON))
	obj.Set("countChatPartJSON", js.FuncOf(countChatPartJSON))
	obj.Set("countChatToolJSON", js.FuncOf(countChatToolJSON))

	obj.Set("countAnthropicRequestJSON", js.FuncOf(countAnthropicRequestJSON))
	obj.Set("countAnthropicTotalQuick", js.FuncOf(countAnthropicTotalQuick))
	obj.Set("countAnthropicMessageJSON", js.FuncOf(countAnthropicMessageJSON))
	obj.Set("countAnthropicBlockJSON", js.FuncOf(countAnthropicBlockJSON))
	obj.Set("countAnthropicToolJSON", js.FuncOf(countAnthropicToolJSON))

	obj.Set("countResponsesRequestJSON", js.FuncOf(countResponsesRequestJSON))
	obj.Set("countResponsesTotalQuick", js.FuncOf(countResponsesTotalQuick))
	obj.Set("countResponsesItemJSON", js.FuncOf(countResponsesItemJSON))
	obj.Set("countResponsesPartJSON", js.FuncOf(countResponsesPartJSON))
	obj.Set("countResponsesToolJSON", js.FuncOf(countResponsesToolJSON))

	obj.Set("kvCacheJSON", js.FuncOf(kvCacheJSON))
	obj.Set("kvStatsJSON", js.FuncOf(kvStatsJSON))
	obj.Set("kvInit", js.FuncOf(kvInit))
	obj.Set("kvClear", js.FuncOf(kvClear))

	js.Global().Set("__btdby4_wasm_instance", obj)

	select {}
}
