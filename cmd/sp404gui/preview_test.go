package main

import (
	"encoding/binary"
	"testing"

	"github.com/davideluzi/wav404/sp404sx"
)

func samples(b []byte) []int16 {
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return out
}

func TestPreviewStereoPlainIsUnchanged(t *testing.T) {
	s := &sp404sx.Sample{Channels: 2, PCM: []int16{1, -1, 2, -2, 3, -3}}
	got := samples(previewPCM(s, s.NewPad()))
	want := []int16{1, -1, 2, -2, 3, -3}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestPreviewMonoIsDuplicated(t *testing.T) {
	s := &sp404sx.Sample{Channels: 1, PCM: []int16{5, 6, 7, 8}}
	got := samples(previewPCM(s, s.NewPad()))
	want := []int16{5, 5, 6, 6, 7, 7, 8, 8}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestPreviewFollowsTrimReverseAndVolume(t *testing.T) {
	s := &sp404sx.Sample{Channels: 1, PCM: []int16{100, 200, 300, 400, 500, 600}}
	p := s.NewPad()
	p.UserStart = sp404sx.HeaderSize + 2*1 // frames 1–3: 200 300 400
	p.UserEnd = sp404sx.HeaderSize + 2*4
	p.Reverse = true
	p.Volume = 127 / 2 // ≈ half
	got := samples(previewPCM(s, p))
	want := []int16{198, 198, 149, 149, 99, 99} // 400 300 200 reversed, × 63/127
	for i := range want {
		if i >= len(got) || abs(int(got[i])-int(want[i])) > 1 {
			t.Fatalf("got %v, want ≈%v", got, want)
		}
	}
}

func TestPreviewBadTrimPlaysWholeSample(t *testing.T) {
	s := &sp404sx.Sample{Channels: 1, PCM: []int16{1, 2, 3}}
	p := s.NewPad()
	p.UserEnd = p.End + 100 // corrupt record: ignore the trim
	if got := samples(previewPCM(s, p)); len(got) != 6 {
		t.Errorf("got %v", got)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
