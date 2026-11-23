package sp404sx

import (
	"errors"
	"fmt"
	"math"
)

// MinDuration is the shortest sample the Roland converter accepts (seconds).
const MinDuration = 0.069

// maxFileSize: PAD_INFO offsets must stay below 2^31 (CSampleParameter::ClassInvariant).
const maxFileSize = 0x7FFFFFFF

var (
	ErrTooShort = errors.New("sample shorter than 0.069 s")
	ErrTooLong  = errors.New("sample too long for the SP-404SX")
	ErrChannels = errors.New("only mono or stereo files can be imported")
)

// ImportAudio turns an uncompressed WAV/AIFF file into a card sample:
// 44.1 kHz, 16 bit, original channel count (mono or stereo).
// Like Roland's converter, loop points in the file are ignored.
//
// Files that are already 16-bit / 44.1 kHz are copied sample by sample
// (byte-swapped for big-endian AIFF); everything else is decoded to float,
// resampled if needed and rounded to 16 bit without dither.
func ImportAudio(raw []byte) (*Sample, error) {
	p, err := parseAudio(raw)
	if err != nil {
		return nil, err
	}
	if p.channels != 1 && p.channels != 2 {
		return nil, fmt.Errorf("%w (file has %d)", ErrChannels, p.channels)
	}
	s, ok := directSample(p)
	if !ok {
		s = audioToSample(decodePCM(p))
	}
	if err := checkLength(s); err != nil {
		return nil, err
	}
	return s, nil
}

// directSample copies 16-bit / 44.1 kHz PCM without conversion.
// ok is false for any other format.
func directSample(p *pcmData) (s *Sample, ok bool) {
	if p.rate != SampleRate || p.sf.bits != 16 || p.sf.float {
		return nil, false
	}
	n := len(p.data) / 2
	n -= n % p.channels
	pcm := make([]int16, n)
	for i := range pcm {
		pcm[i] = int16(p.sf.order.Uint16(p.data[2*i:]))
	}
	return &Sample{Channels: p.channels, PCM: pcm}, true
}

// checkLength applies the converter's length limits.
func checkLength(s *Sample) error {
	if float64(s.Frames())/SampleRate < MinDuration {
		return ErrTooShort
	}
	if HeaderSize+2*int64(len(s.PCM)) > maxFileSize {
		return ErrTooLong
	}
	return nil
}

// audioToSample resamples to 44.1 kHz if needed and rounds to 16 bit,
// clipping out-of-range values. No dither, as in the Roland converter.
func audioToSample(a *Audio) *Sample {
	if a.Rate != SampleRate {
		a = Resample(a, SampleRate)
	}
	pcm := make([]int16, len(a.Samples))
	for i, v := range a.Samples {
		x := math.Round(v * 32768)
		switch {
		case x > math.MaxInt16:
			x = math.MaxInt16
		case x < math.MinInt16:
			x = math.MinInt16
		case math.IsNaN(x):
			x = 0
		}
		pcm[i] = int16(x)
	}
	return &Sample{Channels: a.Channels, PCM: pcm}
}
