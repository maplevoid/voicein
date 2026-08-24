package inject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zway/voicein/internal/config"
)

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

func TestTextRejectsEmpty(t *testing.T) {
	_, err := Text(context.Background(), config.Defaults(), "   ")
	if err == nil {
		t.Fatal("expected empty text error")
	}
}
