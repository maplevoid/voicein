package keys

import "testing"

func TestParseHotkey(t *testing.T) {
	c, err := ParseHotkey("Shift+Alt+V")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Shift || !c.Alt || c.Ctrl || c.Super || c.Key != 47 {
		t.Fatalf("%+v", c)
	}
}

func TestParseHotkeyEmpty(t *testing.T) {
	c, err := ParseHotkey("")
	if err != nil || !c.Empty() {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestParseHotkeyRejectsBareModifiers(t *testing.T) {
	if _, err := ParseHotkey("shift+alt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestChordMatch(t *testing.T) {
	c, err := ParseHotkey("shift+alt+v")
	if err != nil {
		t.Fatal(err)
	}
	if !c.Match(map[uint16]struct{}{42: {}, 56: {}, 47: {}}) {
		t.Fatal("left shift+alt+v")
	}
	if !c.Match(map[uint16]struct{}{54: {}, 100: {}, 47: {}}) {
		t.Fatal("right shift+alt+v")
	}
	if c.Match(map[uint16]struct{}{42: {}, 47: {}}) {
		t.Fatal("missing alt")
	}
	if c.Match(map[uint16]struct{}{42: {}, 56: {}}) {
		t.Fatal("missing v")
	}
}
