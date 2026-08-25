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
}

func TestLoadOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
sample_rate = 8000
silence = "2s"
max_record = "10s"
language = "zh"
itn = false
threads = 4
notify = false
[model]
engine = "whisper"
encoder = "small-encoder.int8.onnx"
decoder = "small-decoder.int8.onnx"
tokens = "small-tokens.txt"
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
	if cfg.SampleRate != 8000 || cfg.Language != "zh" || cfg.ITN || cfg.Notify {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.EngineKind() != "whisper" || cfg.Model.Encoder != "small-encoder.int8.onnx" || cfg.Model.Decoder != "small-decoder.int8.onnx" {
		t.Fatalf("model %+v", cfg.Model)
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

func TestModelPaths(t *testing.T) {
	cfg := Defaults()
	cfg.Model.Dir = "/models"
	cfg.Model.Onnx = "a.onnx"
	if got := cfg.ModelOnnx(); got != "/models/a.onnx" {
		t.Fatal(got)
	}
	cfg.Model.Onnx = "/abs/model.onnx"
	if got := cfg.ModelOnnx(); got != "/abs/model.onnx" {
		t.Fatal(got)
	}
	cfg.Model.Encoder = "enc.onnx"
	cfg.Model.Decoder = "/abs/dec.onnx"
	if got := cfg.ModelEncoder(); got != "/models/enc.onnx" {
		t.Fatal(got)
	}
	if got := cfg.ModelDecoder(); got != "/abs/dec.onnx" {
		t.Fatal(got)
	}
	if Defaults().EngineKind() != "sensevoice" {
		t.Fatalf("default engine %q", Defaults().EngineKind())
	}
}
