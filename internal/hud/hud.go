package hud

import (
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	"golang.org/x/sys/unix"

	"github.com/zway/voicein/internal/config"
)

type Level struct {
	RMS     float32
	Active  bool
	Seconds int
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

func Start(cfg config.HUD) *HUD {
	h := &HUD{
		cfg:    cfg,
		levels: make([]float32, 48),
		in:     make(chan Level, 16),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	if !cfg.Enabled {
		close(h.done)
		return h
	}
	go func() {
		defer close(h.done)
		if err := h.loop(); err != nil {
			h.failed = err
			log.Printf("hud: %v", err)
		}
	}()
	return h
}

func (h *HUD) Update(l Level) {
	if !h.cfg.Enabled {
		return
	}
	select {
	case h.in <- l:
	default:
	}
}

func (h *HUD) Close() {
	if !h.cfg.Enabled {
		return
	}
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}
	<-h.done
}

func (h *HUD) loop() error {
	display, err := client.Connect("")
	if err != nil {
		return fmt.Errorf("wayland: %w", err)
	}
	defer display.Context().Close()

	app := &app{
		hud:     h,
		display: display,
		width:   int32(h.cfg.Width),
		height:  int32(h.cfg.Height),
		margin:  int32(h.cfg.Margin),
		layer:   parseLayer(h.cfg.Layer),
	}
	if err := app.init(); err != nil {
		return err
	}
	defer app.cleanup()

	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	ctx := display.Context()
	fd, err := waylandFD(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-h.stop:
			return nil
		case l := <-h.in:
			h.apply(l)
			if err := app.redraw(); err != nil {
				return err
			}
		case <-ticker.C:
			if err := app.redraw(); err != nil {
				return err
			}
		default:
		}
		if err := drainWayland(ctx, fd); err != nil {
			return err
		}
		time.Sleep(4 * time.Millisecond)
	}
}

func (h *HUD) apply(l Level) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.levels) == 0 {
		h.levels = make([]float32, 48)
	}
	copy(h.levels, h.levels[1:])
	h.levels[len(h.levels)-1] = l.RMS
	h.active = l.Active
	h.secs = l.Seconds
	h.show = true
}

type app struct {
	hud     *HUD
	display *client.Display
	reg     *client.Registry
	comp    *client.Compositor
	shm     *client.Shm
	shell   *LayerShell
	surf    *client.Surface
	ls      *LayerSurface
	width   int32
	height  int32
	margin  int32
	layer   uint32
	ready   bool
	serial  uint32
}

func (a *app) init() error {
	reg, err := a.display.GetRegistry()
	if err != nil {
		return err
	}
	a.reg = reg
	reg.SetGlobalHandler(a.onGlobal)
	if err := roundTrip(a.display); err != nil {
		return err
	}
	if err := roundTrip(a.display); err != nil {
		return err
	}
	if a.comp == nil || a.shm == nil || a.shell == nil {
		return fmt.Errorf("compositor missing wl_compositor, wl_shm, or zwlr_layer_shell_v1")
	}
	surf, err := a.comp.CreateSurface()
	if err != nil {
		return err
	}
	a.surf = surf
	ls, err := a.shell.GetLayerSurface(surf, nil, a.layer, "voicein")
	if err != nil {
		return err
	}
	a.ls = ls
	ls.SetConfigureHandler(func(ev LayerSurfaceConfigureEvent) {
		a.serial = ev.Serial
		if ev.Width > 0 {
			a.width = int32(ev.Width)
		}
		if ev.Height > 0 {
			a.height = int32(ev.Height)
		}
		a.ready = true
		_ = ls.AckConfigure(ev.Serial)
		_ = a.redraw()
	})
	ls.SetClosedHandler(func() { a.ready = false })
	_ = ls.SetSize(uint32(a.width), uint32(a.height))
	_ = ls.SetAnchor(AnchorBottom)
	_ = ls.SetExclusiveZone(0)
	_ = ls.SetMargin(0, 0, a.margin, 0)
	_ = ls.SetKeyboardInteractivity(KeyboardNone)
	if err := surf.Commit(); err != nil {
		return err
	}
	return roundTrip(a.display)
}

func (a *app) onGlobal(e client.RegistryGlobalEvent) {
	switch e.Interface {
	case "wl_compositor":
		c := client.NewCompositor(a.display.Context())
		_ = a.reg.Bind(e.Name, e.Interface, e.Version, c)
		a.comp = c
	case "wl_shm":
		s := client.NewShm(a.display.Context())
		_ = a.reg.Bind(e.Name, e.Interface, e.Version, s)
		a.shm = s
	case "zwlr_layer_shell_v1":
		s := NewLayerShell(a.display.Context())
		ver := e.Version
		if ver > 4 {
			ver = 4
		}
		_ = a.reg.Bind(e.Name, e.Interface, ver, s)
		a.shell = s
	}
}

