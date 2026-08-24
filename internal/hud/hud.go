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

func (h *HUD) Hide() {
	h.Update(Level{Hide: true})
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

	idle := time.NewTimer(time.Hour)
	idle.Stop()

	for {
		select {
		case <-h.stop:
			return nil
		case l := <-h.in:
			if l.Hide {
				h.mu.Lock()
				h.show = false
				h.mu.Unlock()
				if err := app.hide(); err != nil {
					return err
				}
				idle.Stop()
				continue
			}
			h.apply(l)
			if err := app.redraw(); err != nil {
				return err
			}
			idle.Reset(250 * time.Millisecond)
		case <-idle.C:
			h.mu.Lock()
			h.show = false
			h.mu.Unlock()
			if err := app.hide(); err != nil {
				return err
			}
		}
		if err := roundTrip(display); err != nil {
			return err
		}
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

type shmBuf struct {
	file   *os.File
	data   []byte
	pool   *client.ShmPool
	buf    *client.Buffer
	w, h   int32
	stride int32
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
	mem     shmBuf
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

func (a *app) ensureBuf() error {
	stride := a.width * 4
	if a.mem.buf != nil && a.mem.w == a.width && a.mem.h == a.height {
		return nil
	}
	a.releaseBuf()
	size := int(stride * a.height)
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	f, err := os.CreateTemp(dir, "voicein-shm-*")
	if err != nil {
		return err
	}
	_ = os.Remove(f.Name())
	if err := f.Truncate(int64(size)); err != nil {
		f.Close()
		return err
	}
	data, err := unix.Mmap(int(f.Fd()), 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		f.Close()
		return err
	}
	pool, err := a.shm.CreatePool(int(f.Fd()), int32(size))
	if err != nil {
		_ = unix.Munmap(data)
		f.Close()
		return err
	}
	buf, err := pool.CreateBuffer(0, a.width, a.height, stride, uint32(client.ShmFormatArgb8888))
	if err != nil {
		_ = pool.Destroy()
		_ = unix.Munmap(data)
		f.Close()
		return err
	}
	a.mem = shmBuf{file: f, data: data, pool: pool, buf: buf, w: a.width, h: a.height, stride: stride}
	return nil
}

func (a *app) releaseBuf() {
	if a.mem.buf != nil {
		_ = a.mem.buf.Destroy()
	}
	if a.mem.pool != nil {
		_ = a.mem.pool.Destroy()
	}
	if a.mem.data != nil {
		_ = unix.Munmap(a.mem.data)
	}
	if a.mem.file != nil {
		_ = a.mem.file.Close()
	}
	a.mem = shmBuf{}
}

func (a *app) redraw() error {
	if a.surf == nil || a.shm == nil || a.width <= 0 || a.height <= 0 {
		return nil
	}
	if err := a.ensureBuf(); err != nil {
		return err
	}
	pix := a.hud.raster(int(a.width), int(a.height))
	copy(a.mem.data, pix)
	if err := a.surf.Attach(a.mem.buf, 0, 0); err != nil {
		return err
	}
	if err := a.surf.Damage(0, 0, a.width, a.height); err != nil {
		return err
	}
	return a.surf.Commit()
}

func (a *app) hide() error {
	if a.surf == nil {
		return nil
	}
	if err := a.surf.Attach(nil, 0, 0); err != nil {
		return err
	}
	return a.surf.Commit()
}

func (a *app) cleanup() {
	a.releaseBuf()
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
	return raster(w, hgt, levels, active)
}

func raster(w, hgt int, levels []float32, active bool) []byte {
	pix := make([]byte, w*hgt*4)
	rx := float64(hgt) / 2
	for y := range hgt {
		for x := range w {
			if !inPill(float64(x), float64(y), float64(w), float64(hgt), rx) {
				continue
			}
			i := (y*w + x) * 4
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
