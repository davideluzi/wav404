// Command genfixtures writes the synthetic test fixtures in sp404sx/testdata:
// two card samples and a PAD_INFO.BIN. The output is deterministic.
//
//	go run ./tools/genfixtures
//	tools/genfixtures/import-fixtures.sh   # macOS: WAV/AIFF variants via afconvert
//
// The samples keep the properties the tests rely on: J5 is stereo,
// 29729 frames (89.0 BPM by ComputeBPM); A10 is mono, 26680 frames
// (99.1 BPM). They contain only tones below 10 kHz, so resampling tests
// measure the filter, not the loss of content near Nyquist.
package main

import (
	"log"
	"math"
	"os"
	"path/filepath"

	"github.com/davideluzi/wav404/sp404sx"
)

const dir = "sp404sx/testdata"

func main() {
	j5 := &sp404sx.Sample{PadIndex: sp404sx.PadIndex('J', 5), Channels: 2, PCM: stereoHit(29729)}
	a10 := &sp404sx.Sample{PadIndex: sp404sx.PadIndex('A', 10), Channels: 1, PCM: monoBlip(26680)}
	write(sp404sx.SampleFileName(j5.PadIndex), j5.Encode())
	write(sp404sx.SampleFileName(a10.PadIndex), a10.Encode())
	write("PAD_INFO.BIN", padInfo(j5, a10).MarshalBinary())
}

func write(name string, data []byte) {
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		log.Fatal(err)
	}
}

func clip16(v float64) int16 {
	return int16(math.Max(-32768, math.Min(32767, math.Round(v*32767))))
}

// stereoHit: a decaying low tone plus a softer harmonic, different per side.
func stereoHit(frames int) []int16 {
	pcm := make([]int16, 2*frames)
	for i := range frames {
		t := float64(i) / sp404sx.SampleRate
		env := math.Exp(-5 * t)
		l := 0.55*env*math.Sin(2*math.Pi*110*t) + 0.2*env*math.Sin(2*math.Pi*440*t)
		r := 0.55*env*math.Sin(2*math.Pi*110*t+0.3) + 0.2*env*math.Sin(2*math.Pi*660*t)
		pcm[2*i], pcm[2*i+1] = clip16(l), clip16(r)
	}
	return pcm
}

// monoBlip: a short 1 kHz blip with a fast decay.
func monoBlip(frames int) []int16 {
	pcm := make([]int16, frames)
	for i := range pcm {
		t := float64(i) / sp404sx.SampleRate
		pcm[i] = clip16(0.6 * math.Exp(-12*t) * math.Sin(2*math.Pi*1000*t))
	}
	return pcm
}

// padInfo: J5 and A10 as freshly imported, plus records without files that
// exercise every field for the round-trip test. A1 stays empty, A2 is
// assigned (the drop tests rely on both).
func padInfo(j5, a10 *sp404sx.Sample) *sp404sx.PadInfo {
	pi := &sp404sx.PadInfo{}
	for i := range pi.Pads {
		pi.Pads[i] = sp404sx.EmptyPad()
	}
	pi.Pads[j5.PadIndex] = j5.NewPad()
	pi.Pads[a10.PadIndex] = a10.NewPad()

	stereo := func(frames uint32) sp404sx.Pad {
		end := sp404sx.HeaderSize + 4*frames
		return sp404sx.Pad{Start: 512, End: end, UserStart: 512, UserEnd: end,
			Volume: 127, Gate: true, Format: sp404sx.FormatWAV, Channels: 2,
			OriginalTempo: 1200, UserTempo: 1200}
	}
	a2 := stereo(882000) // 20 s loop, edited on the "device"
	a2.Volume, a2.Loop, a2.Gate = 97, true, false
	pi.Pads[sp404sx.PadIndex('A', 2)] = a2

	b4 := stereo(274909) // trimmed, reversed, lo-fi, slowed down
	b4.UserStart, b4.UserEnd = 512+4*1000, 512+4*200000
	b4.Reverse, b4.Lofi, b4.OriginalTempo, b4.UserTempo = true, true, 770, 410
	pi.Pads[sp404sx.PadIndex('B', 4)] = b4

	c1 := stereo(117600)
	c1.TempoMode, c1.Volume, c1.OriginalTempo, c1.UserTempo = 1, 99, 900, 900
	pi.Pads[sp404sx.PadIndex('C', 1)] = c1

	for _, n := range []int{1, 2, 3, 4, 6, 7, 8} { // a fuller bank J for the GUI
		pi.Pads[sp404sx.PadIndex('J', n)] = stereo(uint32(29729 * n))
	}
	return pi
}
