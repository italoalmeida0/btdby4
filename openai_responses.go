package btdby4

import (
	json "github.com/goccy/go-json"

	"github.com/italoalmeida0/btdby4/codec"
)

type ResponsesRequest struct {
	Instructions any             `json:"instructions,omitempty"`
	Input        any             `json:"input,omitempty"`
	Tools        []ResponsesTool `json:"tools,omitempty"`
}

type ResponsesTool struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
	Strict      bool   `json:"strict,omitempty"`
}

// CountResponsesRequest counts a full Responses request.
func CountResponsesRequest(req ResponsesRequest, opts Options) (Breakdown, error) {
	var out Breakdown
	var think thinkingAccumulator
	if req.Instructions != nil {
		n, err := countResponsesText(req.Instructions, opts)
		if err != nil {
			return out, err
		}
		out.System = n
	}
	items, ok := req.Input.([]any)
	if !ok {
		if req.Input != nil {
			n, err := countResponsesText(req.Input, opts)
			if err != nil {
				return out, err
			}
			out.Messages = n
			out.ByMessage = []int{n}
		}
		return finishResponses(&out, req, opts)
	}
	out.ByMessage = make([]int, len(items))
	for i, raw := range items {
		it, ok := raw.(map[string]any)
		if !ok {
			n, err := codec.FastCountNoErr(jsonString(raw))
			if err != nil {
				return out, err
			}
			out.ByMessage[i] = n
			out.Messages += n
			continue
		}
		n, imgs, cnt, th := countResponsesItemAcc(it, opts, &think)
		out.ByMessage[i] = n
		out.Messages += n
		out.Images += imgs
		out.ImageCount += cnt
		_ = th
	}
	applyThinking(&out, req, opts, &think)
	return finishResponses(&out, req, opts)
}

// CountResponsesRequestJSON counts a full request from raw JSON.
// The request is decoded once and then counted + linearized in a
// single walk (kvCountResponsesRequest) shared with the KV cache path.
func CountResponsesRequestJSON(raw []byte, opts Options) (Breakdown, error) {
	var req ResponsesRequest
	if err := json.UnmarshalNoEscape(raw, &req); err != nil {
		return Breakdown{}, err
	}
	bd, _, err := kvCountResponsesRequest(req, opts)
	return bd, err
}

// CountResponsesItem counts a single input item.
func CountResponsesItem(item map[string]any, opts Options) (BlockBreakdown, error) {
	var out BlockBreakdown
	tokens, images, cnt := countResponsesItem(item, opts)
	out.Tokens = tokens
	out.Images = images
	out.TextTokens = tokens - images
	out.ImageCount = cnt
	return out, nil
}

// CountResponsesItemJSON counts a single input item from raw JSON.
func CountResponsesItemJSON(raw []byte, opts Options) (BlockBreakdown, error) {
	var item map[string]any
	if err := json.UnmarshalNoEscape(raw, &item); err != nil {
		return BlockBreakdown{}, err
	}
	return CountResponsesItem(item, opts)
}

// CountResponsesPart counts a single content part.
func CountResponsesPart(part map[string]any, opts Options) (BlockBreakdown, error) {
	var out BlockBreakdown
	tokens, images, cnt := countResponsesPart(part, opts)
	out.Tokens = tokens
	out.Images = images
	out.TextTokens = tokens - images
	out.ImageCount = cnt
	return out, nil
}

// CountResponsesPartJSON counts a single content part from raw JSON.
func CountResponsesPartJSON(raw []byte, opts Options) (BlockBreakdown, error) {
	var part map[string]any
	if err := json.UnmarshalNoEscape(raw, &part); err != nil {
		return BlockBreakdown{}, err
	}
	return CountResponsesPart(part, opts)
}

// CountResponsesTool counts a tool definition.
func CountResponsesTool(tool ResponsesTool) (int, error) {
	return countResponsesTool(tool)
}

// CountResponsesToolJSON counts a tool definition from raw JSON.
func CountResponsesToolJSON(raw []byte) (int, error) {
	var tool ResponsesTool
	if err := json.UnmarshalNoEscape(raw, &tool); err != nil {
		return 0, err
	}
	return countResponsesTool(tool)
}

func finishResponses(out *Breakdown, req ResponsesRequest, opts Options) (Breakdown, error) {
	out.ByTool = make([]int, 0, len(req.Tools))
	for _, t := range req.Tools {
		n, err := countResponsesTool(t)
		if err != nil {
			return *out, err
		}
		out.ByTool = append(out.ByTool, n)
		out.Tools += n
	}
	out.TextTokens = out.System + out.Messages + out.Tools - out.Images
	out.Total = out.System + out.Messages + out.Tools
	applySafetyMargin(out, opts, len(out.ByMessage), len(req.Tools), out.ImageCount)
	return *out, nil
}

func countResponsesText(v any, opts Options) (int, error) {
	switch t := v.(type) {
	case nil:
		return 0, nil
	case string:
		return codec.FastCountNoErr(t)
	case []any:
		total := 0
		for _, p := range t {
			n, err := countResponsesText(p, opts)
			if err != nil {
				return 0, err
			}
			total += n
		}
		return total, nil
	default:
		if m, ok := v.(map[string]any); ok {
			if s, ok := m["text"].(string); ok {
				return codec.FastCountNoErr(s)
			}
		}
		return codec.FastCountNoErr(jsonString(v))
	}
}

