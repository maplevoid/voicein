package scribe

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/zway/scribe/proto"
)

func Transcribe(ctx context.Context, socket string, sampleRate int, samples []float32) (string, error) {
	if socket == "" {
		return "", fmt.Errorf("scribe socket is empty")
	}
	if sampleRate <= 0 {
		return "", fmt.Errorf("bad sample rate %d", sampleRate)
	}
	pcm := pcm16le(samples)
	if ctx == nil {
		ctx = context.Background()
	}
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return "", fmt.Errorf("scribe not running (%s): %w", socket, err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	go func() {
		<-ctx.Done()
		_ = conn.SetDeadline(time.Now())
	}()

	if err := proto.WriteRequest(conn, sampleRate, pcm); err != nil {
		return "", err
	}
	rep, err := proto.ReadReply(conn)
	if err != nil {
		return "", err
	}
	if rep.Status != proto.StatusOK {
		return "", fmt.Errorf("%s", rep.Text)
	}
	return rep.Text, nil
}

// Warmup starts the scribe process and loads a local engine.
// Empty PCM is not transcribed. Failures are the caller's to log.
func Warmup(ctx context.Context, socket string, sampleRate int) error {
	if socket == "" {
		return fmt.Errorf("scribe socket is empty")
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if ctx == nil {
		ctx = context.Background()
	}
	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return fmt.Errorf("scribe not running (%s): %w", socket, err)
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	go func() {
		<-ctx.Done()
		_ = conn.SetDeadline(time.Now())
	}()
	if err := proto.WriteRequest(conn, sampleRate, nil); err != nil {
		return err
	}
	rep, err := proto.ReadReply(conn)
	if err != nil {
		return err
	}
	if rep.Status != proto.StatusOK {
		return fmt.Errorf("%s", rep.Text)
	}
	return nil
}

func pcm16le(samples []float32) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		if s > 1 {
			s = 1
		} else if s < -1 {
			s = -1
		}
		v := int16(math.Round(float64(s) * 32767))
		binary.LittleEndian.PutUint16(out[i*2:], uint16(v))
	}
	return out
}
