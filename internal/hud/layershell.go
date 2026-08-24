//go:build linux

package hud

import (
	"github.com/rajveermalviya/go-wayland/wayland/client"
)

// Minimal zwlr_layer_shell_v1 / zwlr_layer_surface_v1 bindings.
// Opcode layout matches wlr-layer-shell-unstable-v1.xml.

const (
	LayerBackground uint32 = 0
	LayerBottom     uint32 = 1
	LayerTop        uint32 = 2
	LayerOverlay    uint32 = 3

	AnchorTop    uint32 = 1
	AnchorBottom uint32 = 2
	AnchorLeft   uint32 = 4
	AnchorRight  uint32 = 8

	KeyboardNone uint32 = 0
)

type LayerShell struct {
	client.BaseProxy
}

func NewLayerShell(ctx *client.Context) *LayerShell {
	s := &LayerShell{}
	ctx.Register(s)
	return s
}

func (s *LayerShell) Destroy() error {
	const opcode = 1
	const n = 8
	var buf [n]byte
	client.PutUint32(buf[0:4], s.ID())
	client.PutUint32(buf[4:8], uint32(n<<16|opcode))
	err := s.Context().WriteMsg(buf[:], nil)
	s.Context().Unregister(s)
	return err
}

func (s *LayerShell) GetLayerSurface(surface *client.Surface, output client.Proxy, layer uint32, namespace string) (*LayerSurface, error) {
	ls := NewLayerSurface(s.Context())
	const opcode = 0
	nsPad := client.PaddedLen(len(namespace) + 1)
	n := 8 + 4 + 4 + 4 + 4 + 4 + nsPad
	buf := make([]byte, n)
	l := 0
	client.PutUint32(buf[l:l+4], s.ID())
	l += 4
	client.PutUint32(buf[l:l+4], uint32(n<<16|opcode))
	l += 4
	client.PutUint32(buf[l:l+4], ls.ID())
	l += 4
	client.PutUint32(buf[l:l+4], surface.ID())
	l += 4
	var outID uint32
	if output != nil {
		outID = output.ID()
	}
	client.PutUint32(buf[l:l+4], outID)
	l += 4
	client.PutUint32(buf[l:l+4], layer)
	l += 4
	client.PutString(buf[l:], namespace, len(namespace)+1)
	err := s.Context().WriteMsg(buf, nil)
	return ls, err
}

type LayerSurface struct {
	client.BaseProxy
	configureHandler func(LayerSurfaceConfigureEvent)
	closedHandler    func()
}

type LayerSurfaceConfigureEvent struct {
	Serial uint32
	Width  uint32
	Height uint32
}

func NewLayerSurface(ctx *client.Context) *LayerSurface {
	s := &LayerSurface{}
	ctx.Register(s)
	return s
}

func (s *LayerSurface) SetConfigureHandler(f func(LayerSurfaceConfigureEvent)) {
	s.configureHandler = f
}

func (s *LayerSurface) SetClosedHandler(f func()) { s.closedHandler = f }

func (s *LayerSurface) SetSize(w, h uint32) error {
	return s.u32x2(0, w, h)
}

func (s *LayerSurface) SetAnchor(anchor uint32) error {
	return s.u32(1, anchor)
}

func (s *LayerSurface) SetExclusiveZone(zone int32) error {
	return s.u32(2, uint32(zone))
}

func (s *LayerSurface) SetMargin(top, right, bottom, left int32) error {
	const opcode = 3
	const n = 8 + 16
	var buf [n]byte
	l := 0
	client.PutUint32(buf[l:l+4], s.ID())
	l += 4
	client.PutUint32(buf[l:l+4], uint32(n<<16|opcode))
	l += 4
	client.PutUint32(buf[l:l+4], uint32(top))
	l += 4
	client.PutUint32(buf[l:l+4], uint32(right))
	l += 4
	client.PutUint32(buf[l:l+4], uint32(bottom))
	l += 4
	client.PutUint32(buf[l:l+4], uint32(left))
	return s.Context().WriteMsg(buf[:], nil)
}

func (s *LayerSurface) SetKeyboardInteractivity(mode uint32) error {
	return s.u32(4, mode)
}

func (s *LayerSurface) AckConfigure(serial uint32) error {
	return s.u32(6, serial)
}

func (s *LayerSurface) Destroy() error {
	const opcode = 7
	const n = 8
	var buf [n]byte
	client.PutUint32(buf[0:4], s.ID())
	client.PutUint32(buf[4:8], uint32(n<<16|opcode))
	err := s.Context().WriteMsg(buf[:], nil)
	s.Context().Unregister(s)
	return err
}

func (s *LayerSurface) u32(opcode int, v uint32) error {
	const n = 12
	var buf [n]byte
	client.PutUint32(buf[0:4], s.ID())
	client.PutUint32(buf[4:8], uint32(n<<16|opcode))
	client.PutUint32(buf[8:12], v)
	return s.Context().WriteMsg(buf[:], nil)
}

func (s *LayerSurface) u32x2(opcode int, a, b uint32) error {
	const n = 16
	var buf [n]byte
	client.PutUint32(buf[0:4], s.ID())
	client.PutUint32(buf[4:8], uint32(n<<16|opcode))
	client.PutUint32(buf[8:12], a)
	client.PutUint32(buf[12:16], b)
	return s.Context().WriteMsg(buf[:], nil)
}

func (s *LayerSurface) Dispatch(opcode uint32, _ int, data []byte) {
	switch opcode {
	case 0:
		if s.configureHandler == nil || len(data) < 12 {
			return
		}
		s.configureHandler(LayerSurfaceConfigureEvent{
			Serial: uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24,
			Width:  uint32(data[4]) | uint32(data[5])<<8 | uint32(data[6])<<16 | uint32(data[7])<<24,
			Height: uint32(data[8]) | uint32(data[9])<<8 | uint32(data[10])<<16 | uint32(data[11])<<24,
		})
	case 1:
		if s.closedHandler != nil {
			s.closedHandler()
		}
	}
}
