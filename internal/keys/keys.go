package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	evKey          = 0x01
	keyMax         = 0x2ff
	keyCnt         = keyMax + 1
	keyBytes       = (keyCnt + 7) / 8
	evIOCRead      = 2
	iocNRBits      = 8
	iocTypeBits    = 8
	iocSizeBits    = 14
	iocNRShift     = 0
	iocTypeShift   = iocNRShift + iocNRBits
	iocSizeShift   = iocTypeShift + iocTypeBits
	iocDirShift    = iocSizeShift + iocSizeBits
	inputEventSize = 24
)

// EVIOCGKEY(len) = _IOR('E', 0x18, len)
var eviocgkey = uintptr(evIOCRead)<<iocDirShift |
	uintptr(keyBytes)<<iocSizeShift |
	uintptr('E')<<iocTypeShift |
	uintptr(0x18)<<iocNRShift

func openKeyboards() ([]*os.File, error) {
	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}
	var out []*os.File
	for _, path := range matches {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		if !hasEvKey(f) {
			_ = f.Close()
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no readable evdev keyboards")
	}
	return out, nil
}

func hasEvKey(f *os.File) bool {
	var bits [evKey/8 + 1]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), eviocgbit(0, len(bits)), uintptr(unsafe.Pointer(&bits[0])))
	if errno != 0 {
		return false
	}
	return bits[evKey/8]&(1<<(evKey%8)) != 0
}

func eviocgbit(ev, length int) uintptr {
	return uintptr(evIOCRead)<<iocDirShift |
		uintptr(length)<<iocSizeShift |
		uintptr('E')<<iocTypeShift |
		uintptr(0x20+ev)<<iocNRShift
}

func collectPressed(devs []*os.File) []uint16 {
	seen := map[uint16]struct{}{}
	var out []uint16
	for _, f := range devs {
		var bits [keyBytes]byte
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), eviocgkey, uintptr(unsafe.Pointer(&bits[0])))
		if errno != 0 {
			continue
		}
		for code := range keyCnt {
			if bits[code/8]&(1<<(uint(code)%8)) == 0 {
				continue
			}
			c := uint16(code)
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}
