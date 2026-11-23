package sp404sx

import "testing"

func stereoPad(frames uint32) Pad {
	end := HeaderSize + 4*frames
	return Pad{Start: 512, End: end, UserStart: 512, UserEnd: end, Volume: 127,
		Gate: true, Format: FormatWAV, Channels: 2, OriginalTempo: 1200, UserTempo: 1200}
}

func TestTrimFrames(t *testing.T) {
	p := stereoPad(1000)
	p.UserStart, p.UserEnd = 512+4*10, 512+4*900
	if s, e := p.TrimFrames(); s != 10 || e != 900 {
		t.Errorf("TrimFrames = %d, %d", s, e)
	}
}

func TestSetTrimStereo(t *testing.T) {
	p := stereoPad(1000)
	if err := p.SetTrim(10, 900); err != nil {
		t.Fatal(err)
	}
	if p.UserStart != 512+40 || p.UserEnd != 512+3600 {
		t.Errorf("offsets %d–%d", p.UserStart, p.UserEnd)
	}
}

// Mono frames are 2 bytes and offsets must be multiples of 4 (as Roland
// writes them): odd frame positions round down to the even frame.
func TestSetTrimMonoAligns(t *testing.T) {
	p := stereoPad(1000)
	p.Channels, p.End, p.UserEnd = 1, 512+2*1000, 512+2*1000
	if err := p.SetTrim(11, 501); err != nil {
		t.Fatal(err)
	}
	if s, e := p.TrimFrames(); s != 10 || e != 500 {
		t.Errorf("mono trim = %d–%d, want 10–500", s, e)
	}
	if (p.UserStart-512)%4 != 0 || (p.UserEnd-512)%4 != 0 {
		t.Errorf("unaligned offsets %d–%d", p.UserStart, p.UserEnd)
	}
}

// The end of the sample is always reachable, even when the file length is
// not a multiple of 4 bytes (mono samples recorded on the device).
func TestSetTrimToEndOfOddMono(t *testing.T) {
	p := stereoPad(0)
	p.Channels, p.End, p.UserEnd = 1, 512+2*1001, 512+2*1001
	if err := p.SetTrim(0, 1001); err != nil {
		t.Fatal(err)
	}
	if p.UserEnd != p.End {
		t.Errorf("UserEnd = %d, want End %d", p.UserEnd, p.End)
	}
}

func TestSetTrimRejectsBadRange(t *testing.T) {
	for _, r := range [][2]int{{-1, 10}, {10, 10}, {20, 10}, {0, 1001}} {
		p := stereoPad(1000)
		if err := p.SetTrim(r[0], r[1]); err == nil {
			t.Errorf("SetTrim(%d, %d) accepted", r[0], r[1])
		}
		if p != stereoPad(1000) {
			t.Errorf("SetTrim(%d, %d) changed the pad despite the error", r[0], r[1])
		}
	}
	// mono 0–1 rounds to an empty range
	p := stereoPad(0)
	p.Channels, p.End, p.UserEnd = 1, 512+2*1000, 512+2*1000
	if err := p.SetTrim(0, 1); err == nil {
		t.Error("mono trim of a single frame accepted (empty after alignment)")
	}
}

func TestSetPadRejectsUnalignedTrim(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	j5 := PadIndex('J', 5)
	p := c.Info.Pads[j5]
	p.UserStart += 2
	if err := c.SetPad(j5, p); err == nil {
		t.Error("unaligned user start accepted")
	}
}
