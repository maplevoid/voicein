package asr

import "testing"

func TestTrimSpeechDropsLeadAndTailSilence(t *testing.T) {
	const sr = 16000
	pcm := make([]float32, sr*2)
	for i := sr / 2; i < sr; i++ {
		pcm[i] = 0.2
	}
	got := TrimSpeech(pcm, sr)
	if len(got) >= len(pcm) {
		t.Fatalf("did not trim %d -> %d", len(pcm), len(got))
	}
	if len(got) < sr/2 || len(got) > sr {
		t.Fatalf("trimmed length %d", len(got))
	}
}

func TestTrimSpeechKeepsMiddlePause(t *testing.T) {
	const sr = 16000
	pcm := make([]float32, sr*4)
	for i := sr / 4; i < sr/2; i++ {
		pcm[i] = 0.2
	}
	for i := sr*3 + sr/4; i < sr*4-sr/8; i++ {
		pcm[i] = 0.2
	}
	got := TrimSpeech(pcm, sr)
	if len(got) < 3*sr {
		t.Fatalf("dropped the middle pause: %d", len(got))
	}
}
