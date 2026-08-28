package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Socket       string        `toml:"socket"`
	ScribeSocket string        `toml:"-"`
	SampleRate   int           `toml:"sample_rate"`
	Silence      time.Duration `toml:"silence"`
	MaxRecord    time.Duration `toml:"max_record"`
	Notify       bool          `toml:"notify"`
	Mode         string        `toml:"mode"`
	Tap          time.Duration `toml:"tap"`
	Hotkey       string        `toml:"hotkey"`
	Scribe       Scribe        `toml:"scribe"`
	Record       Record        `toml:"record"`
	Inject       Inject        `toml:"inject"`
	HUD          HUD           `toml:"hud"`
}

type Scribe struct {
	Socket string `toml:"socket"`
}

type Record struct {
	Command []string `toml:"command"`
}

type Inject struct {
	Copy    []string `toml:"copy"`
	Paste   []string `toml:"paste"`
	Type    []string `toml:"type"`
	XCopy   []string `toml:"x_copy"`
	XPaste  []string `toml:"x_paste"`
	XType   []string `toml:"x_type"`
	Notify  []string `toml:"notify"`
	Timeout time.Duration
}

type HUD struct {
	Enabled bool   `toml:"enabled"`
	Width   int    `toml:"width"`
	Height  int    `toml:"height"`
	Margin  int    `toml:"margin"`
	Layer   string `toml:"layer"`
}

func Defaults() Config {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), "voicein")
	}
	return Config{
		Socket:       filepath.Join(runtimeDir, "voicein.sock"),
		ScribeSocket: filepath.Join(runtimeDir, "scribe.sock"),
		SampleRate:   16000,
		Silence:      3 * time.Second,
		MaxRecord:    60 * time.Second,
		Notify:       true,
		Mode:         "hybrid",
		Tap:          300 * time.Millisecond,
		Hotkey:       "shift+alt+v",
		Scribe: Scribe{
			Socket: filepath.Join(runtimeDir, "scribe.sock"),
		},
		Record: Record{
			Command: []string{"pw-record", "--rate", "16000", "--channels", "1", "--format", "s16", "--media-role", "Communication", "-"},
		},
		Inject: Inject{
			Copy:    []string{"wl-copy", "--type", "text/plain;charset=utf-8"},
			Paste:   []string{"wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl"},
			Type:    []string{"wtype", "-"},
			XCopy:   []string{"xclip", "-selection", "clipboard"},
			XPaste:  []string{"xdotool", "key", "--clearmodifiers", "ctrl+v"},
			XType:   []string{"xdotool", "type", "--clearmodifiers", "--file", "-"},
			Notify:  []string{"notify-send", "-a", "voicein", "-u", "normal"},
			Timeout: 4 * time.Second,
		},
		HUD: HUD{
			Enabled: true,
			Width:   77,
			Height:  36,
			Margin:  28,
			Layer:   "overlay",
		},
	}
}

func Path() string {
	if p := os.Getenv("VOICEIN_CONFIG"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "voicein", "config.toml")
}

func Load() (Config, error) {
	cfg := Defaults()
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := applyTOML(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 16000
	}
	if cfg.Silence <= 0 {
		cfg.Silence = 3 * time.Second
	}
	if cfg.MaxRecord <= 0 {
		cfg.MaxRecord = 60 * time.Second
	}
	if cfg.Tap <= 0 {
		cfg.Tap = 300 * time.Millisecond
	}
	if cfg.HUD.Width <= 0 {
		cfg.HUD.Width = 77
	}
	if cfg.HUD.Height <= 0 {
		cfg.HUD.Height = 36
	}
	if cfg.HUD.Layer == "" {
		cfg.HUD.Layer = "overlay"
	}
	if cfg.Socket == "" {
		cfg.Socket = Defaults().Socket
	}
	if cfg.Scribe.Socket != "" {
		cfg.ScribeSocket = cfg.Scribe.Socket
	}
	if cfg.ScribeSocket == "" {
		cfg.ScribeSocket = Defaults().ScribeSocket
	}
	return cfg, nil
}

func (c Config) RecordMode() string {
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case "hold", "ptt", "push":
		return "hold"
	case "hybrid", "both", "tap":
		return "hybrid"
	default:
		return "toggle"
	}
}

func Example() string {
	return strings.TrimSpace(`
# ~/.config/voicein/config.toml
# Missing file uses built-in defaults.
# Recognition lives in the scribe daemon (~/.config/scribe/config.toml).

socket = ""                 # default: $XDG_RUNTIME_DIR/voicein.sock
sample_rate = 16000
silence = "3s"              # unused for auto-stop; keep as think-pause hint
max_record = "60s"          # hard stop
notify = true               # mako/notify-send on failure only
mode = "hybrid"             # hybrid | toggle | hold
tap = "300ms"               # hybrid: shorter tap latches; hold releases
hotkey = "shift+alt+v"      # evdev chord; empty disables daemon hotkey

[scribe]
socket = ""                 # default: $XDG_RUNTIME_DIR/scribe.sock

[record]
command = ["pw-record", "--rate", "16000", "--channels", "1", "--format", "s16", "--media-role", "Communication", "-"]

[inject]
copy = ["wl-copy", "--type", "text/plain;charset=utf-8"]
paste = ["wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl"]
type = ["wtype", "-"]
x_copy = ["xclip", "-selection", "clipboard"]
x_paste = ["xdotool", "key", "--clearmodifiers", "ctrl+v"]
x_type = ["xdotool", "type", "--clearmodifiers", "--file", "-"]
notify = ["notify-send", "-a", "voicein", "-u", "normal"]

[hud]
enabled = true
width = 77
height = 36
margin = 28
layer = "overlay"           # overlay | top
`) + "\n"
}
