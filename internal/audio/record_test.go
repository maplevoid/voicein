package audio

import "testing"

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
}
