package wave

import "math"

func Raster(w, hgt int, levels []float32, active bool) []byte {
	pix := make([]byte, w*hgt*4)
	if w < 2 || hgt < 2 {
		return pix
	}

	fgB, fgG, fgR := byte(170), byte(190), byte(80)
	if active {
		fgB, fgG, fgR = 70, 220, 130
	}

	mid := float64(hgt) * 0.5
	maxAmp := float64(hgt) * 0.36
	if maxAmp < 2 {
		maxAmp = 2
	}

	for x := range w {
		t := 0.0
		if w > 1 {
			t = float64(x) / float64(w-1)
		}
		env := 0.0
		if len(levels) > 0 {
			env = math.Sqrt(interp(levels, t))
		}
		gain := env * 7
		if gain > 1 {
			gain = 1
		}
		amp := maxAmp * (0.16 + 0.84*gain)
		phase := t*2*math.Pi*5 + env*2
		shape := math.Sin(phase) + 0.3*math.Sin(phase*2+0.8)
		if shape > 1 {
			shape = 1
		} else if shape < -1 {
			shape = -1
		}
		yf := mid + amp*shape
		for y := range hgt {
			d := math.Abs(float64(y)+0.5 - yf)
			if d >= 1.25 {
				continue
			}
			a := (1.25 - d) / 1.25
			i := (y*w + x) * 4
			pix[i+0] = fgB
			pix[i+1] = fgG
			pix[i+2] = fgR
			pix[i+3] = byte(50 + a*180)
		}
	}
	return pix
}

func interp(levels []float32, t float64) float64 {
	n := len(levels)
	if n == 1 {
		return float64(levels[0])
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	x := t * float64(n-1)
	i := int(x)
	if i >= n-1 {
		return float64(levels[n-1])
	}
	f := x - float64(i)
	f = f * f * (3 - 2*f)
	return float64(levels[i])*(1-f) + float64(levels[i+1])*f
}
