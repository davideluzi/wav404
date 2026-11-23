package sp404sx

import (
	"math"
	"testing"
)

func sine(freq, rate float64, frames, ch int, chanMask []bool) *Audio {
	a := &Audio{Rate: rate, Channels: ch, Samples: make([]float64, frames*ch)}
	for i := 0; i < frames; i++ {
		v := 0.5 * math.Sin(2*math.Pi*freq*float64(i)/rate)
		for c := 0; c < ch; c++ {
			if chanMask == nil || chanMask[c] {
				a.Samples[i*ch+c] = v
			}
		}
	}
	return a
}

// maxErrVsSine compares channel c of a with an ideal sine, ignoring `edge`
// frames at each end (filter ramp-up).
func maxErrVsSine(a *Audio, c int, freq float64, edge int) float64 {
	worst := 0.0
	for i := edge; i < a.Frames()-edge; i++ {
		want := 0.5 * math.Sin(2*math.Pi*freq*float64(i)/a.Rate)
		worst = math.Max(worst, math.Abs(a.Samples[i*a.Channels+c]-want))
	}
	return worst
}

func rms(a *Audio, c, edge int) float64 {
	s, n := 0.0, 0
	for i := edge; i < a.Frames()-edge; i++ {
		v := a.Samples[i*a.Channels+c]
		s += v * v
		n++
	}
	return math.Sqrt(s / float64(n))
}

func TestResampleSameRateIsIdentity(t *testing.T) {
	in := sine(1000, 44100, 1000, 2, nil)
	out := Resample(in, 44100)
	if out.Rate != 44100 || len(out.Samples) != len(in.Samples) {
		t.Fatalf("rate=%v len=%d", out.Rate, len(out.Samples))
	}
	for i := range in.Samples {
		if out.Samples[i] != in.Samples[i] {
			t.Fatalf("sample %d changed", i)
		}
	}
}

func TestResampleLength(t *testing.T) {
	cases := []struct {
		rate   float64
		frames int
		want   int
	}{
		{48000, 48000, 44100},
		{22050, 1001, 2002},
		{96000, 1000, 459}, // round(1000 × 44100 / 96000) = round(459.375)
	}
	for _, c := range cases {
		out := Resample(sine(100, c.rate, c.frames, 1, nil), 44100)
		if out.Frames() != c.want {
			t.Errorf("%v Hz, %d frames → %d, want %d", c.rate, c.frames, out.Frames(), c.want)
		}
	}
}

func TestResamplePassbandAccuracy(t *testing.T) {
	for _, rate := range []float64{48000, 22050, 96000, 32000} {
		out := Resample(sine(1000, rate, int(rate/2), 1, nil), 44100)
		if e := maxErrVsSine(out, 0, 1000, 200); e > 1e-3 {
			t.Errorf("%v Hz → 44100: max error %.2e > 1e-3 (-66 dBFS)", rate, e)
		}
	}
}

func TestResampleAntiAliasing(t *testing.T) {
	// 30 kHz is above the 22.05 kHz Nyquist limit of the output: it must
	// be filtered out instead of folding back as a 14.1 kHz alias.
	out := Resample(sine(30000, 96000, 48000, 1, nil), 44100)
	if r := rms(out, 0, 200); r > 0.5/math.Sqrt2*1e-3 {
		t.Errorf("30 kHz residue rms %.2e: anti-aliasing below 60 dB", r)
	}
}

func TestResampleChannelsIndependent(t *testing.T) {
	out := Resample(sine(1000, 48000, 24000, 2, []bool{true, false}), 44100)
	if r := rms(out, 1, 0); r != 0 {
		t.Errorf("silent right channel picked up rms %v", r)
	}
	if e := maxErrVsSine(out, 0, 1000, 200); e > 1e-3 {
		t.Errorf("left channel error %.2e", e)
	}
}
