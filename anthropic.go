package btdby4

import (
	json "github.com/goccy/go-json"
	"runtime"
	"sync"

	"github.com/italoalmeida0/btdby4/codec"
)

type AnthropicRequest struct {
	System   any               `json:"system,omitempty"`
	Messages []AnthropicMessage `json:"messages"`
	Tools    []AnthropicTool    `json:"tools,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

type Breakdown struct {
	System     int   `json:"system"`
	Messages   int   `json:"messages"`
	Tools      int   `json:"tools"`
	Images     int   `json:"images"`
	ByMessage  []int `json:"by_message,omitempty"`
	ByTool     []int `json:"by_tool,omitempty"`
	TextTokens int   `json:"text_tokens"`
	Total      int   `json:"total"`
	ImageCount int   `json:"image_count"`
}

type BlockBreakdown struct {
	Tokens     int `json:"tokens"`
	Images     int `json:"images"`
	TextTokens int `json:"text_tokens"`
	ImageCount int `json:"image_count"`
}

// CountAnthropicRequest counts a full Anthropic Messages request.
func CountAnthropicRequest(req AnthropicRequest, opts Options) (Breakdown, error) {
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
		countMessagesParallel(req.Messages, opts, out.ByMessage, &out.Messages, &out.Images, &out.ImageCount)
	} else {
		for i, m := range req.Messages {
			n, imgs, err := countMessage(m, opts)
			if err != nil {
				return out, err
			}
			out.ByMessage[i] = n
			out.Messages += n
			out.Images += imgs
			out.ImageCount += imageBlocksIn(m.Content)
		}
	}

	out.ByTool = make([]int, 0, len(req.Tools))
	for _, t := range req.Tools {
		n, err := countTool(t)
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

func countMessagesParallel(msgs []AnthropicMessage, opts Options, byMsg []int, totalMsgs, totalImgs, totalImgCount *int) {
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
				n, imgs, err := countMessage(msgs[i], opts)
				if err != nil {
					continue
				}
				byMsg[i] = n
				ms += n
				is += imgs
				cs += imageBlocksIn(msgs[i].Content)
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

// CountAnthropicRequestJSON counts a full request from raw JSON.
// The request is decoded once and then counted + linearized in a
// single walk (kvCountAnthropicRequest) shared with the KV cache path.
func CountAnthropicRequestJSON(raw []byte, opts Options) (Breakdown, error) {
	var req AnthropicRequest
	if err := json.UnmarshalNoEscape(raw, &req); err != nil {
		return Breakdown{}, err
	}
	bd, _, err := kvCountAnthropicRequest(req, opts)
	return bd, err
}

// CountAnthropicBlock counts a single content block.
func CountAnthropicBlock(block map[string]any, opts Options) (BlockBreakdown, error) {
	var out BlockBreakdown
	tokens, images, err := countContentBlock(block, opts)
	if err != nil {
		return out, err
	}
	out.Tokens = tokens
	out.Images = images
	out.TextTokens = tokens - images
	out.ImageCount = imageBlocksInBlock(block)
	return out, nil
}

// CountAnthropicBlockJSON counts a single content block from raw JSON.
func CountAnthropicBlockJSON(raw []byte, opts Options) (BlockBreakdown, error) {
	var block map[string]any
	if err := json.UnmarshalNoEscape(raw, &block); err != nil {
		return BlockBreakdown{}, err
	}
	return CountAnthropicBlock(block, opts)
}

// CountAnthropicMessage counts a single message.
func CountAnthropicMessage(msg AnthropicMessage, opts Options) (BlockBreakdown, error) {
	var out BlockBreakdown
	tokens, images, err := countMessage(msg, opts)
	if err != nil {
		return out, err
	}
	out.Tokens = tokens
	out.Images = images
	out.TextTokens = tokens - images
	out.ImageCount = imageBlocksIn(msg.Content)
	return out, nil
}

// CountAnthropicMessageJSON counts a single message from raw JSON.
func CountAnthropicMessageJSON(raw []byte, opts Options) (BlockBreakdown, error) {
	var msg AnthropicMessage
	if err := json.UnmarshalNoEscape(raw, &msg); err != nil {
		return BlockBreakdown{}, err
	}
	return CountAnthropicMessage(msg, opts)
}

// CountAnthropicTool counts a tool definition.
func CountAnthropicTool(tool AnthropicTool) (int, error) {
	return countTool(tool)
}

// CountAnthropicToolJSON counts a tool definition from raw JSON.
func CountAnthropicToolJSON(raw []byte) (int, error) {
	var tool AnthropicTool
	if err := json.UnmarshalNoEscape(raw, &tool); err != nil {
		return 0, err
	}
	return countTool(tool)
}

func countMessage(m AnthropicMessage, opts Options) (tokens, images int, err error) {
	switch c := m.Content.(type) {
	case nil:
		return 0, 0, nil
	case string:
		n, err := codec.FastCountNoErr(c)
		return n, 0, err
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
			n, it, err := countContentBlock(bm, opts)
			if err != nil {
				return 0, 0, err
			}
			total += n
			imgToks += it
		}
		return total, imgToks, nil
	default:
		n, err := codec.FastCountNoErr(jsonString(c))
		return n, 0, err
	}
}

func countContentBlock(bm map[string]any, opts Options) (tokens, images int, err error) {
	typ, _ := bm["type"].(string)
	switch typ {
	case "text":
		n, err := codec.FastCountNoErr(strField(bm, "text"))
		return n, 0, err
	case "image":
		if opts.IgnoreImages {
			return 0, 0, nil
		}
		it := countImageSource(bm["source"])
		return it, it, nil
	case "tool_use":
		total := 0
		n, err := codec.FastCountNoErr(strField(bm, "name"))
		if err != nil {
			return 0, 0, err
		}
		total += n
		if in, ok := bm["input"]; ok {
			n, err := codec.FastCountNoErr(jsonString(in))
			if err != nil {
				return 0, 0, err
			}
			total += n
		}
		return total, 0, nil
	case "tool_result":
		return countToolResult(bm["content"], opts)
	case "thinking":
		n, err := codec.FastCountNoErr(strField(bm, "thinking"))
		return n, 0, err
	case "redacted_thinking":
		if d, ok := bm["data"].(string); ok && d != "" {
			return EstimateThinkingTokens(d), 0, nil
		}
		return 0, 0, nil
	default:
		n, err := codec.FastCountNoErr(jsonString(bm))
		return n, 0, err
	}
}

func countToolResult(c any, opts Options) (tokens, images int, err error) {
	switch t := c.(type) {
	case nil:
		return 0, 0, nil
	case string:
		n, err := codec.FastCountNoErr(t)
		return n, 0, err
	case []any:
		total, imgToks := 0, 0
		for _, b := range t {
			bm, ok := b.(map[string]any)
			if !ok {
				n, err := codec.FastCountNoErr(jsonString(b))
				if err != nil {
					return 0, 0, err
				}
				total += n
				continue
			}
			n, it, err := countContentBlock(bm, opts)
			if err != nil {
				return 0, 0, err
			}
			total += n
			imgToks += it
		}
		return total, imgToks, nil
	default:
		n, err := codec.FastCountNoErr(jsonString(c))
		return n, 0, err
	}
}

func countTool(t AnthropicTool) (int, error) {
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
	if t.InputSchema != nil {
		n, err := codec.FastCountNoErr(jsonString(t.InputSchema))
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func countTextValue(v any) (int, error) {
	switch t := v.(type) {
	case string:
		return codec.FastCountNoErr(t)
	case []any:
		total := 0
		for _, b := range t {
			if bm, ok := b.(map[string]any); ok && bm["type"] == "text" {
				n, err := codec.FastCountNoErr(strField(bm, "text"))
				if err != nil {
					return 0, err
				}
				total += n
			} else {
				n, err := codec.FastCountNoErr(jsonString(b))
				if err != nil {
					return 0, err
				}
				total += n
			}
		}
		return total, nil
	default:
		return codec.FastCountNoErr(jsonString(v))
	}
}

func countImageSource(src any) int {
	sm, ok := src.(map[string]any)
	if !ok {
		return 0
	}
	if d, ok := sm["data"].(string); ok && d != "" {
		return CountImageBase64(d)
	}
	if u, ok := sm["url"].(string); ok && u != "" {
		return countChatImageURL(u)
	}
	return 0
}

func imageBlocksIn(content any) int {
	n := 0
	if arr, ok := content.([]any); ok {
		for _, b := range arr {
			if bm, ok := b.(map[string]any); ok {
				if bm["type"] == "image" {
					n++
				} else if bm["type"] == "tool_result" {
					n += imageBlocksInBlock(bm)
				}
			}
		}
	}
	return n
}

func imageBlocksInBlock(bm map[string]any) int {
	if bm["type"] == "image" {
		return 1
	}
	if bm["type"] == "tool_result" {
		if arr, ok := bm["content"].([]any); ok {
			n := 0
			for _, b := range arr {
				if inner, ok := b.(map[string]any); ok && inner["type"] == "image" {
					n++
				}
			}
			return n
		}
	}
	return 0
}

func strField(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
