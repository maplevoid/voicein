package wave

import "testing"

func TestRasterQuietIsShortBaseline(t *testing.T) {
	const w, h = 77, 36
	pix := Raster(w, h, make([]float32, BarCount), false)
	minX, maxX, lit, tall := w, -1, 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pix[(y*w+x)*4+3] == 0 {
				continue
			}
			lit++
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < h-3 {
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
	if maxX-minX < w-8 || maxX-minX > w {
		t.Fatalf("quiet span %d-%d", minX, maxX)
	}
}

func TestRasterLoudBarsBounceAndStayShort(t *testing.T) {
	const w, h = 77, 36
	levels := make([]float32, BarCount)
	for i := range levels {
		levels[i] = 0.12 + 0.75*float32(i%7)/6
	}
	pix := Raster(w, h, levels, true)
	minX, maxX, lit, top := w, -1, 0, 0
	n := displayBars(w)
	x0 := (w - (n*barW + (n-1)*barGap)) / 2
	heights := make([]int, 0, 8)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
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
			if y < h/2 {
				top++
			}
		}
	}
	for i := 0; i < n; i += n / 7 {
		x := x0 + i*(barW+barGap)
		topY := h
		for y := 0; y < h; y++ {
			if pix[(y*w+x)*4+3] != 0 {
				topY = y
				break
			}
		}
		heights = append(heights, h-topY)
	}
	uniq := map[int]struct{}{}
	for _, ht := range heights {
		uniq[ht] = struct{}{}
		if ht >= h {
			t.Fatalf("bar full height %v", heights)
		}
	}
	if lit < 400 {
		t.Fatalf("loud bars too sparse %d", lit)
	}
	if top == 0 {
		t.Fatal("loud bars did not rise")
	}
	if len(uniq) < 3 {
		t.Fatalf("bars did not vary %v", heights)
	}
	if maxX-minX < w-8 || maxX-minX > w {
		t.Fatalf("loud span %d-%d", minX, maxX)
	}
}
