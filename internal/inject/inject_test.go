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

func TestTextCopiesThenPastes(t *testing.T) {
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip")
	paste := filepath.Join(dir, "paste")
	cfg := config.Defaults()
	cfg.Inject.Timeout = time.Second
	cfg.Inject.Copy = []string{"sh", "-c", "cat > " + clip}
	cfg.Inject.Paste = []string{"sh", "-c", "echo pasted > " + paste}
	cfg.Inject.Type = nil

	res, err := Text(context.Background(), cfg, "  你好世界  ")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Copied || res.Method != "paste" {
		t.Fatalf("result %+v", res)
	}
	body, err := os.ReadFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "你好世界" {
		t.Fatalf("clipboard %q", body)
	}
	if _, err := os.Stat(paste); err != nil {
		t.Fatal(err)
	}
}

func TestTextFallsBackToType(t *testing.T) {
	dir := t.TempDir()
	typed := filepath.Join(dir, "typed")
	cfg := config.Defaults()
	cfg.Inject.Timeout = time.Second
	cfg.Inject.Copy = []string{"sh", "-c", "cat >/dev/null"}
	cfg.Inject.Paste = []string{"false"}
	cfg.Inject.Type = []string{"sh", "-c", "cat > " + typed}

	res, err := Text(context.Background(), cfg, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != "type" {
		t.Fatalf("method %s", res.Method)
	}
	body, err := os.ReadFile(typed)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "abc" {
		t.Fatalf("typed %q", body)
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
	paste := filepath.Join(dir, "paste")
	cfg := config.Defaults()
	cfg.Inject.Timeout = time.Second
	cfg.Inject.Copy = []string{"false"}
	cfg.Inject.Paste = []string{"false"}
	cfg.Inject.Type = nil
	cfg.Inject.XCopy = []string{"sh", "-c", "cat > " + clip}
	cfg.Inject.XPaste = []string{"sh", "-c", "echo pasted > " + paste}
	cfg.Inject.XType = nil
	lookupX11 = func() bool { return true }
	t.Cleanup(func() {
		lookupX11 = func() bool { return false }
	})

	res, err := Text(context.Background(), cfg, "qq")
	if err != nil {
		t.Fatal(err)
	}
	if res.Method != "paste" {
		t.Fatalf("method %s", res.Method)
	}
	body, err := os.ReadFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "qq" {
		t.Fatalf("clipboard %q", body)
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
