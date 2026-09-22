package btdby4

import (
	json "github.com/goccy/go-json"
	"runtime"
	"sync"

	"github.com/italoalmeida0/btdby4/codec"
)

type ChatMessage struct {
	Role       string `json:"role"`
	Content    any    `json:"content,omitempty"`
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Name       string `json:"name,omitempty"`
}

type ChatToolCall struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function ChatFunction `json:"function"`
}

type ChatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type ChatTool struct {
	Type     string `json:"type,omitempty"`
	Function ChatFunctionDef `json:"function"`
}

type ChatFunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
	Strict      bool   `json:"strict,omitempty"`
}

type ChatRequest struct {
	System   any           `json:"system,omitempty"`
	Messages []ChatMessage `json:"messages"`
	Tools    []ChatTool    `json:"tools,omitempty"`
}

// CountChatRequest counts a full Chat Completions request.
// NOTE: chat hot path is fused in kvCountChatRequest (single walk);
// this stays for struct-based callers.
func CountChatRequest(req ChatRequest, opts Options) (Breakdown, error) {
	var out Breakdown
	if req.System != nil {
		n, err := countTextValue(req.System)
		if err != nil {
			return out, err
		}
		out.System = n
	}
	out.ByMessage = make([]int, len(req.Messages))
	if len(req.Messages) >= 32 {
		countChatMessagesParallel(req.Messages, opts, out.ByMessage, &out.Messages, &out.Images, &out.ImageCount)
	} else {
		for i, m := range req.Messages {
			n, imgs, err := countChatMessage(m, opts)
			if err != nil {
				return out, err
			}
			out.ByMessage[i] = n
			out.Messages += n
			out.Images += imgs
			out.ImageCount += chatImagesIn(m.Content)
		}
	}
	out.ByTool = make([]int, 0, len(req.Tools))
	for _, t := range req.Tools {
		n, err := countChatTool(t)
		if err != nil {
			return out, err
		}
		out.ByTool = append(out.ByTool, n)
		out.Tools += n
	}
	out.TextTokens = out.System + out.Messages + out.Tools - out.Images
	out.Total = out.System + out.Messages + out.Tools
	applySafetyMargin(&out, opts, len(req.Messages), len(req.Tools), out.ImageCount)
	return out, nil
}

// CountChatRequestJSON counts a full request from raw JSON.
// The request is decoded once and then counted + linearized in a
// single walk (kvCountChatRequest) shared with the KV cache path:
// KvLookup reuses this so a gateway call pays one parse, not two.
func CountChatRequestJSON(raw []byte, opts Options) (Breakdown, error) {
	var req ChatRequest
	if err := json.UnmarshalNoEscape(raw, &req); err != nil {
		return Breakdown{}, err
	}
	bd, _, err := kvCountChatRequest(req, opts)
	return bd, err
}

// CountChatMessage counts a single message.
func CountChatMessage(msg ChatMessage, opts Options) (BlockBreakdown, error) {
	var out BlockBreakdown
	tokens, images, err := countChatMessage(msg, opts)
	if err != nil {
		return out, err
	}
	out.Tokens = tokens
	out.Images = images
	out.TextTokens = tokens - images
	out.ImageCount = chatImagesIn(msg.Content)
	return out, nil
}

// CountChatMessageJSON counts a single message from raw JSON.
func CountChatMessageJSON(raw []byte, opts Options) (BlockBreakdown, error) {
	var msg ChatMessage
	if err := json.UnmarshalNoEscape(raw, &msg); err != nil {
		return BlockBreakdown{}, err
	}
	return CountChatMessage(msg, opts)
}

// CountChatPart counts a single content part.
func CountChatPart(part map[string]any, opts Options) (BlockBreakdown, error) {
	var out BlockBreakdown
	tokens, images, err := countChatPart(part, opts)
	if err != nil {
		return out, err
	}
	out.Tokens = tokens
	out.Images = images
	out.TextTokens = tokens - images
	if part["type"] == "image_url" && !opts.IgnoreImages {
		out.ImageCount = 1
	}
	return out, nil
}

// CountChatPartJSON counts a single content part from raw JSON.
func CountChatPartJSON(raw []byte, opts Options) (BlockBreakdown, error) {
	var part map[string]any
	if err := json.UnmarshalNoEscape(raw, &part); err != nil {
		return BlockBreakdown{}, err
	}
	return CountChatPart(part, opts)
}

// CountChatTool counts a tool definition.
func CountChatTool(tool ChatTool) (int, error) {
	return countChatTool(tool)
}

// CountChatToolJSON counts a tool definition from raw JSON.
func CountChatToolJSON(raw []byte) (int, error) {
	var tool ChatTool
	if err := json.UnmarshalNoEscape(raw, &tool); err != nil {
		return 0, err
	}
	return countChatTool(tool)
}

