package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/zway/voicein/internal/asr"
	"github.com/zway/voicein/internal/audio"
	"github.com/zway/voicein/internal/config"
	"github.com/zway/voicein/internal/hud"
	"github.com/zway/voicein/internal/inject"
	"github.com/zway/voicein/internal/ipc"
	"github.com/zway/voicein/internal/keys"
	"github.com/zway/voicein/internal/wave"
)

type Daemon struct {
	cfg      config.Config
	engine   *asr.Engine
	ep       *asr.Endpoint
	hud      *hud.HUD
	mu       sync.Mutex
	state    ipc.State
	err      string
	text     string
	started  time.Time
	latched  bool
	gen      uint64
	cancel   context.CancelFunc
	stop     chan struct{}
	quit     chan struct{}
	quitOnce sync.Once
}

func Run(cfg config.Config) error {
	if err := os.MkdirAll(cfg.Model.Dir, 0o755); err != nil {
		return err
	}
	engine, err := asr.Open(cfg)
	if err != nil {
		return err
	}
	defer engine.Close()
	switch cfg.EngineKind() {
	case "whisper":
		log.Printf("engine whisper encoder=%s decoder=%s", cfg.ModelEncoder(), cfg.ModelDecoder())
	default:
		log.Printf("engine sensevoice model=%s", cfg.ModelOnnx())
	}

	ln, err := ipc.Listen(cfg.Socket)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(cfg.Socket)

	ep := asr.NewEndpoint(cfg)
	endHow := "end on second keypress"
	switch cfg.RecordMode() {
	case "hold":
		endHow = "end on key release"
	case "hybrid":
		endHow = fmt.Sprintf("tap <%s latches, hold releases", cfg.Tap)
	}
	if ep.Live() {
		log.Printf("vad ready %s; %s", cfg.ModelVAD(), endHow)
	} else {
		log.Printf("vad unavailable; energy gate fallback, %s", endHow)
	}
	d := &Daemon{
		cfg:    cfg,
		engine: engine,
		ep:     ep,
		hud:    hud.Start(cfg.HUD),
		state:  ipc.StateIdle,
		quit:   make(chan struct{}),
	}
	defer d.ep.Close()
	defer d.hud.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		d.requestQuit()
	}()

	log.Printf("listening on %s", cfg.Socket)
	if chord, err := keys.ParseHotkey(cfg.Hotkey); err != nil {
		log.Printf("hotkey %q: %v", cfg.Hotkey, err)
	} else if !chord.Empty() {
		log.Printf("hotkey %s via evdev; niri bind optional (swallow key)", cfg.Hotkey)
		go d.listenHotkey(ctx, chord)
	}
	errc := make(chan error, 1)
	go func() {
		errc <- ipc.Serve(ln, d.handle)
	}()

	select {
	case <-d.quit:
		_ = ln.Close()
		<-errc
		return nil
	case err := <-errc:
		return err
	}
}

func (d *Daemon) handle(cmd ipc.Command) ipc.Reply {
	switch cmd {
	case ipc.CmdStatus:
		return ipc.Reply{OK: true, Status: d.snapshot()}
	case ipc.CmdQuit:
		d.requestQuit()
		return ipc.Reply{OK: true, Status: d.snapshot()}
	case ipc.CmdCancel:
		d.cancelRec()
		return ipc.Reply{OK: true, Status: d.snapshot()}
	case ipc.CmdToggle:
		return d.toggle()
	case ipc.CmdDown:
		return d.press()
	case ipc.CmdUp:
		return d.release()
	default:
		return ipc.Reply{OK: false, Error: fmt.Sprintf("unknown command %q", cmd), Status: d.snapshot()}
	}
}

func (d *Daemon) listenHotkey(ctx context.Context, chord keys.Chord) {
	err := keys.Listen(ctx, chord, func(down bool) {
		d.hotkey(down)
	})
	if err != nil && ctx.Err() == nil {
		log.Printf("hotkey listen: %v", err)
	}
}

