package btdby4

// EstimateThinkingTokens estimates billed thinking tokens hidden behind
// an opaque encrypted payload (redacted_thinking, encrypted_content,
// thought_signature). Provider-agnostic: unwraps the base64 envelope to
// raw bytes and maps per envelope family. Empty input returns 0.
func EstimateThinkingTokens(enc string) int {
	var acc thinkingAccumulator
	acc.add(enc)
	return acc.estimate()
}

type thinkingAccumulator struct {
	fernetRaw float64
	fernetN   int
	other     int
}

func (a *thinkingAccumulator) add(enc string) {
	raw := envelopeRawBytes(enc)
	if raw <= 0 {
		return
	}
	switch {
	case isCompactSignature(enc):
		a.other += singleEnvelopeEstimate(enc)
	case len(enc) >= jumboEnvelopeChars:
		a.other += singleEnvelopeEstimate(enc)
	default:
		a.fernetRaw += raw
		a.fernetN++
	}
}

func (a *thinkingAccumulator) estimate() int {
	total := float64(a.other)
	if a.fernetN > 0 {
		total += (a.fernetRaw - fernetOverhead) / fernetSlope
	}
	if total < 0 {
		return 0
	}
	return int(total + 0.5)
}

const (
	fernetOverhead = 752.0
	fernetSlope    = 4.75
	sigOverhead      = 150.0
	sigSlope         = 2.76
	jumboEnvelopeChars = 8000
	jumboBase          = 966.0
	jumboSlope         = 16.9
)

func singleEnvelopeEstimate(enc string) int {
	raw := envelopeRawBytes(enc)
	if raw <= 0 {
		return 0
	}
	var n float64
	switch {
	case isCompactSignature(enc):
		n = (raw - sigOverhead) / sigSlope
	case len(enc) >= jumboEnvelopeChars:
		n = jumboBase + raw/jumboSlope
	default:
		n = (raw - fernetOverhead) / fernetSlope
	}
	if n < 0 {
		return 0
	}
	return int(n + 0.5)
}

func envelopeRawBytes(enc string) float64 {
	n := 0
	for i := 0; i < len(enc); i++ {
		c := enc[i]
		if c == '=' {
			break
		}
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' ||
			c >= '0' && c <= '9' || c == '+' || c == '/' ||
			c == '-' || c == '_' {
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(n) * 3.0 / 4.0
}

func isCompactSignature(enc string) bool {
	for i := 0; i < len(enc); i++ {
		if enc[i] == '+' || enc[i] == '/' {
			return true
		}
	}
	return false
}

// Options tunes counting. IgnoreImages counts images as zero.
type Options struct {
	IgnoreImages bool
}

func estimateImageURLTokens() int {
	return 0
}
