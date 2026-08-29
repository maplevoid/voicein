package inject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zway/voicein/internal/config"
)

func TestMain(m *testing.M) {
	lookupX11 = func() bool { return false }
	os.Exit(m.Run())
}

func TestTextCopiesThenTypes(t *testing.T) {
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip")
	typed := filepath.Join(dir, "typed")
	paste := filepath.Join(dir, "paste")
	cfg := config.Defaults()
	cfg.Inject.Timeout = time.Second
	cfg.Inject.Copy = []string{"sh", "-c", "cat > " + clip}
	cfg.Inject.Type = []string{"sh", "-c", "cat > " + typed}
	cfg.Inject.Paste = []string{"sh", "-c", "echo pasted > " + paste}

	res, err := Text(context.Background(), cfg, "  你好世界  ")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Copied || res.Method != "type" {
		t.Fatalf("result %+v", res)
	}
	body, err := os.ReadFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "你好世界" {
		t.Fatalf("clipboard %q", body)
	}
	got, err := os.ReadFile(typed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "你好世界" {
		t.Fatalf("typed %q", got)
	}
	if _, err := os.Stat(paste); err == nil {
		t.Fatal("paste ran despite type succeeding")
	}
}

func TestTextFallsBackToPaste(t *testing.T) {
	dir := t.TempDir()
	paste := filepath.Join(dir, "paste")
	cfg := config.Defaults()
	cfg.Inject.Timeout = time.Second
	cfg.Inject.Copy = []string{"sh", "-c", "cat >/dev/null"}
	cfg.Inject.Type = []string{"false"}
	cfg.Inject.Paste = []string{"sh", "-c", "echo pasted > " + paste}

	res, err := Text(context.Background(), cfg, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != "paste" {
		t.Fatalf("method %s", res.Method)
	}
	if _, err := os.Stat(paste); err != nil {
		t.Fatal(err)
	}
}

func TestTextDoesNotWaitOnClipboardOwner(t *testing.T) {
	dir := t.TempDir()
	paste := filepath.Join(dir, "paste")
	cfg := config.Defaults()
	cfg.Inject.Timeout = time.Second
	cfg.Inject.Copy = []string{"sh", "-c", "cat >/dev/null; sleep 30"}
	cfg.Inject.Paste = []string{"sh", "-c", "echo pasted > " + paste}
	cfg.Inject.Type = nil

	start := time.Now()
	res, err := Text(context.Background(), cfg, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != "paste" {
		t.Fatalf("method %s", res.Method)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("blocked on clipboard owner for %s", time.Since(start))
	}
}

func TestTextUsesX11CommandsWhenFocusedX11(t *testing.T) {
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip")
	typed := filepath.Join(dir, "typed")
	cfg := config.Defaults()
	cfg.Inject.Timeout = time.Second
	cfg.Inject.Copy = []string{"false"}
	cfg.Inject.Paste = []string{"false"}
	cfg.Inject.Type = []string{"false"}
	cfg.Inject.XCopy = []string{"sh", "-c", "cat > " + clip}
	cfg.Inject.XPaste = []string{"false"}
	cfg.Inject.XType = []string{"sh", "-c", "cat > " + typed}
	lookupX11 = func() bool { return true }
	t.Cleanup(func() {
		lookupX11 = func() bool { return false }
	})

	res, err := Text(context.Background(), cfg, "qq")
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != "type" {
		t.Fatalf("method %s", res.Method)
	}
	body, err := os.ReadFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "qq" {
		t.Fatalf("clipboard %q", body)
	}
	got, err := os.ReadFile(typed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "qq" {
		t.Fatalf("typed %q", got)
	}
}

func TestFocusedX11MatchesSatelliteComm(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	lookupX11 = nil
	lookupPID = func() (int, bool) { return 2517, true }
	readComm = func(int) (string, error) { return ".xwayland-satel\n", nil }
	t.Cleanup(func() {
		lookupX11 = func() bool { return false }
		lookupPID = nil
		readComm = nil
	})
	if !focusedX11() {
		t.Fatal("expected xwayland-satellite comm to route X11")
	}
}

func TestFocusedX11IgnoresWaylandClient(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	lookupX11 = nil
	lookupPID = func() (int, bool) { return 3423, true }
	readComm = func(int) (string, error) { return "ghostty\n", nil }
	t.Cleanup(func() {
		lookupX11 = func() bool { return false }
		lookupPID = nil
		readComm = nil
	})
	if focusedX11() {
		t.Fatal("wayland client must keep wtype")
	}
}

func TestSessionIsX11(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	if !sessionIsX11() {
		t.Fatal("x11 session")
	}
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("DISPLAY", ":0")
	if sessionIsX11() {
		t.Fatal("wayland session")
	}
	t.Setenv("XDG_SESSION_TYPE", "")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
	if !sessionIsX11() {
		t.Fatal("DISPLAY without WAYLAND_DISPLAY")
	}
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if sessionIsX11() {
		t.Fatal("mixed session defaults to Wayland")
	}
}

func TestTextRejectsEmpty(t *testing.T) {
	_, err := Text(context.Background(), config.Defaults(), "   ")
	if err == nil {
		t.Fatal("expected empty text error")
	}
}
