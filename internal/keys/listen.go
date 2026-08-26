package keys

import (
	"context"
	"encoding/binary"
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

// Listen reports chord edges until ctx is done.
// down=true on press, false on release. Repeats are ignored.
func Listen(ctx context.Context, chord Chord, emit func(down bool)) error {
	if chord.Empty() {
		<-ctx.Done()
		return ctx.Err()
	}
	devs, err := openKeyboards()
	if err != nil {
		return err
	}
	defer func() {
		for _, d := range devs {
			_ = d.Close()
		}
	}()

	down := keySet(collectPressed(devs))
	armed := chord.Match(down)
	buf := make([]byte, inputEventSize*16)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		pfds := make([]unix.PollFd, len(devs))
		for i, f := range devs {
			pfds[i] = unix.PollFd{Fd: int32(f.Fd()), Events: unix.POLLIN}
		}
		n, err := unix.Poll(pfds, 250)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return err
		}
		if n == 0 {
			continue
		}
		for i, p := range pfds {
			if p.Revents&unix.POLLIN == 0 {
				continue
			}
			nr, err := devs[i].Read(buf)
			if err != nil {
				continue
			}
			for off := 0; off+inputEventSize <= nr; off += inputEventSize {
				typ := binary.LittleEndian.Uint16(buf[off+16 : off+18])
				code := binary.LittleEndian.Uint16(buf[off+18 : off+20])
				val := int32(binary.LittleEndian.Uint32(buf[off+20 : off+24]))
				if typ != evKey {
					continue
				}
				if val == 0 {
					delete(down, code)
				} else {
					down[code] = struct{}{}
				}
				now := chord.Match(down)
				if now == armed {
					continue
				}
				armed = now
				emit(now)
			}
		}
	}
}
