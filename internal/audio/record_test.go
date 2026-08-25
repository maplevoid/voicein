package audio

import (
	"context"
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