func countChatMessagesParallel(msgs []ChatMessage, opts Options, byMsg []int, totalMsgs, totalImgs, totalImgCount *int) {
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	if workers > len(msgs) {
		workers = len(msgs)
	}
	chunk := (len(msgs) + workers - 1) / workers
	msgSums := make([]int, workers)
	imgSums := make([]int, workers)
	cntSums := make([]int, workers)
	for w := 0; w < workers; w++ {
		start := w * chunk
		end := start + chunk
		if end > len(msgs) {
			end = len(msgs)
		}
		if start >= end {
			break
		}
		wg.Add(1)
		go func(w, start, end int) {
			defer wg.Done()
			ms, is, cs := 0, 0, 0
			for i := start; i < end; i++ {
				n, imgs, err := countChatMessage(msgs[i], opts)
				if err != nil {
					continue
				}
				byMsg[i] = n
				ms += n
				is += imgs
				cs += chatImagesIn(msgs[i].Content)
			}
			msgSums[w] = ms
			imgSums[w] = is
			cntSums[w] = cs
		}(w, start, end)
	}
	wg.Wait()
	for w := range msgSums {
		*totalMsgs += msgSums[w]
		*totalImgs += imgSums[w]
		*totalImgCount += cntSums[w]
	}
}

func countChatMessage(m ChatMessage, opts Options) (tokens, images int, err error) {
	if m.ToolCallID != "" {
		switch c := m.Content.(type) {
		case nil:
			return 0, 0, nil
		case string:
			n, err := codec.FastCountNoErr(c)
			return n, 0, err
		default:
			n, err := codec.FastCountNoErr(jsonString(c))
			return n, 0, err
		}
	}
	switch c := m.Content.(type) {
	case nil:
		return countChatToolCalls(m.ToolCalls, opts, new(int)), 0, nil
	case string:
		n, err := codec.FastCountNoErr(c)
		if err != nil {
			return 0, 0, err
		}
		var img int
		n += countChatToolCalls(m.ToolCalls, opts, &img)
		return n, img, nil
	case []any:
		total := 0
		imgToks := 0
		for _, b := range c {
			bm, ok := b.(map[string]any)
			if !ok {
				n, err := codec.FastCountNoErr(jsonString(b))
				if err != nil {
					return 0, 0, err
				}
				total += n
				continue
			}
			n, it, err := countChatPart(bm, opts)
			if err != nil {
				return 0, 0, err
			}
			total += n
			imgToks += it
		}
		total += countChatToolCalls(m.ToolCalls, opts, &imgToks)
		return total, imgToks, nil
	default:
		n, err := codec.FastCountNoErr(jsonString(c))
		return n, 0, err
	}
}

func countChatToolCalls(calls []ChatToolCall, opts Options, imgToks *int) int {
	total := 0
	for _, tc := range calls {
		n, _ := codec.FastCountNoErr(tc.Function.Name)
		total += n
		n, _ = codec.FastCountNoErr(tc.Function.Arguments)
		total += n
	}
	return total
}

func countChatPart(bm map[string]any, opts Options) (tokens, images int, err error) {
	typ, _ := bm["type"].(string)
	switch typ {
	case "text":
		n, err := codec.FastCountNoErr(strField(bm, "text"))
		return n, 0, err
	case "reasoning_content":
		n, err := codec.FastCountNoErr(strField(bm, "text"))
		return n, 0, err
	case "image_url":
		if opts.IgnoreImages {
			return 0, 0, nil
		}
		var url string
		if inner, ok := bm["image_url"].(map[string]any); ok {
			url, _ = inner["url"].(string)
		} else {
			url, _ = bm["url"].(string)
		}
		it := countChatImageURL(url)
		return it, it, nil
	case "input_audio":
		return countChatAudio(bm, opts)
	default:
		n, err := codec.FastCountNoErr(jsonString(bm))
		return n, 0, err
	}
}

func countChatImageURL(url string) int {
	if url == "" {
		return 0
	}
	if idx := indexOf(url, ";base64,"); idx >= 0 {
		return CountImageBase64(url[idx+8:])
	}
	return estimateImageURLTokens()
}

func countChatAudio(bm map[string]any, opts Options) (int, int, error) {
	if inner, ok := bm["input_audio"].(map[string]any); ok {
		if d, ok := inner["data"].(string); ok && d != "" {
			n, err := codec.FastCountNoErr(d)
			return n, 0, err
		}
	}
	n, err := codec.FastCountNoErr(jsonString(bm))
	return n, 0, err
}

func countChatTool(t ChatTool) (int, error) {
	total := 0
	n, err := codec.FastCountNoErr(t.Function.Name)
	if err != nil {
		return 0, err
	}
	total += n
	if t.Function.Description != "" {
		n, err := codec.FastCountNoErr(t.Function.Description)
		if err != nil {
			return 0, err
		}
		total += n
	}
	if t.Function.Parameters != nil {
		n, err := codec.FastCountNoErr(jsonString(t.Function.Parameters))
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func chatImagesIn(content any) int {
	arr, ok := content.([]any)
	if !ok {
		return 0
	}
	n := 0
	for _, b := range arr {
		if bm, ok := b.(map[string]any); ok && bm["type"] == "image_url" {
			n++
		}
	}
	return n
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
