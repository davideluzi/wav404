package sp404sx

// BPM values are stored as BPM × 10 (e.g. 1200 = 120.0 BPM).

const (
	defaultBPM = 1020 // what the converter returns for an empty sample
	targetBPM  = 1200 // the bar count is chosen to land closest to 120.0
	maxBPM     = 2000

	oneBarConst = 105840000 // 600 × 44100 × 4: BPM×10 of one 4/4 bar, divided by frames
	barConst    = 826875    // 0xC9DFB = oneBarConst/128
	barUnit     = 256       // b = bars × 256
	maxBarUnits = 0x96C801
)

// calcBPM returns BPM×10 for a sample of `frames` frames that spans b/256 bars.
func calcBPM(frames, b uint64) int { return int(b * barConst / 2 / frames) }

func absDiff(v int) int {
	if v < targetBPM {
		return targetBPM - v
	}
	return v - targetBPM
}

// ComputeBPM reproduces SP::CSampleParameter::ComputeBpm from Roland's
// SP-404SX Wave Converter: the "original tempo" given to a freshly imported
// sample of the given length (in frames, 44.1 kHz).
func ComputeBPM(frames int) int {
	if frames <= 0 {
		return defaultBPM
	}
	n := uint64(frames)
	x := int(oneBarConst / n)
	if x == targetBPM {
		return targetBPM
	}

	var prev, cur int
	if x < targetBPM {
		// Long sample: try 1, 2, 4, 6, 8, … bars.
		b := uint64(barUnit)
		cur = x
		for {
			prev = cur
			if b == barUnit {
				b = 2 * barUnit
			} else {
				b += 2 * barUnit
			}
			cur = calcBPM(n, b)
			if cur >= targetBPM || b >= maxBarUnits {
				break
			}
		}
		if absDiff(cur) >= absDiff(prev) {
			return prev
		}
		return cur
	}

	// Short sample: try 1/128, 1/64, … 1 bar, then 2, 3, 4, … bars.
	b := uint64(2)
	cur = int(barConst / n)
	prev = cur
	for cur < targetBPM && b < maxBarUnits {
		prev = cur
		if b >= barUnit {
			b += barUnit
		} else {
			b *= 2
		}
		cur = calcBPM(n, b)
	}
	if cur > maxBPM {
		return prev
	}
	if absDiff(cur) < absDiff(prev) {
		return cur
	}
	return prev
}
