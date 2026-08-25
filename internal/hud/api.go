package hud

import (
	"sync"

	"github.com/zway/voicein/internal/config"
)

type Level struct {
	RMS     float32
	Bands   []float32
	Active  bool
	Seconds int
	Hide    bool
}

type HUD struct {
	cfg config.HUD

	mu     sync.Mutex
	levels []float32
	active bool
	secs   int
	show   bool

	in     chan Level
	stop   chan struct{}
	done   chan struct{}
	failed error
}

func (h *HUD) Update(l Level) {
	if h == nil || !h.cfg.Enabled {
		return
	}
	select {
	case h.in <- l:
	default:
		select {
		case <-h.in:
		default:
		}
		select {
		case h.in <- l:
		default:
		}
	}
}

func (h *HUD) Hide() {
	h.Update(Level{Hide: true})
}

func (h *HUD) Close() {
	if h == nil || !h.cfg.Enabled {
		return
	}
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}
	<-h.done
}
