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
	Socket     string        `toml:"socket"`
	SampleRate int           `toml:"sample_rate"`
	Silence    time.Duration `toml:"silence"`
	MaxRecord  time.Duration `toml:"max_record"`
	Language   string        `toml:"language"`
	ITN        bool          `toml:"itn"`
	Threads    int           `toml:"threads"`
	Notify     bool          `toml:"notify"`
	Model      Model         `toml:"model"`
	Record     Record        `toml:"record"`
	Inject     Inject        `toml:"inject"`
	HUD        HUD           `toml:"hud"`
}

type Model struct {
	Dir     string `toml:"dir"`
	Engine  string `toml:"engine"`
	Onnx    string `toml:"onnx"`
	Encoder string `toml:"encoder"`
	Decoder string `toml:"decoder"`
	Tokens  string `toml:"tokens"`
	VAD     string `toml:"vad"`
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
	home, _ := os.UserHomeDir()
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = filepath.Join(os.TempDir(), "voicein")
	}
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		dataDir = filepath.Join(home, ".local", "share")
	}
	return Config{
		Socket:     filepath.Join(runtimeDir, "voicein.sock"),
		SampleRate: 16000,
		Silence:    3 * time.Second,
		MaxRecord:  60 * time.Second,
		Language:   "auto",
		ITN:        true,
		Threads:    4,
		Notify:     true,
		Model: Model{
			Dir:    filepath.Join(dataDir, "voicein", "models"),
			Engine: "sensevoice",
			Onnx:   "model.int8.onnx",
			Tokens: "tokens.txt",
			VAD:    "silero_vad.onnx",
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
	if cfg.Threads <= 0 {
		cfg.Threads = 4
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
	if cfg.Model.Dir == "" {
		cfg.Model.Dir = Defaults().Model.Dir
	}
	return cfg, nil
}

func (c Config) EngineKind() string {
	e := strings.ToLower(strings.TrimSpace(c.Model.Engine))
	if e == "" {
		if c.Model.Encoder != "" && c.Model.Decoder != "" {
			return "whisper"
		}
		return "sensevoice"
	}
	return e
}

func (c Config) ModelOnnx() string {
	return resolve(c.Model.Dir, c.Model.Onnx)
}

func (c Config) ModelEncoder() string {
	return resolve(c.Model.Dir, c.Model.Encoder)
}

func (c Config) ModelDecoder() string {
	return resolve(c.Model.Dir, c.Model.Decoder)
}

func (c Config) ModelTokens() string {
	return resolve(c.Model.Dir, c.Model.Tokens)
}

func (c Config) ModelVAD() string {
	return resolve(c.Model.Dir, c.Model.VAD)
}

func resolve(dir, name string) string {
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}

func Example() string {
	return strings.TrimSpace(`
# ~/.config/voicein/config.toml
# Missing file uses built-in defaults.

socket = ""                 # default: $XDG_RUNTIME_DIR/voicein.sock
sample_rate = 16000
silence = "3s"              # unused for auto-stop; keep as think-pause hint
max_record = "60s"          # hard stop
language = "auto"           # auto | zh | en | yue | ja | ko
itn = true
threads = 4
notify = true               # mako/notify-send on failure only

[model]
dir = ""                    # default: $XDG_DATA_HOME/voicein/models
engine = "sensevoice"       # sensevoice | whisper
onnx = "model.int8.onnx"    # sensevoice
encoder = ""                # whisper encoder onnx
decoder = ""                # whisper decoder onnx
tokens = "tokens.txt"
vad = "silero_vad.onnx"

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
