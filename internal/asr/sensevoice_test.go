package asr

import (
	"os"
	"testing"
	"time"

	"github.com/zway/voicein/internal/config"
)

func TestDecodeSmoke(t *testing.T) {
	if os.Getenv("VOICEIN_ASR_SMOKE") == "" {
		t.Skip("set VOICEIN_ASR_SMOKE=1 to run local SenseVoice smoke")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("model=%s threads=%d", cfg.ModelOnnx(), cfg.Threads)
	t0 := time.Now()
	eng, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	t.Logf("open %s", time.Since(t0))

	n := cfg.SampleRate // 1s
	pcm := make([]float32, n)
	for i := range pcm {
		pcm[i] = 0.05 * float32(i%72-36) / 36
	}
	t1 := time.Now()
	res, err := eng.Decode(pcm)
	t.Logf("decode %s err=%v text=%q lang=%q", time.Since(t1), err, res.Text, res.Language)
	if err != nil {
		t.Fatal(err)
	}
}
