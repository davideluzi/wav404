package sp404sx

import (
	"bytes"
	"testing"
)

func TestEncodeWAVIsPlainAndDecodable(t *testing.T) {
	s := refSample(t)
	out := s.EncodeWAV()
	if len(out) != 44+2*len(s.PCM) {
		t.Errorf("size %d, want canonical 44-byte header + data (%d)", len(out), 44+2*len(s.PCM))
	}
	if bytes.Contains(out[:44], []byte("RLND")) {
		t.Error("exported WAV still contains the Roland chunk")
	}
	a, err := DecodeAudio(out)
	if err != nil {
		t.Fatal(err)
	}
	if a.Rate != 44100 || a.Channels != 2 || len(a.Samples) != len(s.PCM) {
		t.Fatalf("rate=%v ch=%d n=%d", a.Rate, a.Channels, len(a.Samples))
	}
	for i, v := range s.PCM {
		if a.Samples[i] != float64(v)/32768 {
			t.Fatalf("sample %d differs", i)
		}
	}
}

func TestTrimToUserRange(t *testing.T) {
	s := refSample(t) // stereo: 4 bytes per frame
	p := s.NewPad()
	p.UserStart = HeaderSize + 4*10
	p.UserEnd = HeaderSize + 4*25
	tr, err := s.Trim(p)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Frames() != 15 || tr.Channels != 2 {
		t.Fatalf("frames=%d ch=%d", tr.Frames(), tr.Channels)
	}
	if tr.PCM[0] != s.PCM[20] || tr.PCM[29] != s.PCM[49] {
		t.Error("trim took the wrong frames")
	}
}

func TestTrimRejectsBadRange(t *testing.T) {
	s := refSample(t)
	p := s.NewPad()
	p.UserEnd = p.End + 4
	if _, err := s.Trim(p); err == nil {
		t.Error("expected error for user end beyond the sample")
	}
	p = s.NewPad()
	p.UserStart, p.UserEnd = p.UserEnd, p.UserStart
	if _, err := s.Trim(p); err == nil {
		t.Error("expected error for start after end")
	}
}
