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
	// drag the knob down, then release
	g.volume.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, 40)})
	dragged := int(g.volume.Value)
	if dragged >= 127 || g.volumeLabel.Text != strconv.Itoa(dragged) {
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

func TestWaveShowsSelectedPad(t *testing.T) {
	g, _ := newTestGUI(t)
	selectJ5(g)
	if g.wave.frames() != 29729 || g.wave.start != 0 || g.wave.end != 29729 {
		t.Errorf("wave: %d frames, trim %d–%d", g.wave.frames(), g.wave.start, g.wave.end)
	}
	if !strings.Contains(g.trimLabel.Text, "0.674") {
		t.Errorf("trim label = %q", g.trimLabel.Text)
	}
}

func TestTrimSavesAndResets(t *testing.T) {
	g, root := newTestGUI(t)
	j5 := selectJ5(g)
	g.wave.onTrim(1000, 20000) // as at the end of a drag
	p := reopenPad(t, root, j5)
	if p.UserStart != 512+4*1000 || p.UserEnd != 512+4*20000 {
		t.Errorf("saved trim %d–%d", p.UserStart, p.UserEnd)
	}
	if g.wave.start != 1000 || g.wave.end != 20000 || !strings.Contains(g.trimLabel.Text, "0.431") {
		t.Errorf("after trim: markers %d–%d, label %q", g.wave.start, g.wave.end, g.trimLabel.Text)
	}
	test.Tap(g.resetTrim)
	if p := reopenPad(t, root, j5); p.UserStart != p.Start || p.UserEnd != p.End {
		t.Errorf("after reset: %d–%d", p.UserStart, p.UserEnd)
	}
	if g.wave.start != 0 || g.wave.end != 29729 {
		t.Errorf("markers after reset: %d–%d", g.wave.start, g.wave.end)
	}
}

func TestWaveEmptyForEmptyPad(t *testing.T) {
	g, _ := newTestGUI(t)
	selectJ5(g)
	g.banks.SetSelected("A")
	test.Tap(g.tiles[0]) // A1
	if g.wave.sample != nil || !g.resetTrim.Disabled() || g.trimLabel.Text != "" {
		t.Errorf("empty pad: wave %v, reset disabled %v, label %q", g.wave.sample != nil, g.resetTrim.Disabled(), g.trimLabel.Text)
	}
}

func TestWaveMissingFile(t *testing.T) {
	g, _ := newTestGUI(t)
	test.Tap(g.tiles[1]) // A2: assigned in PAD_INFO, but no file on the fake card
	if g.wave.sample != nil || !strings.Contains(strings.ToLower(g.trimLabel.Text), "not found") {
		t.Errorf("wave %v, label %q", g.wave.sample != nil, g.trimLabel.Text)
	}
}

// Importing a different file with the same length gives an identical pad
// record: the waveform must still show the new audio.
func TestWaveReloadsAfterImport(t *testing.T) {
	g, root := newTestGUI(t)
	j5 := selectJ5(g)
	before := g.wave.sample
	recBefore := reopenPad(t, root, j5)
	silent := &sp404sx.Sample{Channels: 2, PCM: make([]int16, 2*29729)}
	src := filepath.Join(t.TempDir(), "silent.wav")
	if err := os.WriteFile(src, silent.EncodeWAV(), 0o644); err != nil {
		t.Fatal(err)
	}
	g.importInto(j5, src)
	if reopenPad(t, root, j5) != recBefore {
		t.Fatal("setup: the new pad record should be identical to the old one")
	}
	if g.wave.sample == before || g.wave.sample.PCM[1000] != 0 {
		t.Error("waveform still shows the old sample")
	}
}

func TestBPMDisplay(t *testing.T) {
	g, root := newTestGUI(t)
	j5 := selectJ5(g)
	if g.bpmDisplay.text != "89.0" {
		t.Errorf("display shows %q", g.bpmDisplay.text)
	}
	test.Tap(g.bpmDisplay)
	if g.bpmPopup == nil || !g.bpmPopup.Visible() {
		t.Fatal("tapping the display did not open the tempo editor")
	}
	g.bpm.SetText("120")
	g.bpm.OnSubmitted("120")
	if g.bpmPopup.Visible() {
		t.Error("editor still open after Enter")
	}
	if g.bpmDisplay.text != "120.0" || reopenPad(t, root, j5).UserTempo != 1200 {
		t.Errorf("display %q, saved %d", g.bpmDisplay.text, reopenPad(t, root, j5).UserTempo)
	}
	test.Tap(g.tiles[11]) // J12: empty
	if g.bpmDisplay.text != "" {
		t.Errorf("empty pad: display shows %q", g.bpmDisplay.text)
	}
	test.Tap(g.bpmDisplay)
	if g.bpmPopup.Visible() {
		t.Error("tempo editor opened for an empty pad")
	}
}

func TestChannelsIndicator(t *testing.T) {
	g, _ := newTestGUI(t)
	test.Tap(g.tiles[9]) // A10 mono
	if g.channels.label != "MONO" || !g.channels.lit {
		t.Errorf("A10: %q lit=%v", g.channels.label, g.channels.lit)
	}
	selectJ5(g)
	if g.channels.label != "STEREO" {
		t.Errorf("J5: %q", g.channels.label)
	}
}
