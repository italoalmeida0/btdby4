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
//
// Total always includes a small generic safety margin on top of the
// content count, so the default estimate lands at or above the
// provider-billed total in the common cases — install-and-use safe for
// budget enforcement and automatic compaction (compacting on Total
// fires before the real context fills up instead of after). TextTokens
// keeps the pure content count; set Tight: true to get the closest
// point estimate in Total as well (display/cost paths).
type Options struct {
	IgnoreImages bool
	// Tight disables the built-in safety margin and returns the
	// closest point estimate. Default (false) keeps the margin.
	Tight bool
}

func estimateImageURLTokens() int {
	return 0
}

// Conservative upper-bound margins, applied by default (see Options).
// They cover provider-side framing that is not part of the request
// content: per-message wire overhead, the tool harness preamble plus
// per-tool registration cost, and the minimum billable cost of one
// image. Values are generic (no per-provider or per-model tables):
// they are sized to cover the worst provider measured (Meta ~500 fixed
// + ~40/tool on chat, Google ~1080 floor on tiny images) while staying
// small relative to long contexts (a few hundred tokens against 200k+).
const (
	conservativePerMessage = 12
	conservativeToolsFixed = 500
	conservativePerTool    = 45
	conservativePerImage   = 1100
)

// applySafetyMargin adds the generic safety margin to a finished
// Breakdown, unless Options.Tight opts out. TextTokens keeps the pure
// content count; Total becomes the upper bound used for budgets and
// compaction.
func applySafetyMargin(out *Breakdown, opts Options, nMessages, nTools, nImages int) {
	if opts.Tight {
		return
	}
	if opts.IgnoreImages {
		nImages = 0
	}
	margin := nMessages * conservativePerMessage
	if nTools > 0 {
		margin += conservativeToolsFixed + nTools*conservativePerTool
	}
	margin += nImages * conservativePerImage
	out.Total += margin
}
