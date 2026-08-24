package wave

import "math"

const (
	barCount = 7
	barW     = 2
	barGap   = 2
)

func Raster(w, hgt int, levels []float32, active bool) []byte {
	pix := make([]byte, w*hgt*4)
	if w < 2 || hgt < 2 {
		return pix
	}

	used := barCount*barW + (barCount-1)*barGap
	x0 := (w - used) / 2
	if x0 < 0 {
		x0 = 0
	}

	for i := range barCount {
		amp := sampleAmp(levels, i)
		h := barHeight(hgt, amp)
		if h <= 0 {
			continue
		}
		x := x0 + i*(barW+barGap)
		for y := hgt - h; y < hgt; y++ {
			for dx := range barW {
				px := x + dx
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
	}
	return pix
}

func sampleAmp(levels []float32, i int) float64 {
	n := len(levels)
	if n == 0 {
		return 0
	}
	idx := n - barCount + i
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	a := math.Sqrt(float64(levels[idx])) * 9
	if a < 0.07 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

func barHeight(hgt int, amp float64) int {
	if amp <= 0 {
		return 1
	}
	h := 1 + int(math.Round(amp*float64(hgt-1)))
	if h < 1 {
		return 1
	}
	if h > hgt {
		return hgt
	}
	return h
}
