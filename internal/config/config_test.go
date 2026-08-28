package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingUsesDefaults(t *testing.T) {
	t.Setenv("VOICEIN_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SampleRate != 16000 {
		t.Fatalf("sample rate %d", cfg.SampleRate)
	}
	if cfg.Silence != 3*time.Second {
		t.Fatalf("silence %s", cfg.Silence)
	}
	if cfg.HUD.Layer != "overlay" {
		t.Fatalf("layer %q", cfg.HUD.Layer)
	}
	if cfg.RecordMode() != "hybrid" {
		t.Fatalf("mode %q", cfg.Mode)
	}
	if cfg.Tap != 300*time.Millisecond {
		t.Fatalf("tap %s", cfg.Tap)
	}
	if !stringsHasSuffix(cfg.ScribeSocket, "scribe.sock") {
		t.Fatalf("scribe socket %q", cfg.ScribeSocket)
	}
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
sample_rate = 8000
silence = "2s"
max_record = "10s"
notify = false
mode = "hold"
tap = "150ms"
hotkey = "super+shift+v"

[scribe]
socket = "/tmp/scribe-test.sock"
idle = "5m"
[hud]
enabled = false
width = 100
height = 20
margin = 4
layer = "top"

[inject]
x_copy = ["xclip", "-selection", "clipboard"]
x_paste = ["xdotool", "key", "ctrl+v"]
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VOICEIN_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SampleRate != 8000 || cfg.Notify {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.RecordMode() != "hold" {
		t.Fatalf("mode %q", cfg.Mode)
	}
	if cfg.Hotkey != "super+shift+v" {
		t.Fatalf("hotkey %q", cfg.Hotkey)
	}
	if cfg.Tap != 150*time.Millisecond {
		t.Fatalf("tap %s", cfg.Tap)
	}
	if cfg.ScribeSocket != "/tmp/scribe-test.sock" {
		t.Fatalf("scribe socket %q", cfg.ScribeSocket)
	}
	if cfg.Silence != 2*time.Second || cfg.MaxRecord != 10*time.Second {
		t.Fatalf("durations silence=%s max=%s", cfg.Silence, cfg.MaxRecord)
	}
	if cfg.HUD.Enabled || cfg.HUD.Layer != "top" || cfg.HUD.Width != 100 {
		t.Fatalf("hud %+v", cfg.HUD)
	}
	if len(cfg.Inject.XCopy) != 3 || cfg.Inject.XCopy[0] != "xclip" {
		t.Fatalf("x_copy %+v", cfg.Inject.XCopy)
	}
	if len(cfg.Inject.XPaste) != 3 || cfg.Inject.XPaste[0] != "xdotool" {
		t.Fatalf("x_paste %+v", cfg.Inject.XPaste)
	}
}

func TestRecordMode(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "toggle"},
		{"toggle", "toggle"},
		{"hold", "hold"},
		{"ptt", "hold"},
		{"hybrid", "hybrid"},
		{"both", "hybrid"},
		{"tap", "hybrid"},
	}
	for _, tc := range cases {
		cfg := Config{Mode: tc.in}
		if got := cfg.RecordMode(); got != tc.want {
			t.Fatalf("mode %q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