func countResponsesItem(it map[string]any, opts Options) (tokens, images, imgCount int) {
	n, i, c, _ := countResponsesItemAcc(it, opts, nil)
	return n, i, c
}

func countResponsesItemAcc(it map[string]any, opts Options, acc *thinkingAccumulator) (tokens, images, imgCount, th int) {
	typ, _ := it["type"].(string)
	switch typ {
	case "message":
		n, imgs, cnt := countResponsesContent(asAnyArr(it["content"]), opts)
		return n, imgs, cnt, 0
	case "function_call":
		total := 0
		n, _ := codec.FastCountNoErr(strField(it, "name"))
		total += n
		n, _ = codec.FastCountNoErr(strField(it, "arguments"))
		total += n
		return total, 0, 0, 0
	case "function_call_output":
		n, _ := codec.FastCountNoErr(callOutputString(it["output"]))
		return n, 0, 0, 0
	case "reasoning":
		n, i, c, th := countResponsesReasoningAcc(it, opts, acc)
		return n, i, c, th
	default:
		n, _ := codec.FastCountNoErr(jsonString(it))
		return n, 0, 0, 0
	}
}

func countResponsesContent(parts []any, opts Options) (tokens, images, imgCount int) {
	for _, raw := range parts {
		n, it, c := countResponsesPartAny(raw, opts)
		tokens += n
		images += it
		imgCount += c
	}
	return tokens, images, imgCount
}

func countResponsesPart(part map[string]any, opts Options) (tokens, images, imgCount int) {
	pt, _ := part["type"].(string)
	switch pt {
	case "input_text", "output_text", "text":
		n, _ := codec.FastCountNoErr(strField(part, "text"))
		return n, 0, 0
	case "reasoning_text", "reasoning_content":
		n, _ := codec.FastCountNoErr(strField(part, "text"))
		return n, 0, 0
	case "input_image":
		if opts.IgnoreImages {
			return 0, 0, 0
		}
		var url string
		url, _ = part["image_url"].(string)
		if url == "" {
			url, _ = part["file_id"].(string)
		}
		it := countChatImageURL(url)
		return it, it, 1
	case "summary_text":
		n, _ := codec.FastCountNoErr(strField(part, "text"))
		return n, 0, 0
	default:
		n, _ := codec.FastCountNoErr(jsonString(part))
		return n, 0, 0
	}
}

func countResponsesPartAny(raw any, opts Options) (tokens, images, imgCount int) {
	p, ok := raw.(map[string]any)
	if !ok {
		n, _ := codec.FastCountNoErr(jsonString(raw))
		return n, 0, 0
	}
	return countResponsesPart(p, opts)
}

func countResponsesReasoning(it map[string]any, opts Options) (tokens, images, imgCount int) {
	n, i, c, _ := countResponsesReasoningAcc(it, opts, nil)
	return n, i, c
}

func countResponsesReasoningAcc(it map[string]any, opts Options, acc *thinkingAccumulator) (tokens, images, imgCount, th int) {
	for _, raw := range asAnyArr(it["summary"]) {
		if p, ok := raw.(map[string]any); ok {
			n, _ := codec.FastCountNoErr(strField(p, "text"))
			tokens += n
		}
	}
	if enc, ok := it["encrypted_content"].(string); ok && enc != "" {
		if acc != nil {
			acc.add(enc)
		}
		th = singleEnvelopeEstimate(enc)
		tokens += th
	}
	return tokens, 0, 0, th
}

func applyThinking(out *Breakdown, req ResponsesRequest, opts Options, acc *thinkingAccumulator) {
	if acc == nil || (acc.fernetN == 0 && acc.other == 0) {
		return
	}
	want := acc.estimate()
	got := 0
	items, _ := req.Input.([]any)
	for _, raw := range items {
		it, ok := raw.(map[string]any)
		if !ok || it["type"] != "reasoning" {
			continue
		}
		if enc, ok := it["encrypted_content"].(string); ok && enc != "" {
			got += singleEnvelopeEstimate(enc)
		}
	}
	diff := want - got
	if diff == 0 {
		return
	}
	out.Messages += diff
	for i, raw := range items {
		it, ok := raw.(map[string]any)
		if !ok || it["type"] != "reasoning" {
			continue
		}
		if enc, ok := it["encrypted_content"].(string); ok && enc != "" {
			out.ByMessage[i] += diff
			break
		}
	}
}

func countResponsesTool(t ResponsesTool) (int, error) {
	if t.Type != "function" {
		return codec.FastCountNoErr(jsonString(t))
	}
	total := 0
	n, err := codec.FastCountNoErr(t.Name)
	if err != nil {
		return 0, err
	}
	total += n
	if t.Description != "" {
		n, err := codec.FastCountNoErr(t.Description)
		if err != nil {
			return 0, err
		}
		total += n
	}
	if t.Parameters != nil {
		n, err := codec.FastCountNoErr(jsonString(t.Parameters))
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func callOutputString(output any) string {
	if s, ok := output.(string); ok {
		return s
	}
	parts := []string{}
	for _, raw := range asAnyArr(output) {
		if p, ok := raw.(map[string]any); ok {
			pt, _ := p["type"].(string)
			if (pt == "input_text" || pt == "output_text") {
				if s, ok := p["text"].(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	out := ""
	for i, s := range parts {
		if i > 0 {
			out += "\n"
		}
		out += s
	}
	return out
}

func asAnyArr(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	return nil
}
