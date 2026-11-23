package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/davideluzi/wav404/sp404sx"
)

func reopenPad(t *testing.T, root string, index int) sp404sx.Pad {
	t.Helper()
	c, err := sp404sx.OpenCard(root)
	if err != nil {
		t.Fatal(err)
	}
	return c.Info.Pads[index]
}

// selectJ5 shows bank J and selects J5 (127 vol, gate, 89.0 BPM).
func selectJ5(g *gui) int {
	g.banks.SetSelected("J")
	test.Tap(g.tiles[4])
	return sp404sx.PadIndex('J', 5)
}

func TestPanelShowsSelectedPad(t *testing.T) {
	g, _ := newTestGUI(t)
	selectJ5(g)
	if g.volume.Value != 127 || g.volumeLabel.Text != "127" {
		t.Errorf("volume = %v / %q", g.volume.Value, g.volumeLabel.Text)
	}
	if !g.gate.Checked || g.loop.Checked || g.reverse.Checked || g.lofi.Checked {
		t.Error("flags do not match J5 (gate only)")
	}
	if g.bpm.Text != "89.0" || !strings.Contains(g.origBPM.Text, "89.0") {
		t.Errorf("bpm %q, original %q", g.bpm.Text, g.origBPM.Text)
	}
	if g.volume.Disabled() || g.bpm.Disabled() {
		t.Error("panel disabled on an assigned pad")
	}
}

func TestPanelDisabledForEmptyPad(t *testing.T) {
	g, _ := newTestGUI(t)
	test.Tap(g.tiles[0]) // A1
	if !g.volume.Disabled() || !g.loop.Disabled() || !g.bpm.Disabled() || !g.resetBPM.Disabled() {
		t.Error("panel enabled on an empty pad")
	}
}

// Filling in the panel must not write at all, not even identical bytes:
// every write wears the SD card. Writes replace the file (atomic rename),
// so an unchanged file identity proves nothing was written.
func TestSelectingPadsDoesNotWrite(t *testing.T) {
	g, root := newTestGUI(t)
	path := filepath.Join(root, "ROLAND", "SP-404SX", "SMPL", "PAD_INFO.BIN")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	test.Tap(g.tiles[9]) // A10
	selectJ5(g)
	test.Tap(g.tiles[0]) // J1
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) {
		t.Error("selecting pads rewrote PAD_INFO.BIN")
	}
}

func TestToggleSavesImmediately(t *testing.T) {
	g, root := newTestGUI(t)
	j5 := selectJ5(g)
	test.Tap(g.loop)
	test.Tap(g.gate) // off
	p := reopenPad(t, root, j5)
	if !p.Loop || p.Gate {
		t.Errorf("saved loop=%v gate=%v", p.Loop, p.Gate)
	}
	if !strings.Contains(g.tiles[4].tile.flags, "loop") || strings.Contains(g.tiles[4].tile.flags, "gate") {
		t.Errorf("tile flags = %q", g.tiles[4].tile.flags)
	}
}

func TestVolumeSavesOnRelease(t *testing.T) {
	g, root := newTestGUI(t)
	j5 := selectJ5(g)
	// drag the knob to about the middle, then release
	mid := fyne.NewPos(g.volume.Size().Width/2, g.volume.Size().Height/2)
	g.volume.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: mid}})
	dragged := int(g.volume.Value)
	if dragged == 127 || g.volumeLabel.Text != strconv.Itoa(dragged) {
		t.Fatalf("during drag: value %d, label %q", dragged, g.volumeLabel.Text)
	}
	if reopenPad(t, root, j5).Volume != 127 {
		t.Error("volume written before release")
	}
	g.volume.DragEnd()
	if v := reopenPad(t, root, j5).Volume; int(v) != dragged {
		t.Errorf("saved volume = %d, want %d", v, dragged)
	}
}

func TestBPMEntry(t *testing.T) {
	g, root := newTestGUI(t)
	j5 := selectJ5(g)
	g.bpm.SetText("130")
	g.bpm.OnSubmitted("130")
	if p := reopenPad(t, root, j5); p.UserTempo != 1300 || p.OriginalTempo != 890 {
		t.Errorf("tempo = %d (original %d)", p.UserTempo, p.OriginalTempo)
	}
	if g.bpm.Text != "130.0" {
		t.Errorf("entry shows %q", g.bpm.Text)
	}

	g.bpm.SetText("250") // out of range: rejected, entry restored
	g.bpm.OnSubmitted("250")
	if reopenPad(t, root, j5).UserTempo != 1300 || g.bpm.Text != "130.0" {
		t.Errorf("invalid tempo: saved %d, entry %q", reopenPad(t, root, j5).UserTempo, g.bpm.Text)
	}

	test.Tap(g.resetBPM)
	if reopenPad(t, root, j5).UserTempo != 890 || g.bpm.Text != "89.0" {
		t.Error("reset to original tempo failed")
	}
}

func TestUnchangedEditDoesNotWrite(t *testing.T) {
	g, root := newTestGUI(t)
	selectJ5(g)
	path := filepath.Join(root, "ROLAND", "SP-404SX", "SMPL", "PAD_INFO.BIN")
	before, _ := os.Stat(path)
	g.bpm.OnSubmitted("89.0") // same tempo
	g.volume.DragEnd()        // released on the same value
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) {
		t.Error("an edit that changes nothing rewrote PAD_INFO.BIN")
	}
}
