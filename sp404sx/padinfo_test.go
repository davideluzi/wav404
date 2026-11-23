package sp404sx

import (
	"bytes"
	"os"
	"testing"
)

func loadPadInfo(t *testing.T) (*PadInfo, []byte) {
	t.Helper()
	raw, err := os.ReadFile("testdata/PAD_INFO.BIN")
	if err != nil {
		t.Fatal(err)
	}
	pi, err := ParsePadInfo(raw)
	if err != nil {
		t.Fatalf("ParsePadInfo: %v", err)
	}
	return pi, raw
}

func TestParsePadInfoKnownPad(t *testing.T) {
	pi, _ := loadPadInfo(t)
	// J5: 118916 bytes of stereo PCM, gate on, 89.0 BPM.
	p := pi.Pads[PadIndex('J', 5)]
	want := Pad{
		Start: 512, End: 512 + 118916, UserStart: 512, UserEnd: 512 + 118916,
		Volume: 127, Gate: true, Format: FormatWAV, Channels: 2,
		OriginalTempo: 890, UserTempo: 890,
	}
	if p != want {
		t.Errorf("J5 = %+v\nwant %+v", p, want)
	}
	if p.Empty() {
		t.Error("J5 reported empty")
	}
	if got := p.Frames(); got != 118916/4 {
		t.Errorf("J5 frames = %d", got)
	}
}

func TestParsePadInfoEmptyPad(t *testing.T) {
	pi, _ := loadPadInfo(t)
	p := pi.Pads[PadIndex('A', 1)] // A1 is empty on the card
	if p != EmptyPad() {
		t.Errorf("A1 = %+v, want EmptyPad %+v", p, EmptyPad())
	}
	if !p.Empty() {
		t.Error("A1 not reported empty")
	}
}

func TestPadInfoRoundTrip(t *testing.T) {
	pi, raw := loadPadInfo(t)
	if out := pi.MarshalBinary(); !bytes.Equal(out, raw) {
		t.Error("marshal(parse(PAD_INFO.BIN)) differs from the original")
	}
}

func TestParsePadInfoRejectsWrongSize(t *testing.T) {
	if _, err := ParsePadInfo(make([]byte, 100)); err == nil {
		t.Error("expected an error for a 100-byte file")
	}
}

func TestPadIndexAndName(t *testing.T) {
	if PadIndex('A', 1) != 0 || PadIndex('J', 12) != 119 || PadIndex('B', 3) != 14 {
		t.Error("PadIndex wrong")
	}
	if got := SampleFileName(PadIndex('J', 12)); got != "J0000012.WAV" {
		t.Errorf("SampleFileName = %q", got)
	}
}