func (d *Daemon) hotkey(down bool) {
	d.mu.Lock()
	st := d.state
	latched := d.latched
	started := d.started
	mode := d.cfg.RecordMode()
	tap := d.cfg.Tap
	d.mu.Unlock()

	held := time.Duration(0)
	if st == ipc.StateRecording && !started.IsZero() {
		held = time.Since(started)
	}
	switch keys.Classify(mode, down, st == ipc.StateRecording, latched, held, tap) {
	case keys.ActionStart:
		_ = d.beginRec()
	case keys.ActionLatch:
		d.latch()
		log.Printf("hotkey latched (held %s < %s)", held.Round(time.Millisecond), tap)
	case keys.ActionStop:
		d.finishRec()
	}
}

func (d *Daemon) latch() {
	d.mu.Lock()
	if d.state == ipc.StateRecording {
		d.latched = true
	}
	d.mu.Unlock()
}

func (d *Daemon) toggle() ipc.Reply {
	d.mu.Lock()
	st := d.state
	d.mu.Unlock()
	switch st {
	case ipc.StateIdle:
		return d.beginRec()
	case ipc.StateRecording:
		d.finishRec()
		return ipc.Reply{OK: true, Status: d.snapshot()}
	default:
		return ipc.Reply{OK: true, Status: d.snapshot()}
	}
}

func (d *Daemon) press() ipc.Reply {
	d.mu.Lock()
	st := d.state
	d.mu.Unlock()
	if st == ipc.StateIdle {
		return d.beginRec()
	}
	return ipc.Reply{OK: true, Status: d.snapshot()}
}

func (d *Daemon) release() ipc.Reply {
	d.mu.Lock()
	st := d.state
	d.mu.Unlock()
	if st == ipc.StateRecording {
		d.finishRec()
	}
	return ipc.Reply{OK: true, Status: d.snapshot()}
}

func (d *Daemon) beginRec() ipc.Reply {
	if _, err := d.startRec(); err != nil {
		d.setIdle(err.Error(), "")
		inject.Notify(context.Background(), d.cfg, "voicein", err.Error())
		return ipc.Reply{OK: false, Error: err.Error(), Status: d.snapshot()}
	}
	return ipc.Reply{OK: true, Status: d.snapshot()}
}

func (d *Daemon) startRec() (context.Context, error) {
	d.mu.Lock()
	if d.state != ipc.StateIdle {
		d.mu.Unlock()
		return context.Background(), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.stop = make(chan struct{})
	d.state = ipc.StateRecording
	d.err = ""
	d.latched = false
	d.started = time.Now()
	d.mu.Unlock()

	rec, err := audio.Start(ctx, d.cfg.Record.Command, d.cfg.SampleRate)
	if err != nil {
		cancel()
		return ctx, err
	}
	go d.recordLoop(ctx, rec)
	return ctx, nil
}

func (d *Daemon) recordLoop(ctx context.Context, rec *audio.Recorder) {
	defer rec.Stop()
	d.ep.Reset()

	var (
		pcm         []float32
		lastVoice   = time.Now()
		heard       bool
		speechFloor = float32(0.0004)
		bank        = audio.NewBank(wave.BarCount, d.cfg.SampleRate)
		bands       = make([]float32, wave.BarCount)
	)
	deadline := time.NewTimer(d.cfg.MaxRecord)
	defer deadline.Stop()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	flush := func(reason error) {
		rec.Stop()
		d.transcribe(asr.TrimSpeech(pcm, d.cfg.SampleRate), reason)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			flush(nil)
			return
		case err := <-rec.Err():
			if err != nil && ctx.Err() == nil {
				d.setIdle(err.Error(), "")
				inject.Notify(context.Background(), d.cfg, "voicein", err.Error())
			}
			return
		case <-deadline.C:
			flush(nil)
			return
		case <-tick.C:
			d.hud.Update(hud.Level{
				RMS:     lastRMS(pcm),
				Bands:   append([]float32(nil), bands...),
				Active:  heard && time.Since(lastVoice) < 400*time.Millisecond,
				Seconds: int(time.Since(d.started).Seconds()),
			})
		case frame, ok := <-rec.Samples():
			if !ok {
				if ctx.Err() == nil {
					flush(nil)
				}
				return
			}
			pcm = append(pcm, frame...)
			rms := audio.RMS(frame)
			bands = bank.Push(frame)
			speech := rms >= speechFloor || d.ep.Push(frame)
			if speech {
				heard = true
				lastVoice = time.Now()
			}
			d.hud.Update(hud.Level{
				RMS:     rms,
				Bands:   append([]float32(nil), bands...),
				Active:  speech,
				Seconds: int(time.Since(d.started).Seconds()),
			})
		}
	}
}

