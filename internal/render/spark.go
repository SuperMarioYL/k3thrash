// Package render renders the shareable ASCII report — verdict summary + a
// unicode re-read-rate sparkline — that an operator pastes into an
// r/LocalLLaMA / 掘金 thread. One block, no external rendering deps.
package render

import "strings"

// sparkBlocks maps a normalized [0,1] value to a unicode bar character.
// 8 levels (▁▂▃▄▅▆▇█) gives a smooth-enough line at typical terminal widths.
var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// Sparkline renders a series of floats as a compact unicode sparkline. The
// series is min-max normalized so the shape (not the absolute values) is what
// the eye reads. Edge cases:
//
//   - empty input → ""
//   - single value → one mid-bar "▄"
//   - all-equal values → all mid-bars (one level above floor, readable)
//   - NaN/Inf values are clamped to the floor
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 {
		return string(sparkBlocks[len(sparkBlocks)/2])
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	b.Grow(len(values) * 4)
	span := max - min
	for _, v := range values {
		var idx int
		switch {
		case span <= 0:
			// all-equal: render mid-bar
			idx = len(sparkBlocks) / 2
		case v <= min:
			idx = 0
		case v >= max:
			idx = len(sparkBlocks) - 1
		default:
			idx = int((v - min) / span * float64(len(sparkBlocks)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(sparkBlocks) {
				idx = len(sparkBlocks) - 1
			}
		}
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}
