package wave

import "testing"

func TestRasterPillOpaqueAndOutsideClear(t *testing.T) {
	pix := Raster(80, 20, []float32{0.25, 0.01}, true)
	if len(pix) != 80*20*4 {
		t.Fatalf("len %d", len(pix))
	}
	if pix[3] != 0 {
		t.Fatalf("corner alpha %d", pix[3])
	}
	cx := (10*80 + 40) * 4
	if pix[cx+3] == 0 {
		t.Fatal("center transparent")
	}
}

func TestInPill(t *testing.T) {
	if !InPill(40, 10, 80, 20, 10) {
		t.Fatal("center")
	}
	if InPill(0, 0, 80, 20, 10) {
		t.Fatal("corner should be outside")
	}
}
