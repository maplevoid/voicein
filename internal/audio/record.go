package audio

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"sync"
)

// Recorder captures mono s16le PCM from cfg.Record.Command (pw-record by default).
type Recorder struct {
	cmd     *exec.Cmd
	ctx     context.Context
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
		ctx:     ctx,
		cancel:  cancel,
		samples: make(chan []float32, 64),
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
		case <-r.ctx.Done():
			return
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

// Bands is a one-shot wrapper around Bank for tests.
func Bands(samples []float32, n int) []float32 {
	b := NewBank(n, 16000)
	return b.Push(samples)
}

// Bank is n constant-Q bandpass filters across the speech band.
type Bank struct {
	n  int
	bp []biquad
}

type biquad struct {
	b0, b1, b2, a1, a2 float64
	z1, z2             float64
}

func NewBank(n, sampleRate int) *Bank {
	if n <= 0 {
		n = 1
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	b := &Bank{n: n, bp: make([]biquad, n)}
	const fMin, fMax = 180.0, 3800.0
	for i := range n {
		lo := fMin * math.Pow(fMax/fMin, float64(i)/float64(n))
		hi := fMin * math.Pow(fMax/fMin, float64(i+1)/float64(n))
		fc := math.Sqrt(lo * hi)
		q := fc / (hi - lo)
		if q < 1.2 {
			q = 1.2
		}
		b.bp[i] = newBandpass(fc, q, float64(sampleRate))
	}
	return b
}

func newBandpass(fc, q, sr float64) biquad {
	w0 := 2 * math.Pi * fc / sr
	alpha := math.Sin(w0) / (2 * q)
	a0 := 1 + alpha
	return biquad{
		b0: alpha / a0,
		b1: 0,
		b2: -alpha / a0,
		a1: -2 * math.Cos(w0) / a0,
		a2: (1 - alpha) / a0,
	}
}

func (f *biquad) tick(x float64) float64 {
	y := f.b0*x + f.z1
	f.z1 = f.b1*x - f.a1*y + f.z2
	f.z2 = f.b2*x - f.a2*y
	return y
}

func (b *Bank) Push(samples []float32) []float32 {
	if b == nil {
		return nil
	}
	out := make([]float32, b.n)
	if len(samples) == 0 {
		return out
	}
	mean := mean32(samples)
	const gain = 3.2
	for i := range b.n {
		var acc float64
		f := &b.bp[i]
		for _, x := range samples {
			y := f.tick(float64(x) - mean)
			if y < 0 {
				acc -= y
			} else {
				acc += y
			}
		}
		a := (acc / float64(len(samples))) * gain
		if a > 1 {
			a = 1
		}
		out[i] = float32(a)
	}
	return out
}
