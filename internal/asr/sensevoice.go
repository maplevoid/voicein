package asr

import (
	"fmt"
	"os"
	"regexp"
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

type Endpoint struct {
	vad    *sherpa.VoiceActivityDetector
	buf    []float32
	window int
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
	if err := requireModelFiles(cfg); err != nil {
		return nil, err
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

func requireModelFiles(cfg config.Config) error {
	paths := []string{cfg.ModelTokens()}
	switch cfg.EngineKind() {
	case "whisper":
		paths = append(paths, cfg.ModelEncoder(), cfg.ModelDecoder())
	case "sensevoice":
		paths = append(paths, cfg.ModelOnnx())
	default:
		return fmt.Errorf("unknown model engine %q", cfg.EngineKind())
	}
	for _, p := range paths {
		if p == "" {
			return fmt.Errorf("model engine %s is missing a required path", cfg.EngineKind())
		}
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("model %s: %w", p, err)
		}
	}
	return nil
}

func (e *Engine) loop(errc chan error) {
	runtime.LockOSThread()
	defer close(e.done)

	c := recognizerConfig(e.cfg)
	rec := sherpa.NewOfflineRecognizer(&c)
	if rec == nil {
		errc <- fmt.Errorf("failed to load %s", e.cfg.EngineKind())
		return
	}
	defer sherpa.DeleteOfflineRecognizer(rec)
	errc <- nil

	for j := range e.jobs {
		j.out <- decodeOne(rec, e.cfg.SampleRate, j.samples)
	}
}

func recognizerConfig(cfg config.Config) sherpa.OfflineRecognizerConfig {
	c := sherpa.OfflineRecognizerConfig{}
	c.FeatConfig.SampleRate = cfg.SampleRate
	c.FeatConfig.FeatureDim = 80
	c.ModelConfig.Tokens = cfg.ModelTokens()
	c.ModelConfig.NumThreads = cfg.Threads
	c.ModelConfig.Provider = "cpu"
	c.ModelConfig.Debug = 0
	c.DecodingMethod = "greedy_search"
	switch cfg.EngineKind() {
	case "whisper":
		lang := cfg.Language
		if lang == "auto" {
			lang = ""
		}
		c.ModelConfig.ModelType = "whisper"
		c.ModelConfig.Whisper.Encoder = cfg.ModelEncoder()
		c.ModelConfig.Whisper.Decoder = cfg.ModelDecoder()
		c.ModelConfig.Whisper.Language = lang
		c.ModelConfig.Whisper.Task = "transcribe"
	default:
		c.ModelConfig.ModelType = "sense_voice"
		c.ModelConfig.SenseVoice.Model = cfg.ModelOnnx()
		c.ModelConfig.SenseVoice.Language = cfg.Language
		if cfg.ITN {
			c.ModelConfig.SenseVoice.UseInverseTextNormalization = 1
		}
	}
	return c
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
	text, lang := cleanTranscript(out.Text, out.Lang)
	return decodeOut{res: Result{Text: text, Language: lang}}
}

var (
	senseTag = regexp.MustCompile(`<\|[^|]*\|>`)
	langTag  = regexp.MustCompile(`<\|(zh|en|yue|ja|ko|auto)\|>`)
)

func cleanTranscript(text, lang string) (string, string) {
	if m := langTag.FindStringSubmatch(lang); len(m) == 2 {
		lang = m[1]
	} else if m := langTag.FindStringSubmatch(text); len(m) == 2 {
		lang = m[1]
	}
	text = senseTag.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	return text, lang
}

func NewEndpoint(cfg config.Config) *Endpoint {
	path := cfg.ModelVAD()
	if path == "" {
		return &Endpoint{}
	}
	if _, err := os.Stat(path); err != nil {
		return &Endpoint{}
	}
	vad := sherpa.NewVoiceActivityDetector(&sherpa.VadModelConfig{
		SileroVad: sherpa.SileroVadModelConfig{
			Model:              path,
			Threshold:          0.5,
			MinSilenceDuration: 0.3,
			MinSpeechDuration:  0.15,
			WindowSize:         512,
			MaxSpeechDuration:  60,
		},
		SampleRate: cfg.SampleRate,
		NumThreads: 1,
		Provider:   "cpu",
	}, 30)
	if vad == nil {
		return &Endpoint{}
	}
	return &Endpoint{vad: vad, window: 512}
}

func (e *Endpoint) Live() bool { return e != nil && e.vad != nil }

func (e *Endpoint) Push(samples []float32) bool {
	if !e.Live() || len(samples) == 0 {
		return false
	}
	e.buf = append(e.buf, samples...)
	speech := false
	for len(e.buf) >= e.window {
		chunk := append([]float32(nil), e.buf[:e.window]...)
		e.buf = e.buf[e.window:]
		e.vad.AcceptWaveform(chunk)
		if e.vad.IsSpeech() {
			speech = true
		}
		for !e.vad.IsEmpty() {
			e.vad.Pop()
		}
	}
	return speech
}

func (e *Endpoint) Reset() {
	if e == nil {
		return
	}
	e.buf = e.buf[:0]

	if e.vad != nil {
		e.vad.Clear()
		e.vad.Reset()
	}
}

func (e *Endpoint) Close() {
	if e == nil || e.vad == nil {
		return
	}
	sherpa.DeleteVoiceActivityDetector(e.vad)
	e.vad = nil
}

func TrimSpeech(pcm []float32, sampleRate int) []float32 {
	if len(pcm) == 0 || sampleRate <= 0 {
		return pcm
	}
	frame := sampleRate / 50
	if frame < 160 {
		frame = 160
	}
	const floor = float32(0.0008)
	first, last := -1, -1
	for i := 0; i+frame <= len(pcm); i += frame {
		if rms(pcm[i:i+frame]) >= floor {
			if first < 0 {
				first = i
			}
			last = i + frame
		}
	}
	if first < 0 {
		return pcm
	}
	pad := sampleRate / 5
	if first > pad {
		first -= pad
	} else {
		first = 0
	}
	if last+pad < len(pcm) {
		last += pad
	} else {
		last = len(pcm)
	}
	return pcm[first:last]
}

func rms(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return float32(sum / float64(len(samples)))
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