func (a *app) redraw() error {
	if a.surf == nil || a.shm == nil || a.width <= 0 || a.height <= 0 {
		return nil
	}
	pix := a.hud.raster(int(a.width), int(a.height))
	stride := a.width * 4
	size := int(stride * a.height)
	f, err := os.CreateTemp(os.Getenv("XDG_RUNTIME_DIR"), "voicein-shm-*")
	if err != nil {
		return err
	}
	defer f.Close()
	_ = os.Remove(f.Name())
	if err := f.Truncate(int64(size)); err != nil {
		return err
	}
	data, err := unix.Mmap(int(f.Fd()), 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return err
	}
	copy(data, pix)
	pool, err := a.shm.CreatePool(int(f.Fd()), int32(size))
	if err != nil {
		_ = unix.Munmap(data)
		return err
	}
	buf, err := pool.CreateBuffer(0, a.width, a.height, stride, uint32(client.ShmFormatArgb8888))
	if err != nil {
		_ = pool.Destroy()
		_ = unix.Munmap(data)
		return err
	}
	buf.SetReleaseHandler(func(_ client.BufferReleaseEvent) {
		_ = buf.Destroy()
		_ = pool.Destroy()
		_ = unix.Munmap(data)
	})
	if err := a.surf.Attach(buf, 0, 0); err != nil {
		return err
	}
	if err := a.surf.Damage(0, 0, a.width, a.height); err != nil {
		return err
	}
	return a.surf.Commit()
}

func (a *app) cleanup() {
	if a.ls != nil {
		_ = a.ls.Destroy()
	}
	if a.surf != nil {
		_ = a.surf.Destroy()
	}
	if a.shell != nil {
		_ = a.shell.Destroy()
	}
	if a.shm != nil {
		_ = a.shm.Destroy()
	}
	if a.comp != nil {
		_ = a.comp.Destroy()
	}
	if a.reg != nil {
		_ = a.reg.Destroy()
	}
	_ = a.display.Destroy()
}

func (h *HUD) raster(w, hgt int) []byte {
	h.mu.Lock()
	levels := append([]float32(nil), h.levels...)
	active := h.active
	h.mu.Unlock()

	pix := make([]byte, w*hgt*4)
	// Fully transparent outside the pill so exclusive-zone 0 never paints a box.
	pillW := w
	pillH := hgt
	rx := float64(hgt) / 2
	for y := range hgt {
		for x := range w {
			if !inPill(float64(x), float64(y), float64(pillW), float64(pillH), rx) {
				continue
			}
			i := (y*w + x) * 4
			// ARGB8888 little-endian = B,G,R,A
			pix[i+0] = 18
			pix[i+1] = 16
			pix[i+2] = 14
			pix[i+3] = 170
		}
	}

	n := len(levels)
	if n == 0 {
		return pix
	}
	padX := 16
	padY := 10
	innerW := w - padX*2
	innerH := hgt - padY*2
	if innerW <= 0 || innerH <= 0 {
		return pix
	}
	barW := float64(innerW) / float64(n)
	fgB, fgG, fgR := byte(210), byte(220), byte(80)
	if active {
		fgB, fgG, fgR = 90, 210, 120
	}
	mid := padY + innerH/2
	for i, rms := range levels {
		amp := math.Sqrt(float64(rms)) * float64(innerH)
		if amp < 2 {
			amp = 2
		}
		if amp > float64(innerH) {
			amp = float64(innerH)
		}
		x0 := padX + int(float64(i)*barW)
		x1 := padX + int(float64(i+1)*barW) - 1
		if x1 <= x0 {
			x1 = x0 + 1
		}
		half := int(amp / 2)
		y0 := mid - half
		y1 := mid + half
		for y := y0; y <= y1; y++ {
			if y < 0 || y >= hgt {
				continue
			}
			for x := x0; x <= x1; x++ {
				if x < 0 || x >= w {
					continue
				}
				if !inPill(float64(x), float64(y), float64(w), float64(hgt), rx) {
					continue
				}
				off := (y*w + x) * 4
				pix[off+0] = fgB
				pix[off+1] = fgG
				pix[off+2] = fgR
				pix[off+3] = 230
			}
		}
	}
	return pix
}

func inPill(x, y, w, h, r float64) bool {
	if y < 0 || y >= h || x < 0 || x >= w {
		return false
	}
	if x >= r && x < w-r {
		return true
	}
	cx := r
	if x >= w-r {
		cx = w - r
	}
	cy := h / 2
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= r*r
}

func parseLayer(s string) uint32 {
	if s == "top" {
		return LayerTop
	}
	return LayerOverlay
}

func roundTrip(d *client.Display) error {
	cb, err := d.Sync()
	if err != nil {
		return err
	}
	done := false
	cb.SetDoneHandler(func(_ client.CallbackDoneEvent) { done = true })
	for !done {
		if err := d.Context().Dispatch(); err != nil {
			return err
		}
	}
	return cb.Destroy()
}

func drainWayland(ctx *client.Context, fd int) error {
	for {
		var pfd unix.PollFd
		pfd.Fd = int32(fd)
		pfd.Events = unix.POLLIN
		n, err := unix.Poll([]unix.PollFd{pfd}, 0)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return err
		}
		if n == 0 {
			return nil
		}
		if err := ctx.Dispatch(); err != nil {
			return err
		}
	}
}

func waylandFD(ctx *client.Context) (int, error) {
	type fdder interface{ Fd() uintptr }
	// go-wayland hides the unix conn. Peek via Display Sync is enough for
	// correctness; for poll we recover the fd from the net.Conn if exposed.
	// Fallback: Dispatch is nonblocking-ish after Sync; use a dummy poll skip.
	_ = fdder(nil)
	return -1, nil
}
