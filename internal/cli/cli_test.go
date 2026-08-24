package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/zway/voicein/internal/config"
	"github.com/zway/voicein/internal/ipc"
)

func TestHelpAndConfig(t *testing.T) {
	var buf bytes.Buffer
	if err := Run([]string{"help"}, Deps{Stdout: &buf}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "toggle") {
		t.Fatalf("help: %q", buf.String())
	}
	buf.Reset()
	if err := Run([]string{"config"}, Deps{Stdout: &buf}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[hud]") {
		t.Fatalf("config: %q", buf.String())
	}
}

func TestUnknown(t *testing.T) {
	err := Run([]string{"nope"}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("got %v", err)
	}
}

func TestStatusWithoutDaemon(t *testing.T) {
	err := Run([]string{"status"}, Deps{
		Load: func() (config.Config, error) {
			return config.Config{Socket: "/tmp/missing.sock"}, nil
		},
		Call: func(string, ipc.Command) (ipc.Reply, error) {
			return ipc.Reply{}, fmt.Errorf("connect")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "daemon not running") {
		t.Fatalf("got %v", err)
	}
}

func TestToggleOK(t *testing.T) {
	var buf bytes.Buffer
	err := Run([]string{"toggle"}, Deps{
		Stdout: &buf,
		Load: func() (config.Config, error) {
			return config.Config{Socket: "x"}, nil
		},
		Call: func(_ string, cmd ipc.Command) (ipc.Reply, error) {
			if cmd != ipc.CmdToggle {
				t.Fatalf("cmd %q", cmd)
			}
			return ipc.Reply{OK: true, Status: ipc.Status{State: ipc.StateRecording, Seconds: 2}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != "recording 2s" {
		t.Fatal(got)
	}
}
