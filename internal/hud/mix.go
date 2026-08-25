package hud

import (
	"math"

	"github.com/zway/voicein/internal/wave"
)

func mixBars(levels []float32, ms float32, bands []float32) []float32 {
	if len(levels) != wave.BarCount {
		levels = make([]float32, wave.BarCount)
	}
	if len(bands) != wave.BarCount {
		padded := make([]float32, wave.BarCount)
		copy(padded, bands)
		bands = padded
	}
	raw := float32(math.Sqrt(float64(ms)))
	// Absolute compressive map so quiet and loud stay distinct.
	// 0.05 RMS → ~0.33, 0.11 → ~0.52, 0.55 → ~0.85.
	env := raw / (raw + 0.10)
	if env > 1 {
		env = 1
	}
	var peak float32
	for _, v := range bands {
		if v > peak {
			peak = v
		}
	}
	const attack, decay = float32(0.62), float32(0.22)
	for i := range wave.BarCount {
		shape := float32(0.45)
		if peak > 1e-5 {
			shape = 0.22 + 0.78*(bands[i]/peak)
		}
		v := env * shape
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		cur := levels[i]
		if v > cur {
			levels[i] = cur + (v-cur)*attack
		} else {
			levels[i] = cur + (v-cur)*decay
		}
	}
	return levels
}
