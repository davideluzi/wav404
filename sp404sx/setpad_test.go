package sp404sx

import (
	"testing"
)

func TestSetPadPersists(t *testing.T) {
	root := fakeCard(t)
	c, _ := OpenCard(root)
	j5 := PadIndex('J', 5)
	p := c.Info.Pads[j5]
	p.Volume, p.Loop, p.Gate, p.Reverse, p.Lofi, p.UserTempo = 90, true, false, true, true, 1300
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
		"user tempo < 40.0":      func(p *Pad) { p.UserTempo = 399 },
		"user tempo > 200.0":     func(p *Pad) { p.UserTempo = 2001 },
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
	p.Volume, p.UserTempo = 0, 400
	if err := c.SetPad(j5, p); err != nil {
		t.Errorf("volume 0 / 40.0 BPM rejected: %v", err)
	}
	p.Volume, p.UserTempo = 127, 2000
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
