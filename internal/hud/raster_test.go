package hud

import "testing"

func TestRasterPillOpaqueAndOutsideClear(t *testing.T) {
	pix := raster(80, 20, []float32{0.25, 0.01}, true)
	if len(pix) != 80*20*4 {
		t.Fatalf("len %d", len(pix))
	}
	// Corner outside the capsule stays fully transparent.
	if pix[3] != 0 {
		t.Fatalf("corner alpha %d", pix[3])
	}
	// Center of the pill is painted.
	cx := (10*80 + 40) * 4
	if pix[cx+3] == 0 {
		t.Fatal("center transparent")
	}
}

func TestInPill(t *testing.T) {
	if !inPill(40, 10, 80, 20, 10) {
		t.Fatal("center")
	}
	if inPill(0, 0, 80, 20, 10) {
		t.Fatal("corner should be outside")
	}
}
