package btdby4

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

// CountImageSize returns visual tokens for a W×H image (1568px fit, 28px tiles).
func CountImageSize(width, height int) int {
	w, h := resizeFit(width, height, 1568, 1568)
	return ceilDiv(w, 28) * ceilDiv(h, 28)
}

// CountImageBytes decodes image bytes (PNG/JPEG/GIF) and counts visual tokens; 0 if undecodable.
func CountImageBytes(b []byte) int {
	w, h, ok := dimsOfBytes(b)
	if !ok {
		return 0
	}
	return CountImageSize(w, h)
}

// CountImageBase64 strips an optional data: prefix, base64-decodes, and counts visual tokens; 0 on error.
func CountImageBase64(s string) int {
	if i := strings.Index(s, ","); i >= 0 {
		s = s[i+1:]
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return CountImageBytes(b)
}

func dimsOfBytes(b []byte) (int, int, bool) {
	if len(b) == 0 {
		return 0, 0, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

func visualTokens(w, h int) int { return ceilDiv(w, 28) * ceilDiv(h, 28) }

func fitsResize(w, h, maxEdge, maxTokens int) bool {
	return ceilDiv(w, 28)*28 <= maxEdge &&
		ceilDiv(h, 28)*28 <= maxEdge &&
		visualTokens(w, h) <= maxTokens
}

func resizeFit(w, h, maxEdge, maxTokens int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	if fitsResize(w, h, maxEdge, maxTokens) {
		return w, h
	}
	if h > w {
		rw, rh := resizeFit(h, w, maxEdge, maxTokens)
		return rh, rw
	}
	aspect := float64(w) / float64(h)
	lo, hi := 1, w
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if fitsResize(mid, imax(int(math.Round(float64(mid)/aspect)), 1), maxEdge, maxTokens) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo, imax(int(math.Round(float64(lo)/aspect)), 1)
}

func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ceilDiv(a, b int) int { return (a + b - 1) / b }
