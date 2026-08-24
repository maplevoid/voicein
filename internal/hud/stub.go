//go:build !linux

package hud

import "github.com/zway/voicein/internal/config"

func Start(cfg config.HUD) *HUD {
	h := &HUD{
		cfg:  cfg,
		in:   make(chan Level, 1),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	close(h.done)
	return h
}
