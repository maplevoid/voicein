package wave

import "testing"

func TestRasterQuietIsAlmostEmpty(t *testing.T) {
	pix := Raster(72, 18, make([]float32, 12), false)
	if len(pix) != 72*18*4 {
		t.Fatalf("len %d", len(pix))
	}
	var lit, top int
	for y := 0; y < 18; y++ {
		for x := 0; x < 72; x++ {
			if pix[(y*72+x)*4+3] == 0 {
				continue
			}
			lit++
			if y < 16 {
				top++
			}
		}
	}
	if lit == 0 {
		t.Fatal("quiet bars missing")
	}
	if top != 0 {
		t.Fatalf("quiet bars too tall %d", top)
	}
}

func TestRasterLoudBarsGrowUp(t *testing.T) {
	levels := make([]float32, 12)
	for i := range levels {
		if i%2 == 0 {
			levels[i] = 0.04
		} else {
			levels[i] = 0.002
		}
	}
	pix := Raster(72, 18, levels, true)
	var lit, top int
	for y := 0; y < 18; y++ {
		for x := 0; x < 72; x++ {
			a := pix[(y*72+x)*4+3]
			if a == 0 {
				continue
			}
			if pix[(y*72+x)*4] != 255 || pix[(y*72+x)*4+1] != 255 || pix[(y*72+x)*4+2] != 255 {
				t.Fatalf("bar not white at %d,%d", x, y)
			}
			lit++
			if y < 8 {
				top++
			}
		}
	}
	if lit < 40 {
		t.Fatalf("loud bars too sparse %d", lit)
	}
	if top == 0 {
		t.Fatal("loud bars did not rise")
	}
}
