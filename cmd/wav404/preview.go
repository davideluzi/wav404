package main

import (
	"encoding/binary"
	"math"

	"github.com/davideluzi/wav404/sp404sx"
)

// previewPCM renders a pad roughly as the SP-404SX plays it: only the user
// start/end range, reversed if set, scaled by the pad volume (linear
// vol/127, an approximation of the device). The result is always stereo
// 16-bit little-endian, the format of the shared audio output. Lo-fi and
// looping are not rendered.
func previewPCM(s *sp404sx.Sample, p sp404sx.Pad) []byte {
	if t, err := s.Trim(p); err == nil {
		s = t
	} // a corrupt range plays the whole sample
	frames := s.Frames()
	out := make([]byte, 0, frames*4)
	gain := float64(p.Volume) / 127
	for f := range frames {
		src := f
		if p.Reverse {
			src = frames - 1 - f
		}
		frame := s.PCM[src*s.Channels : (src+1)*s.Channels]
		for c := range 2 {
			v := frame[min(c, s.Channels-1)] // mono → both channels
			out = binary.LittleEndian.AppendUint16(out, uint16(int16(math.Round(float64(v)*gain))))
		}
	}
	return out
}
