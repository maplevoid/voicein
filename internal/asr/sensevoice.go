package asr

import (
	"fmt"
	"os"
	"strings"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"

	"github.com/zway/voicein/internal/config"
)

type Engine struct {
	mu         sync.Mutex
	recognizer *sherpa.OfflineRecognizer
	cfg        config.Config
}

type Result struct {
	Text     string
	Language string
}

func Open(cfg config.Config) (*Engine, error) {
	onnx := cfg.ModelOnnx()
	tokens := cfg.ModelTokens()
	for _, p := range []string{onnx, tokens} {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("model %s: %w", p, err)
		}
	}
	c := sherpa.OfflineRecognizerConfig{}
	c.FeatConfig.SampleRate = cfg.SampleRate
	c.FeatConfig.FeatureDim = 80
	c.ModelConfig.SenseVoice.Model = onnx
	c.ModelConfig.SenseVoice.Language = cfg.Language
	if cfg.ITN {
		c.ModelConfig.SenseVoice.UseInverseTextNormalization = 1
	}
	c.ModelConfig.Tokens = tokens
	c.ModelConfig.NumThreads = cfg.Threads
	c.ModelConfig.Provider = "cpu"
	c.ModelConfig.Debug = 0
	rec := sherpa.NewOfflineRecognizer(&c)
	if rec == nil {
		return nil, fmt.Errorf("failed to load SenseVoice from %s", onnx)
	}
	return &Engine{recognizer: rec, cfg: cfg}, nil
}

func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(e.recognizer)
		e.recognizer = nil
	}
}

func (e *Engine) Decode(samples []float32) (Result, error) {
	if len(samples) == 0 {
		return Result{}, fmt.Errorf("empty audio")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.recognizer == nil {
		return Result{}, fmt.Errorf("recognizer closed")
	}
	stream := sherpa.NewOfflineStream(e.recognizer)
	if stream == nil {
		return Result{}, fmt.Errorf("failed to create decode stream")
	}
	defer sherpa.DeleteOfflineStream(stream)
	stream.AcceptWaveform(e.cfg.SampleRate, samples)
	e.recognizer.Decode(stream)
	out := stream.GetResult()
	text := strings.TrimSpace(out.Text)
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	return Result{Text: text, Language: out.Lang}, nil
}
