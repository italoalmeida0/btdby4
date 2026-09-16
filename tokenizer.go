package btdby4

import (
	"github.com/italoalmeida0/btdby4/codec"
)

// CountText counts tokens in a plain string.
func CountText(text string) (int, error) {
	return codec.FastCount(text), nil
}
