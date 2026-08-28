package scribe

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/zway/scribe/proto"
)

func TestTranscribeOK(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "scribe.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		if _, err := proto.ReadRequest(c); err != nil {
			return
		}
		_ = proto.WriteReply(c, proto.StatusOK, "hello")
	}()

	got, err := Transcribe(t.Context(), sock, 16000, []float32{0.1, -0.1})
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatal(got)
	}
}

func TestWarmup(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "scribe.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		req, err := proto.ReadRequest(c)
		if err != nil {
			return
		}
		if len(req.PCM) != 0 {
			_ = proto.WriteReply(c, proto.StatusErr, "expected empty pcm")
			return
		}
		_ = proto.WriteReply(c, proto.StatusOK, "")
	}()

	if err := Warmup(t.Context(), sock, 16000); err != nil {
		t.Fatal(err)
	}
}

func TestTranscribeError(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "scribe.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = proto.ReadRequest(c)
		_ = proto.WriteReply(c, proto.StatusErr, "boom")
	}()
	_, err = Transcribe(t.Context(), sock, 16000, []float32{0})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("got %v", err)
	}
}

func TestMissingSocket(t *testing.T) {
	_, err := Transcribe(t.Context(), filepath.Join(t.TempDir(), "no.sock"), 16000, []float32{0})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCancel(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "scribe.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = proto.ReadRequest(c)
		time.Sleep(time.Second)
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err = Transcribe(ctx, sock, 16000, []float32{0.1})
	if err == nil {
		t.Fatal("expected cancel")
	}
}
