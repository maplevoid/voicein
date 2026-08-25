package audio

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Recorder captures mono s16le PCM from cfg.Record.Command (pw-record by default).
type Recorder struct {
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	samples chan []float32
	errc    chan error
	once    sync.Once
}

func Start(ctx context.Context, argv []string, sampleRate int) (*Recorder, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty record command")
	}
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	r := &Recorder{
		cmd:     cmd,
		cancel:  cancel,
		samples: make(chan []float32, 8),
		errc:    make(chan error, 1),
	}
	go r.read(stdout, sampleRate)
	go func() {
		err := cmd.Wait()
		r.once.Do(func() {
			if err != nil && ctx.Err() == nil {
				r.errc <- err
			}
			close(r.errc)
		})
	}()
	return r, nil
}

func (r *Recorder) Samples() <-chan []float32 { return r.samples }
func (r *Recorder) Err() <-chan error         { return r.errc }

func (r *Recorder) Stop() {
	r.cancel()
	if r.cmd.Process != nil {
		_ = r.cmd.Process.Kill()
	}
}

func (r *Recorder) read(rc io.ReadCloser, sampleRate int) {
	defer rc.Close()
	defer close(r.samples)
	// ~20ms frames keep HUD and silence timer responsive.
	frame := sampleRate / 50
	if frame < 160 {
		frame = 160
	}
	buf := make([]byte, frame*2)
	br := bufio.NewReaderSize(rc, 16*1024)
	for {
		if _, err := io.ReadFull(br, buf); err != nil {
			return
		}
		out := make([]float32, frame)
		for i := range frame {
			s := int16(binary.LittleEndian.Uint16(buf[i*2:]))
			out[i] = float32(s) / 32768
		}
		select {
		case r.samples <- out:
		default:
			// Drop a frame rather than blocking the recorder if the consumer stalls.
		}
	}
}

func RMS(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}
	mean := mean32(samples)
	var sum float64
	for _, s := range samples {
		d := float64(s) - mean
		sum += d * d
	}
	return float32(sum / float64(len(samples)))
}

func mean32(samples []float32) float64 {
	var sum float64
	for _, s := range samples {
		sum += float64(s)
	}
	return sum / float64(len(samples))
}
