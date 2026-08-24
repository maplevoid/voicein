package inject

import (
	"bytes"
	"context"
	"fmt"
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
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := runStdin(ctx, cfg.Inject.Copy, text); err != nil {
		return Result{}, fmt.Errorf("clipboard: %w", err)
	}
	res := Result{Copied: true, Method: "clipboard"}

	if len(cfg.Inject.Paste) > 0 {
		if err := run(ctx, cfg.Inject.Paste); err == nil {
			res.Method = "paste"
			return res, nil
		}
	}
	if len(cfg.Inject.Type) > 0 {
		if err := runStdin(ctx, cfg.Inject.Type, text); err == nil {
			res.Method = "type"
			return res, nil
		}
	}
	return res, fmt.Errorf("inject failed after clipboard copy")
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
