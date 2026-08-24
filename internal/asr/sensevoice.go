package asr

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"

	"github.com/zway/voicein/internal/config"
)

type Engine struct {
	cfg  config.Config
	jobs chan job
	done chan struct{}
}

type Result struct {
	Text     string
	Language string
}

type job struct {
	samples []float32
	out     chan decodeOut
}

type decodeOut struct {
	res Result
	err error
}

func Open(cfg config.Config) (*Engine, error) {
	onnx := cfg.ModelOnnx()
	tokens := cfg.ModelTokens()
	for _, p := range []string{onnx, tokens} {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("model %s: %w", p, err)
		}
	}
	e := &Engine{
		cfg:  cfg,
		jobs: make(chan job),
		done: make(chan struct{}),
	}
	errc := make(chan error, 1)
	go e.loop(errc)
	if err := <-errc; err != nil {
		<-e.done
		return nil, err
	}
	return e, nil
}

func (e *Engine) loop(errc chan error) {
	runtime.LockOSThread()
	defer close(e.done)

	c := sherpa.OfflineRecognizerConfig{}
	c.FeatConfig.SampleRate = e.cfg.SampleRate
	c.FeatConfig.FeatureDim = 80
	c.ModelConfig.SenseVoice.Model = e.cfg.ModelOnnx()
	c.ModelConfig.SenseVoice.Language = e.cfg.Language
	if e.cfg.ITN {
		c.ModelConfig.SenseVoice.UseInverseTextNormalization = 1
	}
	c.ModelConfig.Tokens = e.cfg.ModelTokens()
	c.ModelConfig.NumThreads = e.cfg.Threads
	c.ModelConfig.Provider = "cpu"
	c.ModelConfig.Debug = 0
	c.DecodingMethod = "greedy_search"
	rec := sherpa.NewOfflineRecognizer(&c)
	if rec == nil {
		errc <- fmt.Errorf("failed to load SenseVoice from %s", e.cfg.ModelOnnx())
		return
	}
	defer sherpa.DeleteOfflineRecognizer(rec)
	errc <- nil

	for j := range e.jobs {
		j.out <- decodeOne(rec, e.cfg.SampleRate, j.samples)
	}
}

func decodeOne(rec *sherpa.OfflineRecognizer, sampleRate int, samples []float32) decodeOut {
	stream := sherpa.NewOfflineStream(rec)
	if stream == nil {
		return decodeOut{err: fmt.Errorf("failed to create decode stream")}
	}
	defer sherpa.DeleteOfflineStream(stream)
	stream.AcceptWaveform(sampleRate, samples)
	rec.Decode(stream)
	out := stream.GetResult()
	text := strings.TrimSpace(out.Text)
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	return decodeOut{res: Result{Text: text, Language: out.Lang}}
}

func (e *Engine) Close() {
	close(e.jobs)
	<-e.done
}

func (e *Engine) Decode(samples []float32) (Result, error) {
	if len(samples) == 0 {
		return Result{}, fmt.Errorf("empty audio")
	}
	out := make(chan decodeOut, 1)
	e.jobs <- job{samples: samples, out: out}
	got := <-out
	return got.res, got.err
}
