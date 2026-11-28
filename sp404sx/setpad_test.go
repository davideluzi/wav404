package sp404sx

import (
	"testing"
)

func TestSetPadPersists(t *testing.T) {
	root := fakeCard(t)
	c, _ := OpenCard(root)
	j5 := PadIndex('J', 5)
	p := c.Info.Pads[j5]
	p.Volume, p.Loop, p.Gate, p.Reverse, p.Lofi = 90, true, false, true, true
	p.SetTempo(1300)
	if err := c.SetPad(j5, p); err != nil {
		t.Fatal(err)
	}
	c2, _ := OpenCard(root)
	if c2.Info.Pads[j5] != p {
		t.Errorf("reopened J5 = %+v\nwant %+v", c2.Info.Pads[j5], p)
	}
}

func TestSetPadValidates(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	j5 := PadIndex('J', 5)
	orig := c.Info.Pads[j5]
	bad := map[string]func(*Pad){
		"volume > 127":           func(p *Pad) { p.Volume = 128 },
		"tempo < 40.0":           func(p *Pad) { p.SetTempo(399) },
		"tempo > 200.0":          func(p *Pad) { p.SetTempo(2001) },
		"only the shown tempo":   func(p *Pad) { p.Tempo = 1300 },
		"only the tempo copy":    func(p *Pad) { p.TempoCopy = 1300 },
		"sample offsets changed": func(p *Pad) { p.End -= 4 },
		"channels changed":       func(p *Pad) { p.Channels = 1 },
	}
	for name, change := range bad {
		p := orig
		change(&p)
		if err := c.SetPad(j5, p); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if c.Info.Pads[j5] != orig {
			t.Fatalf("%s: pad modified despite the error", name)
		}
	}
	// edges are valid
	p := orig
	p.Volume = 0
	p.SetTempo(400)
	if err := c.SetPad(j5, p); err != nil {
		t.Errorf("volume 0 / 40.0 BPM rejected: %v", err)
	}
	p.Volume = 127
	p.SetTempo(2000)
	if err := c.SetPad(j5, p); err != nil {
		t.Errorf("volume 127 / 200.0 BPM rejected: %v", err)
	}
}

func TestSetPadRejectsEmptyPad(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	p := c.Info.Pads[0] // A1 empty
	p.Loop = true
	if err := c.SetPad(0, p); err == nil {
		t.Error("settings of an empty pad accepted")
	}
}

// B4 in the fixture has different values in its two tempo fields, as
// written by wav404 before it knew the device shows bytes 24–27. Other
// settings still save, and SetTempo puts the fields back in step.
func TestSetPadMismatchedTempoFields(t *testing.T) {
	root := fakeCard(t)
	c, _ := OpenCard(root)
	b4 := PadIndex('B', 4)
	p := c.Info.Pads[b4]
	if p.Tempo == p.TempoCopy {
		t.Fatalf("fixture B4 tempo fields already equal (%d)", p.Tempo)
	}
	p.Volume = 50
	if err := c.SetPad(b4, p); err != nil {
		t.Fatalf("volume change refused: %v", err)
	}
	p.SetTempo(p.Tempo)
	if err := c.SetPad(b4, p); err != nil {
		t.Fatalf("SetTempo to the shown tempo refused: %v", err)
	}
	c2, _ := OpenCard(root)
	if got := c2.Info.Pads[b4]; got.Tempo != 770 || got.TempoCopy != 770 {
		t.Errorf("tempo fields = %d, %d; want 770, 770", got.Tempo, got.TempoCopy)
	}
}
