package wave

import "testing"

func TestRasterWaveIsThinAndInterior(t *testing.T) {
	levels := make([]float32, 16)
	for i := range levels {
		levels[i] = 0.04
	}
	pix := Raster(80, 20, levels, true)
	if len(pix) != 80*20*4 {
		t.Fatalf("len %d", len(pix))
	}
	if pix[3] != 0 {
		t.Fatalf("corner alpha %d", pix[3])
	}
	var lit int
	for i := 3; i < len(pix); i += 4 {
		if pix[i] > 0 {
			lit++
		}
	}
	if lit < 80 {
		t.Fatalf("wave too sparse %d", lit)
	}
	if lit > 80*6 {
		t.Fatalf("wave too thick %d", lit)
	}
}

func TestRasterQuietStillDraws(t *testing.T) {
	pix := Raster(64, 16, make([]float32, 8), false)
	var lit int
	for i := 3; i < len(pix); i += 4 {
		if pix[i] > 0 {
			lit++
		}
	}
	if lit < 48 {
		t.Fatalf("idle wave missing %d", lit)
	}
}
