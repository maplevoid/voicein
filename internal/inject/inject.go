package inject

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/zway/voicein/internal/config"
)

type Result struct {
	Method string
	Copied bool
}

func Text(ctx context.Context, cfg config.Config, text string) (Result, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{}, fmt.Errorf("empty text")
	}
	timeout := cfg.Inject.Timeout
	if timeout <= 0 {
		timeout = 4 * time.Second
	}

	copyCmd, pasteCmd, typeCmd := route(cfg.Inject)
	if err := startStdin(copyCmd, text); err != nil {
		return Result{}, fmt.Errorf("clipboard: %w", err)
	}
	res := Result{Copied: true, Method: "clipboard"}

	if len(pasteCmd) > 0 {
		pctx, cancel := context.WithTimeout(ctx, timeout)
		err := run(pctx, pasteCmd)
		cancel()
		if err == nil {
			res.Method = "paste"
			return res, nil
		}
	}
	if len(typeCmd) > 0 {
		tctx, cancel := context.WithTimeout(ctx, timeout)
		err := runStdin(tctx, typeCmd, text)
		cancel()
		if err == nil {
			res.Method = "type"
			return res, nil
		}
	}
	return res, fmt.Errorf("inject failed after clipboard copy")
}

func route(inj config.Inject) (copyCmd, pasteCmd, typeCmd []string) {
	if focusedX11() && len(inj.XCopy)+len(inj.XPaste)+len(inj.XType) > 0 {
		return inj.XCopy, inj.XPaste, inj.XType
	}
	return inj.Copy, inj.Paste, inj.Type
}

var lookupX11 func() bool

func focusedX11() bool {
	if lookupX11 != nil {
		return lookupX11()
	}
	return sessionIsX11()
}

func sessionIsX11() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE"))) {
	case "x11":
		return true
	case "wayland":
		return false
	}
	return os.Getenv("DISPLAY") != "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

func Notify(ctx context.Context, cfg config.Config, title, body string) {
	if !cfg.Notify || len(cfg.Inject.Notify) == 0 {
		return
	}
	args := append([]string{}, cfg.Inject.Notify[1:]...)
	args = append(args, title, body)
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = exec.CommandContext(c, cfg.Inject.Notify[0], args...).Run()
}

func run(ctx context.Context, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w (%s)", argv[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func runStdin(ctx context.Context, argv []string, stdin string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w (%s)", argv[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// startStdin writes stdin then returns. Commands that stay alive as a
// clipboard owner (wl-copy) are detached; commands that exit after
// consuming stdin are waited on briefly so tests can observe the write.
func startStdin(argv []string, stdin string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("%s: %w", argv[0], err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w (%s)", argv[0], err, strings.TrimSpace(stderr.String()))
	}
	if _, err := io.WriteString(pipe, stdin); err != nil {
		_ = pipe.Close()
		_ = cmd.Process.Kill()
		return fmt.Errorf("%s: %w", argv[0], err)
	}
	_ = pipe.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s: %w (%s)", argv[0], err, strings.TrimSpace(stderr.String()))
		}
		return nil
	case <-time.After(200 * time.Millisecond):
		return nil
	}
}
