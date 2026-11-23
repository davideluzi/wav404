package sp404sx

import "math"

// Band-limited resampling with a Kaiser-windowed sinc.
//
// The cutoff sits at 95% of the lower Nyquist frequency; with 48 zero
// crossings per side and β = 9 the stopband is below about -90 dB, which is
// well under the 16-bit noise floor of the SP-404SX.
const (
	rsZeroCrossings = 48
	rsBeta          = 9.0
	rsCutoff        = 0.95
	rsTableRes      = 512 // filter table entries per zero crossing
)

var rsTable = buildSincTable()

// buildSincTable holds the windowed sinc h(x) for x in [0, rsZeroCrossings],
// sampled rsTableRes times per unit (x measured in zero crossings).
func buildSincTable() []float64 {
	n := rsZeroCrossings*rsTableRes + 2
	t := make([]float64, n)
	norm := bessI0(rsBeta)
	for i := range t {
		x := float64(i) / rsTableRes
		r := x / rsZeroCrossings
		if r > 1 {
			continue
		}
		s := 1.0
		if x != 0 {
			s = math.Sin(math.Pi*x) / (math.Pi * x)
		}
		t[i] = s * bessI0(rsBeta*math.Sqrt(1-r*r)) / norm
	}
	return t
}

// bessI0 is the zeroth-order modified Bessel function of the first kind.
func bessI0(x float64) float64 {
	sum, term := 1.0, 1.0
	for k := 1; k < 64; k++ {
		term *= (x / (2 * float64(k))) * (x / (2 * float64(k)))
		sum += term
		if term < sum*1e-17 {
			break
		}
	}
	return sum
}

func sincAt(x float64) float64 { // x in zero crossings, may be negative
	x = math.Abs(x) * rsTableRes
	i := int(x)
	if i >= len(rsTable)-1 {
		return 0
	}
	f := x - float64(i)
	return rsTable[i] + f*(rsTable[i+1]-rsTable[i])
}

// Resample converts a to the given rate (a itself if the rate already
// matches). The output has round(frames × rate / a.Rate) frames. Samples
// outside the input are treated as silence.
func Resample(a *Audio, rate float64) *Audio {
	if a.Rate == rate {
		return a
	}
	ch := a.Channels
	inFrames := a.Frames()
	outFrames := int(math.Round(float64(inFrames) * rate / a.Rate))
	out := &Audio{Rate: rate, Channels: ch, Samples: make([]float64, outFrames*ch)}

	step := a.Rate / rate                     // input frames per output frame
	fc := rsCutoff * math.Min(1, rate/a.Rate) // cutoff relative to input Nyquist
	half := float64(rsZeroCrossings) / fc     // filter half-width in input frames

	for j := range outFrames {
		t := float64(j) * step
		lo := max(0, int(math.Ceil(t-half)))
		hi := min(inFrames-1, int(math.Floor(t+half)))
		dst := out.Samples[j*ch : (j+1)*ch]
		for k := lo; k <= hi; k++ {
			w := fc * sincAt((t-float64(k))*fc)
			for c, v := range a.Samples[k*ch : (k+1)*ch] {
				dst[c] += w * v
			}
		}
	}
	return out
}
