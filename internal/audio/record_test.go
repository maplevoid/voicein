package audio

import (
	"context"
	"math"
	"os"
	"testing"
	"time"
)

func TestRMSSilenceAndPeak(t *testing.T) {
	if RMS(nil) != 0 {
		t.Fatal("empty")
	}
	if RMS([]float32{0, 0, 0}) != 0 {
		t.Fatal("silence")
	}
	peak := RMS([]float32{1, -1})
	if peak < 0.99 || peak > 1.01 {
		t.Fatalf("peak %v", peak)
	}
	dc := RMS([]float32{0.2, 0.2, 0.2, 0.2})
	if dc > 1e-6 {
		t.Fatalf("dc should be zero, got %v", dc)
	}
}

func TestBandsPeaksNearTone(t *testing.T) {
	const n, sr = 320, 16000.0
	low := Bands(tone(n, 250, sr), 7)
	high := Bands(tone(n, 3000, sr), 7)
	if argmax(low) >= argmax(high) {
		t.Fatalf("low %v high %v", low, high)
	}
	if max32(low) < 0.2 || max32(high) < 0.2 {
		t.Fatalf("too quiet low %v high %v", low, high)
	}
	if max32(Bands(make([]float32, n), 7)) != 0 {
		t.Fatal("silence should be zero")
	}
	biased := tone(n, 250, sr)
	for i := range biased {
		biased[i] += 0.25
	}
	if argmax(Bands(biased, 7)) > 2 {
		t.Fatalf("dc+low %v", Bands(biased, 7))
	}
}

func TestRecorderFinitePCMDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/tone.s16"
	pcm := make([]byte, 320*2*8)
	if err := os.WriteFile(path, pcm, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rec, err := Start(ctx, []string{"cat", path}, 16000)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Stop()
	got := 0
	for range rec.Samples() {
		got++
	}
	if got == 0 {
		t.Fatal("no frames")
	}
}

func tone(n int, freq, sr float64) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(math.Sin(2 * math.Pi * freq * float64(i) / sr))
	}
	return out
}

func argmax(v []float32) int {
	best := 0
	for i := range v {
		if v[i] > v[best] {
			best = i
		}
	}
	return best
}

func max32(v []float32) float32 {
	var m float32
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}
