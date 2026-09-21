package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"unsafe"

	"github.com/italoalmeida0/btdby4"
)

//export CountText
func CountText(str *C.char) C.int {
	if str == nil {
		return 0
	}
	goStr := C.GoString(str)
	n, _ := btdby4.CountText(goStr)
	return C.int(n)
}

//export EstimateThinkingTokens
func EstimateThinkingTokens(str *C.char) C.int {
	if str == nil {
		return 0
	}
	goStr := C.GoString(str)
	n := btdby4.EstimateThinkingTokens(goStr)
	return C.int(n)
}

//export CountImageSize
func CountImageSize(width, height C.int) C.int {
	return C.int(btdby4.CountImageSize(int(width), int(height)))
}

//export CountImageBase64
func CountImageBase64(str *C.char) C.int {
	if str == nil {
		return 0
	}
	goStr := C.GoString(str)
	return C.int(btdby4.CountImageBase64(goStr))
}

//export CountImageBytes
func CountImageBytes(buf *C.char, length C.int) C.int {
	if buf == nil || length <= 0 {
		return 0
	}
	bytes := C.GoBytes(unsafe.Pointer(buf), length)
	return C.int(btdby4.CountImageBytes(bytes))
}

func marshalBreakdown(b btdby4.Breakdown, err error) *C.char {
	if err != nil {
		errObj, _ := json.Marshal(map[string]string{"error": err.Error()})
		return C.CString(string(errObj))
	}
	res, _ := json.Marshal(b)
	return C.CString(string(res))
}

func marshalBlockBreakdown(b btdby4.BlockBreakdown, err error) *C.char {
	if err != nil {
		errObj, _ := json.Marshal(map[string]string{"error": err.Error()})
		return C.CString(string(errObj))
	}
	res, _ := json.Marshal(b)
	return C.CString(string(res))
}

// --- OpenAI Chat ---

//export CountChatRequestJSON
func CountChatRequestJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountChatRequestJSON(goBytes, opts)
	return marshalBreakdown(b, err)
}

//export CountChatTotalQuick
func CountChatTotalQuick(jsonStr *C.char, tight C.int, ignoreImages C.int) C.int {
	if jsonStr == nil {
		return 0
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountChatRequestJSON(goBytes, opts)
	if err != nil {
		return -1
	}
	return C.int(b.Total)
}

//export CountChatMessageJSON
func CountChatMessageJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountChatMessageJSON(goBytes, opts)
	return marshalBlockBreakdown(b, err)
}

//export CountChatPartJSON
func CountChatPartJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountChatPartJSON(goBytes, opts)
	return marshalBlockBreakdown(b, err)
}

//export CountChatToolJSON
func CountChatToolJSON(jsonStr *C.char) C.int {
	if jsonStr == nil {
		return 0
	}
	goBytes := []byte(C.GoString(jsonStr))
	n, err := btdby4.CountChatToolJSON(goBytes)
	if err != nil {
		return -1
	}
	return C.int(n)
}

// --- Anthropic Messages ---

//export CountAnthropicRequestJSON
func CountAnthropicRequestJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountAnthropicRequestJSON(goBytes, opts)
	return marshalBreakdown(b, err)
}

//export CountAnthropicTotalQuick
func CountAnthropicTotalQuick(jsonStr *C.char, tight C.int, ignoreImages C.int) C.int {
	if jsonStr == nil {
		return 0
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountAnthropicRequestJSON(goBytes, opts)
	if err != nil {
		return -1
	}
	return C.int(b.Total)
}

//export CountAnthropicMessageJSON
func CountAnthropicMessageJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountAnthropicMessageJSON(goBytes, opts)
	return marshalBlockBreakdown(b, err)
}

//export CountAnthropicBlockJSON
func CountAnthropicBlockJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountAnthropicBlockJSON(goBytes, opts)
	return marshalBlockBreakdown(b, err)
}

//export CountAnthropicToolJSON
func CountAnthropicToolJSON(jsonStr *C.char) C.int {
	if jsonStr == nil {
		return 0
	}
	goBytes := []byte(C.GoString(jsonStr))
	n, err := btdby4.CountAnthropicToolJSON(goBytes)
	if err != nil {
		return -1
	}
	return C.int(n)
}

// --- OpenAI Responses ---

//export CountResponsesRequestJSON
func CountResponsesRequestJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountResponsesRequestJSON(goBytes, opts)
	return marshalBreakdown(b, err)
}

//export CountResponsesTotalQuick
func CountResponsesTotalQuick(jsonStr *C.char, tight C.int, ignoreImages C.int) C.int {
	if jsonStr == nil {
		return 0
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountResponsesRequestJSON(goBytes, opts)
	if err != nil {
		return -1
	}
	return C.int(b.Total)
}

//export CountResponsesItemJSON
func CountResponsesItemJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountResponsesItemJSON(goBytes, opts)
	return marshalBlockBreakdown(b, err)
}

//export CountResponsesPartJSON
func CountResponsesPartJSON(jsonStr *C.char, tight C.int, ignoreImages C.int) *C.char {
	if jsonStr == nil {
		return nil
	}
	goBytes := []byte(C.GoString(jsonStr))
	opts := btdby4.Options{Tight: tight != 0, IgnoreImages: ignoreImages != 0}
	b, err := btdby4.CountResponsesPartJSON(goBytes, opts)
	return marshalBlockBreakdown(b, err)
}

//export CountResponsesToolJSON
func CountResponsesToolJSON(jsonStr *C.char) C.int {
	if jsonStr == nil {
		return 0
	}
	goBytes := []byte(C.GoString(jsonStr))
	n, err := btdby4.CountResponsesToolJSON(goBytes)
	if err != nil {
		return -1
	}
	return C.int(n)
}

//export FreeCString
func FreeCString(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}

func main() {}
