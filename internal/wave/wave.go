package wave

import "math"

func Raster(w, hgt int, levels []float32, active bool) []byte {
	pix := make([]byte, w*hgt*4)
	rx := float64(hgt) / 2
	for y := range hgt {
		for x := range w {
			if !InPill(float64(x), float64(y), float64(w), float64(hgt), rx) {
				continue
			}
			i := (y*w + x) * 4
			pix[i+0] = 18
			pix[i+1] = 16
			pix[i+2] = 14
			pix[i+3] = 170
		}
	}

	n := len(levels)
	if n == 0 {
		return pix
	}
	padX := 16
	padY := 10
	innerW := w - padX*2
	innerH := hgt - padY*2
	if innerW <= 0 || innerH <= 0 {
		return pix
	}
	barW := float64(innerW) / float64(n)
	fgB, fgG, fgR := byte(210), byte(220), byte(80)
	if active {
		fgB, fgG, fgR = 90, 210, 120
	}
	mid := padY + innerH/2
	for i, rms := range levels {
		amp := math.Sqrt(float64(rms)) * float64(innerH)
		if amp < 2 {
			amp = 2
		}
		if amp > float64(innerH) {
			amp = float64(innerH)
		}
		x0 := padX + int(float64(i)*barW)
		x1 := padX + int(float64(i+1)*barW) - 1
		if x1 <= x0 {
			x1 = x0 + 1
		}
		half := int(amp / 2)
		y0 := mid - half
		y1 := mid + half
		for y := y0; y <= y1; y++ {
			if y < 0 || y >= hgt {
				continue
			}
			for x := x0; x <= x1; x++ {
				if x < 0 || x >= w {
					continue
				}
				if !InPill(float64(x), float64(y), float64(w), float64(hgt), rx) {
					continue
				}
				off := (y*w + x) * 4
				pix[off+0] = fgB
				pix[off+1] = fgG
				pix[off+2] = fgR
				pix[off+3] = 230
			}
		}
	}
	return pix
}

func InPill(x, y, w, h, r float64) bool {
	if y < 0 || y >= h || x < 0 || x >= w {
		return false
	}
	if x >= r && x < w-r {
		return true
	}
	cx := r
	if x >= w-r {
		cx = w - r
	}
	cy := h / 2
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= r*r
}
