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
)

type Daemon struct {
	cfg      config.Config
	engine   *asr.Engine
	hud      *hud.HUD
	mu       sync.Mutex
	state    ipc.State
	err      string
	text     string
	started  time.Time
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

	ln, err := ipc.Listen(cfg.Socket)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(cfg.Socket)

	d := &Daemon{
		cfg:    cfg,
		engine: engine,
		hud:    hud.Start(cfg.HUD),
		state:  ipc.StateIdle,
		quit:   make(chan struct{}),
	}
	defer d.hud.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		d.requestQuit()
	}()

	log.Printf("listening on %s", cfg.Socket)
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
	default:
		return ipc.Reply{OK: false, Error: fmt.Sprintf("unknown command %q", cmd), Status: d.snapshot()}
	}
}

func (d *Daemon) toggle() ipc.Reply {
	d.mu.Lock()
	st := d.state
	d.mu.Unlock()
	switch st {
	case ipc.StateIdle:
		if err := d.startRec(); err != nil {
			d.setIdle(err.Error(), "")
			inject.Notify(context.Background(), d.cfg, "voicein", err.Error())
			return ipc.Reply{OK: false, Error: err.Error(), Status: d.snapshot()}
		}
		return ipc.Reply{OK: true, Status: d.snapshot()}
	case ipc.StateRecording:
		d.finishRec()
		return ipc.Reply{OK: true, Status: d.snapshot()}
	default:
		return ipc.Reply{OK: true, Status: d.snapshot()}
	}
}

func (d *Daemon) startRec() error {
	d.mu.Lock()
	if d.state != ipc.StateIdle {
		d.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.stop = make(chan struct{})
	d.state = ipc.StateRecording
	d.err = ""
	d.started = time.Now()
	d.mu.Unlock()

	rec, err := audio.Start(ctx, d.cfg.Record.Command, d.cfg.SampleRate)
	if err != nil {
		cancel()
		return err
	}
	go d.recordLoop(ctx, rec)
	return nil
}

func (d *Daemon) recordLoop(ctx context.Context, rec *audio.Recorder) {
	defer rec.Stop()

	var (
		pcm         []float32
		lastVoice   = time.Now()
		heard       bool
		speechFloor = float32(0.0004)
	)
	deadline := time.NewTimer(d.cfg.MaxRecord)
	defer deadline.Stop()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	flush := func(reason error) {
		rec.Stop()
		d.transcribe(pcm, reason)
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
			if heard && time.Since(lastVoice) >= d.cfg.Silence {
				flush(nil)
				return
			}
			d.hud.Update(hud.Level{
				RMS:     lastRMS(pcm),
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
			if audio.RMS(frame) >= speechFloor {
				heard = true
				lastVoice = time.Now()
			}
			d.hud.Update(hud.Level{
				RMS:     audio.RMS(frame),
				Active:  audio.RMS(frame) >= speechFloor,
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
	log.Printf("decode %d samples in %s: %v", len(pcm), time.Since(t0), err)
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
