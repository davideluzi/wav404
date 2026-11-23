package main

import (
	"testing"

	"github.com/davideluzi/wav404/sp404sx"
)

func TestPeaksCoverEveryFrame(t *testing.T) {
	// 10 mono frames, one loud sample at the very end
	s := &sp404sx.Sample{Channels: 1, PCM: []int16{0, 0, 0, 0, 0, 0, 0, 0, 0, 32767}}
	p := peaks(s, 3)
	if len(p) != 3 {
		t.Fatalf("%d columns", len(p))
	}
	if p[2].max < 0.99 {
		t.Errorf("last frame missing from the last column: %+v", p)
	}
	if p[0].max != 0 || p[0].min != 0 {
		t.Errorf("silent column = %+v", p[0])
	}
}

func TestPeaksUseBothChannels(t *testing.T) {
	// left silent, right a negative peak
	s := &sp404sx.Sample{Channels: 2, PCM: []int16{0, -16384, 0, 0}}
	p := peaks(s, 1)
	if p[0].min > -0.49 || p[0].max != 0 {
		t.Errorf("peak = %+v, want min ≈ -0.5", p[0])
	}
}

func TestPeaksMoreColumnsThanFrames(t *testing.T) {
	s := &sp404sx.Sample{Channels: 1, PCM: []int16{16384, -16384}}
	p := peaks(s, 8)
	if len(p) != 8 {
		t.Fatalf("%d columns", len(p))
	}
	for i, c := range p {
		if c.max == 0 && c.min == 0 {
			t.Errorf("column %d is empty: every column must show a frame", i)
		}
	}
}

func TestPeaksDegenerate(t *testing.T) {
	if peaks(&sp404sx.Sample{Channels: 1, PCM: []int16{1}}, 0) != nil {
		t.Error("0 columns should give nil")
	}
	if peaks(nil, 10) != nil || peaks(&sp404sx.Sample{Channels: 2}, 10) != nil {
		t.Error("no sample / no frames should give nil")
	}
}
