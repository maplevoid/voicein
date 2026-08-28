package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetPathPatchesExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `# header
mode = "hybrid"             # keep me
hotkey = "shift+alt+v"

[hud]
enabled = true
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetPath(path, "mode", "hold"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, `mode = "hold"             # keep me`) {
		t.Fatalf("patch:\n%s", s)
	}
	if !strings.Contains(s, `# header`) {
		t.Fatalf("lost comments:\n%s", s)
	}
	t.Setenv("VOICEIN_CONFIG", path)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RecordMode() != "hold" {
		t.Fatalf("mode %q", cfg.Mode)
	}
}

func TestSetPathCreatesSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := SetPath(path, "hud.layer", "top"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "[hud]") || !strings.Contains(string(got), `layer = "top"`) {
		t.Fatalf("created:\n%s", got)
	}
}

func TestGetListUnknown(t *testing.T) {
	cfg := Defaults()
	v, err := Get(cfg, "hotkey")
	if err != nil {
		t.Fatal(err)
	}
	if v != `"shift+alt+v"` {
		t.Fatalf("got %s", v)
	}
	list := List(cfg)
	if !strings.Contains(list, "mode = \"hybrid\"") {
		t.Fatalf("list:\n%s", list)
	}
	if _, err := Get(cfg, "engine"); err == nil {
		t.Fatal("expected unknown key")
	}
}
