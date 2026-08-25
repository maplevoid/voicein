package wave

import "math"

const (
	// Analysis bands stay cheap. Display bars are packed 1px+1px across the HUD.
	BarCount = 24
	barW     = 1
	barGap   = 1
)

func Raster(w, hgt int, levels []float32, active bool) []byte {
	pix := make([]byte, w*hgt*4)
	if w < 2 || hgt < 2 {
		return pix
	}

	n := displayBars(w)
	used := n*barW + (n-1)*barGap
	x0 := (w - used) / 2
	if x0 < 0 {
		x0 = 0
	}

	baseY := hgt - 2
	if baseY < 0 {
		baseY = 0
	}
	for x := x0; x < x0+used && x < w; x++ {
		off := (baseY*w + x) * 4
		pix[off+0] = 255
		pix[off+1] = 255
		pix[off+2] = 255
		pix[off+3] = 90
	}

	for i := range n {
		h := barHeight(hgt, sampleAmp(levels, i, n))
		if h <= 0 {
			continue
		}
		x := x0 + i*(barW+barGap)
		for y := hgt - h; y < hgt; y++ {
			px := x
			if px < 0 || px >= w || y < 0 || y >= hgt {
				continue
			}
			off := (y*w + px) * 4
			pix[off+0] = 255
			pix[off+1] = 255
			pix[off+2] = 255
			pix[off+3] = 235
		}
	}
	return pix
}

func displayBars(w int) int {
	n := (w + barGap) / (barW + barGap)
	if n < 8 {
		return 8
	}
	if n > w {
		return w
	}
	return n
}

func sampleAmp(levels []float32, i, n int) float64 {
	if len(levels) == 0 || n <= 0 {
		return 0
	}
	if n == 1 || len(levels) == 1 {
		return clamp01(float64(levels[0]))
	}
	pos := float64(i) * float64(len(levels)-1) / float64(n-1)
	lo := int(pos)
	if lo >= len(levels)-1 {
		return clamp01(float64(levels[len(levels)-1]))
	}
	t := pos - float64(lo)
	a := float64(levels[lo])*(1-t) + float64(levels[lo+1])*t
	return clamp01(a)
}

func clamp01(a float64) float64 {
	if a < 0.01 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

func barHeight(hgt int, amp float64) int {
	if amp <= 0 {
		return 2
	}
	h := 2 + int(math.Round(amp*float64(hgt-2)))
	if h < 2 {
		return 2
	}
	if h > hgt {
		return hgt
	}
	return h
}
