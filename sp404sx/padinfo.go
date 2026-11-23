package sp404sx

import (
	"encoding/binary"
	"fmt"
)

const (
	NumBanks    = 10
	PadsPerBank = 12
	NumPads     = NumBanks * PadsPerBank

	padRecordSize = 32
	padInfoSize   = NumPads * padRecordSize

	// HeaderSize is the fixed size of the WAV header on the card; PCM starts here.
	HeaderSize = 512
)

// FileFormat is byte 21 of a pad record: the type of the sample file on the card.
type FileFormat uint8

const (
	FormatAIFF FileFormat = 0 // A0000001.AIF
	FormatWAV  FileFormat = 1 // A0000001.WAV
)

// Pad is one 32-byte record of PAD_INFO.BIN. Positions are byte offsets
// into the sample file (the PCM starts at HeaderSize).
type Pad struct {
	Start, End         uint32
	UserStart, UserEnd uint32
	Volume             uint8 // 0–127
	Lofi, Loop, Gate   bool
	Reverse            bool
	Format             FileFormat
	Channels           uint8  // 1 or 2
	TempoMode          uint8  // 0–2, device-side; passed through untouched
	OriginalTempo      uint32 // BPM × 10
	UserTempo          uint32 // BPM × 10
}

// EmptyPad returns the record the SP-404SX writes for an unassigned pad.
func EmptyPad() Pad {
	return Pad{
		Start: HeaderSize, End: HeaderSize, UserStart: HeaderSize, UserEnd: HeaderSize,
		Volume: 127, Gate: true, Format: FormatWAV, Channels: 2,
		OriginalTempo: 1200, UserTempo: 1200,
	}
}

// Empty reports whether the pad has no sample data.
func (p Pad) Empty() bool { return p.End <= p.Start }

// Frames is the length of the whole sample in frames.
func (p Pad) Frames() int {
	if p.Empty() || p.Channels == 0 {
		return 0
	}
	return int(p.End-p.Start) / (2 * int(p.Channels))
}

// PadInfo is the content of /ROLAND/SP-404SX/SMPL/PAD_INFO.BIN.
type PadInfo struct {
	Pads [NumPads]Pad
}

// PadIndex maps bank 'A'–'J' and pad 1–12 to 0–119.
func PadIndex(bank byte, pad int) int {
	return int(bank-'A')*PadsPerBank + pad - 1
}

// SampleFileName is the WAV file name for a pad index (e.g. "A0000001.WAV").
func SampleFileName(index int) string {
	return fmt.Sprintf("%c%07d.WAV", 'A'+index/PadsPerBank, index%PadsPerBank+1)
}

func b2u(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// ParsePadInfo decodes PAD_INFO.BIN (120 big-endian records of 32 bytes).
func ParsePadInfo(raw []byte) (*PadInfo, error) {
	if len(raw) != padInfoSize {
		return nil, fmt.Errorf("PAD_INFO.BIN: size %d, expected %d", len(raw), padInfoSize)
	}
	be := binary.BigEndian
	pi := &PadInfo{}
	for i := range pi.Pads {
		r := raw[i*padRecordSize : (i+1)*padRecordSize]
		pi.Pads[i] = Pad{
			Start: be.Uint32(r[0:]), End: be.Uint32(r[4:]),
			UserStart: be.Uint32(r[8:]), UserEnd: be.Uint32(r[12:]),
			Volume: r[16], Lofi: r[17] != 0, Loop: r[18] != 0, Gate: r[19] != 0,
			Reverse: r[20] != 0, Format: FileFormat(r[21]), Channels: r[22], TempoMode: r[23],
			OriginalTempo: be.Uint32(r[24:]), UserTempo: be.Uint32(r[28:]),
		}
	}
	return pi, nil
}

// MarshalBinary encodes the pads back into the PAD_INFO.BIN layout.
func (pi *PadInfo) MarshalBinary() []byte {
	be := binary.BigEndian
	out := make([]byte, padInfoSize)
	for i, p := range pi.Pads {
		r := out[i*padRecordSize : (i+1)*padRecordSize]
		be.PutUint32(r[0:], p.Start)
		be.PutUint32(r[4:], p.End)
		be.PutUint32(r[8:], p.UserStart)
		be.PutUint32(r[12:], p.UserEnd)
		r[16], r[17], r[18], r[19], r[20] = p.Volume, b2u(p.Lofi), b2u(p.Loop), b2u(p.Gate), b2u(p.Reverse)
		r[21], r[22], r[23] = byte(p.Format), p.Channels, p.TempoMode
		be.PutUint32(r[24:], p.OriginalTempo)
		be.PutUint32(r[28:], p.UserTempo)
	}
	return out
}

// TrimFrames returns the user start and end points as frame positions.
func (p Pad) TrimFrames() (start, end int) {
	fb := 2 * int(p.Channels)
	return int(p.UserStart-p.Start) / fb, int(p.UserEnd-p.Start) / fb
}

// SetTrim sets the user start and end points from frame positions
// (0 ≤ start < end ≤ Frames). Like Roland's converter, offsets are rounded
// down to a multiple of 4 bytes (every frame in stereo, every second frame
// in mono); the end of the sample always stays reachable.
func (p *Pad) SetTrim(start, end int) error {
	frames := p.Frames()
	if start < 0 || end > frames || start >= end {
		return fmt.Errorf("trim %d–%d outside the sample (%d frames)", start, end, frames)
	}
	align := func(f int) uint32 {
		if f == frames {
			return p.End
		}
		return p.Start + uint32(f*2*int(p.Channels))&^3
	}
	us, ue := align(start), align(end)
	if ue <= us {
		return fmt.Errorf("trim %d–%d too short", start, end)
	}
	p.UserStart, p.UserEnd = us, ue
	return nil
}

// trimAligned reports whether the user points sit on 4-byte boundaries
// (the end may also be the end of the sample).
func (p Pad) trimAligned() bool {
	return (p.UserStart-p.Start)%4 == 0 && ((p.UserEnd-p.Start)%4 == 0 || p.UserEnd == p.End)
}
