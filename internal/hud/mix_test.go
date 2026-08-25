package hud

import "testing"

func max32(v []float32) float32 {
	var m float32
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

func min32(v []float32) float32 {
	m := float32(1)
	for _, x := range v {
		if x < m {
			m = x
		}
	}
	return m
}

func TestMixBarsQuietStaysShort(t *testing.T) {
	levels := mixBars(nil, 0, make([]float32, 7))
	for i, v := range levels {
		if v > 0.02 {
			t.Fatalf("quiet bar %d=%v", i, v)
		}
	}
}

func TestMixBarsSpeechVariesAndLeavesHeadroom(t *testing.T) {
	shape := ramp(0.02, 0.9)
	levels := mixBars(nil, 0.01, shape)
	for range 5 {
		levels = mixBars(levels, 0.01, shape)
	}
	if max32(levels) >= 0.98 {
		t.Fatalf("clipped %v", levels)
	}
	if max32(levels) < 0.35 {
		t.Fatalf("too short %v", levels)
	}
	if max32(levels)-min32(levels) < 0.15 {
		t.Fatalf("no bounce %v", levels)
	}
}

func TestMixBarsQuietShorterThanLoud(t *testing.T) {
	shape := ramp(0.05, 1)
	quiet := mixBars(nil, 0.012, shape)
	loud := mixBars(nil, 0.30, shape)
	for range 7 {
		quiet = mixBars(quiet, 0.012, shape) // ~0.11 RMS
		loud = mixBars(loud, 0.30, shape)    // ~0.55 RMS
	}
	if max32(loud)-max32(quiet) < 0.18 {
		t.Fatalf("volume flattened quiet=%v loud=%v", quiet, loud)
	}
	if max32(loud) >= 0.98 {
		t.Fatalf("loud clipped %v", loud)
	}
}

func ramp(lo, hi float32) []float32 {
	out := make([]float32, 7)
	for i := range out {
		t := float32(i) / float32(len(out)-1)
		out[i] = lo + (hi-lo)*t
	}
	return out
}
