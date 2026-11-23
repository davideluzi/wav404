package sp404sx

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// SampleRate is the only rate the SP-404SX plays.
const SampleRate = 44100

const (
	rlndSize    = 458 // RLND chunk body: "roifspsx", u32 4, u32 pad index, zeros
	rlndVersion = 4
)

// Sample is 16-bit PCM at 44.1 kHz, as stored on the card.
type Sample struct {
	PadIndex int     // 0–119, written into the RLND chunk
	Channels int     // 1 or 2
	PCM      []int16 // interleaved
}

// Frames is the sample length in frames.
func (s *Sample) Frames() int { return len(s.PCM) / s.Channels }

// DecodeSample parses a card WAV (Axxxxxxx.WAV).
func DecodeSample(raw []byte) (*Sample, error) {
	le := binary.LittleEndian
	if len(raw) < HeaderSize || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, errors.New("not a RIFF/WAVE file with a 512-byte header")
	}
	if string(raw[12:16]) != "fmt " || le.Uint16(raw[20:]) != 1 {
		return nil, errors.New("fmt chunk missing or not PCM")
	}
	ch := int(le.Uint16(raw[22:]))
	rate := le.Uint32(raw[24:])
	bits := le.Uint16(raw[34:])
	if (ch != 1 && ch != 2) || rate != SampleRate || bits != 16 {
		return nil, fmt.Errorf("unsupported format: %d ch, %d Hz, %d bit", ch, rate, bits)
	}
	if string(raw[38:42]) != "RLND" || string(raw[46:54]) != "roifspsx" {
		return nil, errors.New("RLND chunk missing")
	}
	if string(raw[504:508]) != "data" {
		return nil, errors.New("data chunk not at offset 504")
	}
	n := int(le.Uint32(raw[508:]))
	if n > len(raw)-HeaderSize {
		return nil, fmt.Errorf("data size %d exceeds file", n)
	}
	s := &Sample{PadIndex: int(le.Uint32(raw[58:])), Channels: ch, PCM: make([]int16, n/2)}
	for i := range s.PCM {
		s.PCM[i] = int16(le.Uint16(raw[HeaderSize+2*i:]))
	}
	return s, nil
}

// Encode produces the card file: 512-byte header with the RLND chunk.
func (s *Sample) Encode() []byte { return s.encode(true) }

// EncodeWAV produces a standard WAV (44-byte header, no Roland chunk).
func (s *Sample) EncodeWAV() []byte { return s.encode(false) }

func (s *Sample) encode(card bool) []byte {
	le := binary.LittleEndian
	header, fmtSize := 44, uint32(16)
	if card {
		header, fmtSize = HeaderSize, 18 // fmt carries cbSize = 0
	}
	out := make([]byte, header+2*len(s.PCM))
	copy(out[0:], "RIFF")
	le.PutUint32(out[4:], uint32(len(out)-8))
	copy(out[8:], "WAVEfmt ")
	le.PutUint32(out[16:], fmtSize)
	le.PutUint16(out[20:], 1) // PCM
	le.PutUint16(out[22:], uint16(s.Channels))
	le.PutUint32(out[24:], SampleRate)
	le.PutUint32(out[28:], uint32(SampleRate*2*s.Channels))
	le.PutUint16(out[32:], uint16(2*s.Channels))
	le.PutUint16(out[34:], 16)
	if card {
		copy(out[38:], "RLND")
		le.PutUint32(out[42:], rlndSize)
		copy(out[46:], "roifspsx")
		le.PutUint32(out[54:], rlndVersion)
		le.PutUint32(out[58:], uint32(s.PadIndex))
	}
	copy(out[header-8:], "data")
	le.PutUint32(out[header-4:], uint32(2*len(s.PCM)))
	for i, v := range s.PCM {
		le.PutUint16(out[header+2*i:], uint16(v))
	}
	return out
}

// NewPad returns the PAD_INFO record the Roland converter creates when this
// sample is imported: whole sample, volume 127, gate on, tempo from
// ComputeBPM. Like the converter, the end offset is rounded down to a
// multiple of 4 bytes.
func (s *Sample) NewPad() Pad {
	end := uint32(HeaderSize + (2*len(s.PCM))&^3)
	bpm := uint32(ComputeBPM(s.Frames()))
	return Pad{
		Start: HeaderSize, End: end, UserStart: HeaderSize, UserEnd: end,
		Volume: 127, Gate: true, Format: FormatWAV, Channels: uint8(s.Channels),
		OriginalTempo: bpm, UserTempo: bpm,
	}
}

// Trim returns the part of the sample between the pad's user start and
// user end points.
func (s *Sample) Trim(p Pad) (*Sample, error) {
	if p.UserStart < HeaderSize || p.UserEnd < p.UserStart {
		return nil, fmt.Errorf("invalid user range %d–%d", p.UserStart, p.UserEnd)
	}
	frameBytes := 2 * s.Channels
	from := int(p.UserStart-HeaderSize) / frameBytes
	to := int(p.UserEnd-HeaderSize) / frameBytes
	if to > s.Frames() {
		return nil, fmt.Errorf("user end %d beyond sample (%d frames)", p.UserEnd, s.Frames())
	}
	pcm := append([]int16(nil), s.PCM[from*s.Channels:to*s.Channels]...)
	return &Sample{PadIndex: s.PadIndex, Channels: s.Channels, PCM: pcm}, nil
}
