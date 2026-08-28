package asr

func TrimSpeech(pcm []float32, sampleRate int) []float32 {
	if len(pcm) == 0 || sampleRate <= 0 {
		return pcm
	}
	frame := sampleRate / 50
	if frame < 160 {
		frame = 160
	}
	const floor = float32(0.0008)
	first, last := -1, -1
	for i := 0; i+frame <= len(pcm); i += frame {
		if rms(pcm[i:i+frame]) >= floor {
			if first < 0 {
				first = i
			}
			last = i + frame
		}
	}
	if first < 0 {
		return pcm
	}
	pad := sampleRate / 5
	if first > pad {
		first -= pad
	} else {
		first = 0
	}
	if last+pad < len(pcm) {
		last += pad
	} else {
		last = len(pcm)
	}
	return pcm[first:last]
}

func rms(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return float32(sum / float64(len(samples)))
}
