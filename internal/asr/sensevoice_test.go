package asr

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zway/voicein/internal/config"
)

func TestCleanTranscriptStripsSenseVoiceTags(t *testing.T) {
	text, lang := cleanTranscript("<|en|><|NEUTRAL|><|Speech|>hello world", "")
	if text != "hello world" || lang != "en" {
		t.Fatalf("got text=%q lang=%q", text, lang)
	}
	_, lang = cleanTranscript("继续继续。", "<|zh|>")
	if lang != "zh" {
		t.Fatalf("lang=%q", lang)
	}
}

func TestRecognizerConfigSelectsWhisper(t *testing.T) {
	cfg := config.Defaults()
	cfg.Model.Engine = "whisper"
	cfg.Model.Encoder = "/tmp/enc.onnx"
	cfg.Model.Decoder = "/tmp/dec.onnx"
	cfg.Model.Tokens = "/tmp/tokens.txt"
	cfg.Language = "auto"
	c := recognizerConfig(cfg)
	if c.ModelConfig.ModelType != "whisper" {
		t.Fatalf("type %q", c.ModelConfig.ModelType)
	}
	if c.ModelConfig.Whisper.Encoder != "/tmp/enc.onnx" || c.ModelConfig.Whisper.Decoder != "/tmp/dec.onnx" {
		t.Fatalf("whisper paths %+v", c.ModelConfig.Whisper)
	}
	if c.ModelConfig.Whisper.Language != "" || c.ModelConfig.Whisper.Task != "transcribe" {
		t.Fatalf("whisper opts %+v", c.ModelConfig.Whisper)
	}
	if c.ModelConfig.SenseVoice.Model != "" {
		t.Fatalf("sensevoice still set: %q", c.ModelConfig.SenseVoice.Model)
	}
}

func TestRequireModelFilesNeedWhisperPair(t *testing.T) {
	cfg := config.Defaults()
	cfg.Model.Engine = "whisper"
	if err := requireModelFiles(cfg); err == nil {
		t.Fatal("expected missing whisper files")
	}
}

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

func TestDecodeSmoke(t *testing.T) {
	if os.Getenv("VOICEIN_ASR_SMOKE") == "" {
		t.Skip("set VOICEIN_ASR_SMOKE=1 to run local SenseVoice smoke")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("engine=%s model=%s encoder=%s decoder=%s threads=%d", cfg.EngineKind(), cfg.ModelOnnx(), cfg.ModelEncoder(), cfg.ModelDecoder(), cfg.Threads)
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

func TestDecodeLatencyCompare(t *testing.T) {
	if os.Getenv("VOICEIN_ASR_SMOKE") == "" {
		t.Skip("set VOICEIN_ASR_SMOKE=1 to compare engine latency")
	}
	pcm, sr := loadPCM16(t, "/tmp/voicein-models/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17/test_wavs/zh.wav")
	if sr != 16000 {
		t.Fatalf("sr=%d", sr)
	}
	base := config.Defaults()
	base.Threads = 4
	base.Language = "auto"
	base.Model.Dir = filepath.Join(os.Getenv("HOME"), ".local/share/voicein/models")

	type spec struct {
		name string
		mut  func(*config.Config)
	}
	cases := []spec{
		{"sensevoice", func(c *config.Config) {
			c.Model.Engine = "sensevoice"
			c.Model.Onnx = "model.int8.onnx"
			c.Model.Tokens = "tokens.txt"
		}},
		{"whisper-small", func(c *config.Config) {
			c.Model.Engine = "whisper"
			c.Model.Encoder = "small-encoder.int8.onnx"
			c.Model.Decoder = "small-decoder.int8.onnx"
			c.Model.Tokens = "small-tokens.txt"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mut(&cfg)
			t0 := time.Now()
			eng, err := Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer eng.Close()
			open := time.Since(t0)
			// warmup
			if _, err := eng.Decode(pcm[:min(len(pcm), cfg.SampleRate)]); err != nil {
				t.Fatal(err)
			}
			t1 := time.Now()
			res, err := eng.Decode(pcm)
			dec := time.Since(t1)
			t.Logf("open=%s decode=%s samples=%d dur=%.2fs rtf=%.2f text=%q lang=%q err=%v",
				open, dec, len(pcm), float64(len(pcm))/float64(sr), dec.Seconds()/(float64(len(pcm))/float64(sr)), res.Text, res.Language, err)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func loadPCM16(t *testing.T, path string) ([]float32, int) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 44 || string(b[0:4]) != "RIFF" {
		t.Fatalf("not wav: %s", path)
	}
	sr := int(binary.LittleEndian.Uint32(b[24:28]))
	data := b[44:]
	pcm := make([]float32, len(data)/2)
	for i := range pcm {
		pcm[i] = float32(int16(binary.LittleEndian.Uint16(data[i*2:]))) / 32768
	}
	return pcm, sr
}
