package sp404sx

import (
	"bytes"
	"os"
	"testing"
)

func TestDecodeSampleStereo(t *testing.T) {
	raw, err := os.ReadFile("testdata/J0000005.WAV")
	if err != nil {
		t.Fatal(err)
	}
	s, err := DecodeSample(raw)
	if err != nil {
		t.Fatalf("DecodeSample: %v", err)
	}
	if s.Channels != 2 || s.PadIndex != PadIndex('J', 5) {
		t.Errorf("channels=%d pad=%d", s.Channels, s.PadIndex)
	}
	if len(s.PCM) != 118916/2 {
		t.Errorf("len(PCM) = %d samples, want %d", len(s.PCM), 118916/2)
	}
	if s.Frames() != 118916/4 {
		t.Errorf("Frames() = %d", s.Frames())
	}
}

func TestSampleRoundTrip(t *testing.T) {
	for _, name := range []string{"J0000005.WAV", "A0000010.WAV"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			s, err := DecodeSample(raw)
			if err != nil {
				t.Fatal(err)
			}
			if out := s.Encode(); !bytes.Equal(out, raw) {
				t.Errorf("Encode(Decode(%s)) differs from the original (len %d vs %d)", name, len(out), len(raw))
			}
		})
	}
}

func TestDecodeSampleRejectsGarbage(t *testing.T) {
	if _, err := DecodeSample([]byte("RIFF....WAVEjunk")); err == nil {
		t.Error("expected error")
	}
}

func TestSamplePadRecord(t *testing.T) {
	// A freshly imported sample gets the converter's default parameters.
	s := &Sample{PadIndex: 0, Channels: 2, PCM: make([]int16, 118916/2)}
	want := Pad{
		Start: 512, End: 512 + 118916, UserStart: 512, UserEnd: 512 + 118916,
		Volume: 127, Gate: true, Format: FormatWAV, Channels: 2,
		Tempo: 890, TempoCopy: 890,
	}
	if got := s.NewPad(); got != want {
		t.Errorf("NewPad = %+v\nwant %+v", got, want)
	}
}

// Roland's converter rounds every PAD_INFO offset down to a multiple of 4
// bytes; a mono sample with an odd frame count ends 2 bytes before the file.
func TestNewPadRoundsOffsetsLikeRoland(t *testing.T) {
	s := &Sample{Channels: 1, PCM: make([]int16, 3043)}
	p := s.NewPad()
	if p.End != 512+6084 || p.UserEnd != 512+6084 {
		t.Errorf("End=%d UserEnd=%d, want %d", p.End, p.UserEnd, 512+6084)
	}
}
