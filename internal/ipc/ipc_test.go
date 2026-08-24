package ipc

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCallToggleStatus(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "voicein.sock")
	ln, err := Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	got := make(chan Command, 4)
	go func() {
		_ = Serve(ln, func(cmd Command) Reply {
			got <- cmd
			st := Status{State: StateIdle}
			if cmd == CmdToggle {
				st.State = StateRecording
				st.Seconds = 1
			}
			return Reply{OK: true, Status: st}
		})
	}()

	reply, err := Call(sock, CmdToggle)
	if err != nil {
		t.Fatal(err)
	}
	if !reply.OK || reply.Status.State != StateRecording {
		t.Fatalf("toggle reply %+v", reply)
	}
	select {
	case cmd := <-got:
		if cmd != CmdToggle {
			t.Fatalf("handler saw %q", cmd)
		}
	case <-time.After(time.Second):
		t.Fatal("handler not called")
	}

	reply, err = Call(sock, CmdStatus)
	if err != nil {
		t.Fatal(err)
	}
	if Format(reply.Status) != "idle" {
		t.Fatalf("status format %q", Format(reply.Status))
	}
}

func TestCallMissingSocket(t *testing.T) {
	_, err := Call(filepath.Join(t.TempDir(), "no.sock"), CmdStatus)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormat(t *testing.T) {
	if got := Format(Status{State: StateRecording, Seconds: 3}); got != "recording 3s" {
		t.Fatal(got)
	}
	if got := Format(Status{State: StateTranscribing}); got != "transcribing" {
		t.Fatal(got)
	}
	if got := Format(Status{State: StateIdle, Text: "你好"}); got != "idle last=你好" {
		t.Fatal(got)
	}
}
