package wave

import "math"

func Raster(w, hgt int, levels []float32, active bool) []byte {
	pix := make([]byte, w*hgt*4)
	n := len(levels)
	if w < 2 || hgt < 2 || n == 0 {
		return pix
	}

	const minBar, minGap = 2, 1
	fit := (w + minGap) / (minBar + minGap)
	if fit < 4 {
		fit = 4
	}
	bars := n
	if bars > fit {
		bars = fit
	}
	if bars < 4 {
		bars = n
	}

	gap := minGap
	barW := (w - gap*(bars-1)) / bars
	if barW < 1 {
		barW = 1
	}
	used := bars*barW + (bars-1)*gap
	x0 := (w - used) / 2
	if x0 < 0 {
		x0 = 0
	}

	base := 1
	maxH := hgt - 1
	if maxH < 1 {
		maxH = 1
	}

	for i := range bars {
		amp := barAmp(levels, bars, i)
		h := base + int(math.Round(amp*float64(maxH-base)))
		if h < base {
			h = base
		}
		if h > hgt {
			h = hgt
		}
		x := x0 + i*(barW+gap)
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

func barAmp(levels []float32, bars, i int) float64 {
	n := len(levels)
	start := i * n / bars
	end := (i + 1) * n / bars
	if end <= start {
		end = start + 1
	}
	if end > n {
		end = n
	}
	peak := 0.0
	for _, v := range levels[start:end] {
		a := math.Sqrt(float64(v)) * 8
		if a > peak {
			peak = a
		}
	}
	if peak < 0.04 {
		return 0
	}
	if peak > 1 {
		return 1
	}
	return peak
}
