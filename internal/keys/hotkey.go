package keys

import (
	"fmt"
	"strings"
)

// Chord is a modifier set plus one non-modifier key.
// Left/right variants of the same modifier both count.
type Chord struct {
	Shift bool
	Ctrl  bool
	Alt   bool
	Super bool
	Key   uint16
}

func (c Chord) Empty() bool {
	return c.Key == 0
}

// ParseHotkey accepts strings like "shift+alt+v". Empty means no chord.
func ParseHotkey(s string) (Chord, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Chord{}, nil
	}
	var c Chord
	for _, part := range strings.Split(strings.ToLower(s), "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			return Chord{}, fmt.Errorf("empty token in %q", s)
		}
		switch part {
		case "shift", "lshift", "rshift":
			c.Shift = true
		case "ctrl", "control", "lctrl", "rctrl":
			c.Ctrl = true
		case "alt", "lalt", "ralt", "mod1":
			c.Alt = true
		case "super", "meta", "win", "mod", "mod4":
			c.Super = true
		default:
			code, ok := keyName[part]
			if !ok {
				return Chord{}, fmt.Errorf("unknown key %q", part)
			}
			if c.Key != 0 {
				return Chord{}, fmt.Errorf("multiple keys in %q", s)
			}
			c.Key = code
		}
	}
	if c.Key == 0 {
		return Chord{}, fmt.Errorf("hotkey %q has no key", s)
	}
	return c, nil
}

func (c Chord) Match(down map[uint16]struct{}) bool {
	if c.Key == 0 {
		return false
	}
	if _, ok := down[c.Key]; !ok {
		return false
	}
	if c.Shift && !anyDown(down, 42, 54) {
		return false
	}
	if c.Ctrl && !anyDown(down, 29, 97) {
		return false
	}
	if c.Alt && !anyDown(down, 56, 100) {
		return false
	}
	if c.Super && !anyDown(down, 125, 126) {
		return false
	}
	return true
}

func anyDown(down map[uint16]struct{}, codes ...uint16) bool {
	for _, code := range codes {
		if _, ok := down[code]; ok {
			return true
		}
	}
	return false
}

func keySet(codes []uint16) map[uint16]struct{} {
	m := make(map[uint16]struct{}, len(codes))
	for _, code := range codes {
		m[code] = struct{}{}
	}
	return m
}

// Linux input-event-codes.h, enough for a PTT chord.
var keyName = map[string]uint16{
	"esc": 1, "escape": 1, "1": 2, "2": 3, "3": 4, "4": 5, "5": 6,
	"6": 7, "7": 8, "8": 9, "9": 10, "0": 11, "minus": 12, "equal": 13,
	"backspace": 14, "tab": 15,
	"q": 16, "w": 17, "e": 18, "r": 19, "t": 20, "y": 21, "u": 22,
	"i": 23, "o": 24, "p": 25, "a": 30, "s": 31, "d": 32, "f": 33,
	"g": 34, "h": 35, "j": 36, "k": 37, "l": 38, "z": 44, "x": 45,
	"c": 46, "v": 47, "b": 48, "n": 49, "m": 50,
	"enter": 28, "return": 28, "space": 57,
	"comma": 51, "dot": 52, "period": 52, "slash": 53,
	"f1": 59, "f2": 60, "f3": 61, "f4": 62, "f5": 63, "f6": 64,
	"f7": 65, "f8": 66, "f9": 67, "f10": 68, "f11": 87, "f12": 88,
}
