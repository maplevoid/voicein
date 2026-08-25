//go:build linux

package hud

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/rajveermalviya/go-wayland/wayland/client"

	"github.com/zway/voicein/internal/config"
	"github.com/zway/voicein/internal/wave"
)

func Start(cfg config.HUD) *HUD {
	h := &HUD{
		cfg:    cfg,
		levels: make([]float32, wave.BarCount),
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
	log.Printf("hud: wayland ready %dx%d layer=%s", app.width, app.height, h.cfg.Layer)
	defer app.cleanup()

	for {
		select {
		case <-h.stop:
			return nil
		case l := <-h.in:
			if l.Hide {
				h.apply(l)
				if err := app.hide(); err != nil {
					return err
				}
				continue
			}
			h.apply(l)
			if err := app.redraw(); err != nil {
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
	if len(h.levels) != wave.BarCount {
		h.levels = make([]float32, wave.BarCount)
	}
	if l.Hide {
		h.show = false
		return
	}
	h.levels = mixBars(h.levels, l.RMS, l.Bands)
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
	busy   bool
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
	bufs    [2]shmBuf
	cur     int
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
	return nil
}

func (a *app) mapSurface() error {
	if a.surf != nil {
		return nil
	}
	surf, err := a.comp.CreateSurface()
	if err != nil {
		return err
	}
	ls, err := a.shell.GetLayerSurface(surf, nil, a.layer, "voicein")
	if err != nil {
		_ = surf.Destroy()
		return err
	}
	a.surf = surf
	a.ls = ls
	a.ready = false
	ls.SetConfigureHandler(func(ev LayerSurfaceConfigureEvent) {
		a.serial = ev.Serial
		// Keep the requested box. A 0x0 configure means "use our size";
		// a stretched configure would hide a 160x48 HUD in a full-width strip.
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
	deadline := time.Now().Add(500 * time.Millisecond)
	for !a.ready && time.Now().Before(deadline) {
		if err := a.display.Context().Dispatch(); err != nil {
			return err
		}
	}
	if !a.ready {
		return fmt.Errorf("layer surface configure timeout")
	}
	return nil
}

func bindGlobal(reg *client.Registry, name uint32, iface string, version uint32, id client.Proxy) error {
	const opcode = 0
	ifacePad := client.PaddedLen(len(iface) + 1)
	n := 8 + 4 + 4 + ifacePad + 4 + 4
	buf := make([]byte, n)
	l := 0
	client.PutUint32(buf[l:l+4], reg.ID())
	l += 4
	client.PutUint32(buf[l:l+4], uint32(n<<16|opcode))
	l += 4
	client.PutUint32(buf[l:l+4], name)
	l += 4
	client.PutString(buf[l:l+(4+ifacePad)], iface, len(iface)+1)
	l += 4 + ifacePad
	client.PutUint32(buf[l:l+4], version)
	l += 4
	client.PutUint32(buf[l:l+4], id.ID())
	return reg.Context().WriteMsg(buf, nil)
}

func (a *app) onGlobal(e client.RegistryGlobalEvent) {
	switch e.Interface {
	case "wl_compositor":
		c := client.NewCompositor(a.display.Context())
		_ = bindGlobal(a.reg, e.Name, e.Interface, e.Version, c)
		a.comp = c
	case "wl_shm":
		s := client.NewShm(a.display.Context())
		_ = bindGlobal(a.reg, e.Name, e.Interface, e.Version, s)
		a.shm = s
	case "zwlr_layer_shell_v1":
		s := NewLayerShell(a.display.Context())
		ver := e.Version
		if ver > 4 {
			ver = 4
		}
		_ = bindGlobal(a.reg, e.Name, e.Interface, ver, s)
		a.shell = s
	}
}

func (a *app) ensureBuf(i int) error {
	mem := &a.bufs[i]
	stride := a.width * 4
	if mem.buf != nil && mem.w == a.width && mem.h == a.height {
		return nil
	}
	releaseOne(mem)
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
	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return err
	}
	pool, err := a.shm.CreatePool(int(f.Fd()), int32(size))
	if err != nil {
		_ = syscall.Munmap(data)
		f.Close()
		return err
	}
	buf, err := pool.CreateBuffer(0, a.width, a.height, stride, uint32(client.ShmFormatArgb8888))
	if err != nil {
		_ = pool.Destroy()
		_ = syscall.Munmap(data)
		f.Close()
		return err
	}
	idx := i
	buf.SetReleaseHandler(func(_ client.BufferReleaseEvent) {
		a.bufs[idx].busy = false
	})
	*mem = shmBuf{file: f, data: data, pool: pool, buf: buf, w: a.width, h: a.height, stride: stride}
	return nil
}

func releaseOne(mem *shmBuf) {
	if mem.buf != nil {
		_ = mem.buf.Destroy()
	}
	if mem.pool != nil {
		_ = mem.pool.Destroy()
	}
	if mem.data != nil {
		_ = syscall.Munmap(mem.data)
	}
	if mem.file != nil {
		_ = mem.file.Close()
	}
	*mem = shmBuf{}
}

func (a *app) releaseBuf() {
	releaseOne(&a.bufs[0])
	releaseOne(&a.bufs[1])
	a.cur = 0
}

func (a *app) redraw() error {
	if a.shm == nil || a.width <= 0 || a.height <= 0 {
		return nil
	}
	if err := a.mapSurface(); err != nil {
		return err
	}
	next := 1 - a.cur
	if err := a.ensureBuf(next); err != nil {
		return err
	}
	mem := &a.bufs[next]
	if mem.busy {
		if err := a.ensureBuf(a.cur); err != nil {
			return err
		}
		if !a.bufs[a.cur].busy {
			next = a.cur
			mem = &a.bufs[next]
		} else {
			return nil
		}
	}
	pix := a.hud.raster(int(a.width), int(a.height))
	copy(mem.data, pix)
	if err := a.surf.Attach(mem.buf, 0, 0); err != nil {
		return err
	}
	if err := a.surf.Damage(0, 0, a.width, a.height); err != nil {
		return err
	}
	mem.busy = true
	a.cur = next
	return a.surf.Commit()
}

func (a *app) hide() error {
	a.releaseBuf()
	if a.ls != nil {
		_ = a.ls.Destroy()
		a.ls = nil
	}
	if a.surf != nil {
		_ = a.surf.Destroy()
		a.surf = nil
	}
	a.ready = false
	return nil
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
	return wave.Raster(w, hgt, levels, active)
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