func (d *Daemon) transcribe(pcm []float32, recErr error) {
	d.mu.Lock()
	if d.state != ipc.StateRecording {
		d.mu.Unlock()
		return
	}
	d.state = ipc.StateTranscribing
	gen := d.gen
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	d.mu.Unlock()

	if recErr != nil {
		d.finishIdle(gen, recErr.Error(), "")
		inject.Notify(context.Background(), d.cfg, "voicein", recErr.Error())
		return
	}
	if len(pcm) < d.cfg.SampleRate/5 {
		d.finishIdle(gen, "too short", "")
		return
	}
	t0 := time.Now()
	res, err := d.engine.Decode(pcm)
	log.Printf("decode %d samples in %s lang=%q err=%v text=%q", len(pcm), time.Since(t0), res.Language, err, res.Text)
	d.mu.Lock()
	stale := d.gen != gen
	d.mu.Unlock()
	if stale {
		return
	}
	if err != nil {
		d.finishIdle(gen, err.Error(), "")
		inject.Notify(context.Background(), d.cfg, "voicein", err.Error())
		return
	}
	if res.Text == "" {
		d.finishIdle(gen, "empty transcript", "")
		return
	}
	if _, err := inject.Text(context.Background(), d.cfg, res.Text); err != nil {
		d.finishIdle(gen, err.Error(), res.Text)
		inject.Notify(context.Background(), d.cfg, "voicein", "clipboard ok, inject failed: "+err.Error())
		return
	}
	d.finishIdle(gen, "", res.Text)
}

func (d *Daemon) finishRec() {
	d.mu.Lock()
	stop := d.stop
	d.mu.Unlock()
	if stop == nil {
		return
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
}

func (d *Daemon) cancelRec() {
	d.mu.Lock()
	cancel := d.cancel
	st := d.state
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if st == ipc.StateRecording || st == ipc.StateTranscribing {
		d.setIdle("cancelled", "")
	}
}

func (d *Daemon) setIdle(errText, text string) {
	d.mu.Lock()
	d.gen++
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	d.state = ipc.StateIdle
	d.latched = false
	d.err = errText
	if text != "" {
		d.text = text
	}
	d.mu.Unlock()
	d.hud.Hide()
}

func (d *Daemon) finishIdle(gen uint64, errText, text string) {
	d.mu.Lock()
	if d.gen != gen {
		d.mu.Unlock()
		return
	}
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	d.state = ipc.StateIdle
	d.latched = false
	d.err = errText
	if text != "" {
		d.text = text
	}
	d.mu.Unlock()
	d.hud.Hide()
}

func (d *Daemon) snapshot() ipc.Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := ipc.Status{State: d.state, Error: d.err, Text: d.text}
	if d.state == ipc.StateRecording && !d.started.IsZero() {
		s.Seconds = int(time.Since(d.started).Seconds())
	}
	return s
}

func (d *Daemon) requestQuit() {
	d.cancelRec()
	d.quitOnce.Do(func() { close(d.quit) })
}

func lastRMS(pcm []float32) float32 {
	if len(pcm) == 0 {
		return 0
	}
	n := 320
	if len(pcm) < n {
		n = len(pcm)
	}
	return audio.RMS(pcm[len(pcm)-n:])
}
