package wave

import "testing"

func TestRasterQuietIsShortBaseline(t *testing.T) {
	pix := Raster(36, 16, make([]float32, 16), false)
	minX, maxX, lit, tall := 36, -1, 0, 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 36; x++ {
			if pix[(y*36+x)*4+3] == 0 {
				continue
			}
			lit++
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < 15 {
				tall++
			}
		}
	}
	if lit == 0 {
		t.Fatal("quiet bars missing")
	}
	if tall != 0 {
		t.Fatalf("quiet bars too tall %d", tall)
	}
	if maxX-minX > 28 {
		t.Fatalf("quiet span too wide %d-%d", minX, maxX)
	}
}

func TestRasterLoudBarsBounceAndStayShort(t *testing.T) {
	levels := []float32{0, 0, 0, 0, 0, 0, 0, 0, 0.05, 0.002, 0.03, 0.008, 0.04, 0.001, 0.02}
	pix := Raster(36, 16, levels, true)
	minX, maxX, lit, top := 36, -1, 0, 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 36; x++ {
			off := (y*36 + x) * 4
			if pix[off+3] == 0 {
				continue
			}
			if pix[off] != 255 || pix[off+1] != 255 || pix[off+2] != 255 {
				t.Fatalf("bar not white at %d,%d", x, y)
			}
			lit++
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < 8 {
				top++
			}
		}
	}
	if lit < 20 {
		t.Fatalf("loud bars too sparse %d", lit)
	}
	if top == 0 {
		t.Fatal("loud bars did not rise")
	}
	if maxX-minX > 28 {
		t.Fatalf("loud span too wide %d-%d", minX, maxX)
	}
}
