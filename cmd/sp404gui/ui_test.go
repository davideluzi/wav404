package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"

	"github.com/davideluzi/wav404/sp404sx"
)

// newTestGUI builds the UI on a fake card. Confirmations are accepted and
// background tasks run synchronously.
func newTestGUI(t *testing.T) (*gui, string) {
	t.Helper()
	a := test.NewTempApp(t)
	w := a.NewWindow("test")
	g := newGUI(w)
	g.confirm = func(_, _, _ string, ok func()) { ok() }
	g.runTask = func(_ string, task func(progressFunc) error, done func(error)) { done(task(func(float64, string) {})) }
	g.player = &fakePlayer{}
	g.async = func(work, done func()) { work(); done() } // reads complete at once
	w.SetContent(g.content())
	w.Resize(fyne.NewSize(800, 600))
	root := fakeCard(t)
	if err := g.openCard(root); err != nil {
		t.Fatal(err)
	}
	return g, root
}

func TestTapSelectsPad(t *testing.T) {
	g, _ := newTestGUI(t)
	test.Tap(g.tiles[9]) // pad 10 of bank A
	if g.selected != sp404sx.PadIndex('A', 10) {
		t.Fatalf("selected = %d", g.selected)
	}
	if !strings.Contains(g.info.Text, "A10") || !strings.Contains(g.info.Text, "mono") {
		t.Errorf("info = %q", g.info.Text)
	}
}

func TestBankSwitchUpdatesTiles(t *testing.T) {
	g, _ := newTestGUI(t)
	g.banks.SetSelected("J")
	if got := g.tiles[4].tile; got.title != "J5" || !strings.Contains(got.detail, "89.0") {
		t.Errorf("tile 5 in bank J = %+v", got)
	}
}

func TestDropImportsIntoPadUnderCursor(t *testing.T) {
	g, root := newTestGUI(t)
	target := g.tiles[0] // A1, empty
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(target).
		Add(fyne.NewPos(target.Size().Width/2, target.Size().Height/2))
	src, _ := filepath.Abs(filepath.Join(testdata, "import", "s24.wav"))
	g.drop(pos, []fyne.URI{storage.NewFileURI(src)})

	c, _ := sp404sx.OpenCard(root)
	if c.Info.Pads[0].Empty() || c.Info.Pads[0].OriginalTempo != 890 {
		t.Errorf("A1 after drop = %+v", c.Info.Pads[0])
	}
	if g.tiles[0].tile.empty {
		t.Error("tile A1 not refreshed")
	}
}

func TestClearEmptiesSelectedPad(t *testing.T) {
	g, root := newTestGUI(t)
	test.Tap(g.tiles[9]) // A10
	g.clearSelected()
	c, _ := sp404sx.OpenCard(root)
	if !c.Info.Pads[sp404sx.PadIndex('A', 10)].Empty() || !g.tiles[9].tile.empty {
		t.Error("A10 not cleared")
	}
}

func TestActionsDisabledWithoutSelection(t *testing.T) {
	g, _ := newTestGUI(t)
	if !g.exportBtn.Disabled() || !g.clearBtn.Disabled() || !g.importBtn.Disabled() {
		t.Error("actions enabled with no pad selected")
	}
	test.Tap(g.tiles[0]) // A1 is empty: import yes, export/clear no
	if g.importBtn.Disabled() || !g.exportBtn.Disabled() || !g.clearBtn.Disabled() {
		t.Error("wrong enabled state for an empty pad")
	}
}

// fakePlayer records what the GUI asks to play.
type fakePlayer struct {
	pcm     []byte
	stopped int
	onEnd   func()
}

func (f *fakePlayer) play(pcm []byte, onEnd func()) error {
	f.pcm, f.onEnd = pcm, onEnd
	return nil
}

func (f *fakePlayer) stop() { f.stopped++ }

func TestDoubleTapPlaysPad(t *testing.T) {
	g, _ := newTestGUI(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9]) // A10: mono, 26680 frames → stereo preview
	if len(fp.pcm) != 26680*4 {
		t.Fatalf("played %d bytes, want %d", len(fp.pcm), 26680*4)
	}
	if g.playing != sp404sx.PadIndex('A', 10) || g.selected != g.playing {
		t.Errorf("playing=%d selected=%d", g.playing, g.selected)
	}
	if g.playBtn.label != "STOP" || !g.tiles[9].playing {
		t.Errorf("button %q, tile playing %v", g.playBtn.label, g.tiles[9].playing)
	}
}

func TestPlayButtonToggles(t *testing.T) {
	g, _ := newTestGUI(t)
	fp := g.player.(*fakePlayer)
	test.Tap(g.tiles[9])
	test.Tap(g.playBtn)
	if g.playing < 0 {
		t.Fatal("not playing after Play")
	}
	test.Tap(g.playBtn)
	if g.playing >= 0 || fp.stopped == 0 || g.playBtn.label != "PLAY" {
		t.Errorf("after Stop: playing=%d stopped=%d button=%q", g.playing, fp.stopped, g.playBtn.label)
	}
}

func TestPlaybackEndResetsState(t *testing.T) {
	g, _ := newTestGUI(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9])
	fp.onEnd()
	if g.playing >= 0 || g.playBtn.label != "PLAY" || g.tiles[9].playing {
		t.Error("state not reset when playback ended")
	}
}

func TestStaleEndDoesNotStopNewPlayback(t *testing.T) {
	g, _ := newTestGUI(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9]) // A10
	oldEnd := fp.onEnd
	g.banks.SetSelected("J")
	doubleTap(g.tiles[4]) // J5 starts
	oldEnd()              // A10 finishing late must not stop J5
	if g.playing != sp404sx.PadIndex('J', 5) {
		t.Errorf("playing = %d, want J5", g.playing)
	}
}

func TestPlayDisabledOnEmptyPad(t *testing.T) {
	g, _ := newTestGUI(t)
	test.Tap(g.tiles[0]) // A1 empty
	if !g.playBtn.Disabled() {
		t.Error("Play enabled on an empty pad")
	}
	doubleTap(g.tiles[0])
	if g.playing >= 0 {
		t.Error("empty pad started playing")
	}
}

func TestCardChangesStopPlayback(t *testing.T) {
	g, _ := newTestGUI(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9])
	g.clearSelected()
	if g.playing >= 0 || fp.stopped == 0 {
		t.Error("clearing the playing pad did not stop playback")
	}
}

// doubleTap is two quick taps, as the GLFW driver delivers a double click
// to a widget that is only Tappable.
func doubleTap(p *padTile) {
	test.Tap(p)
	test.Tap(p)
}

// A widget implementing fyne.DoubleTappable gets every single tap delayed
// by the system double-click interval: selection would feel slow.
func TestPadTileIsNotDoubleTappable(t *testing.T) {
	var o fyne.CanvasObject = newPadTile(1, func() {}, func() {})
	if _, ok := o.(fyne.DoubleTappable); ok {
		t.Error("padTile implements fyne.DoubleTappable: single taps would be delayed")
	}
}

func TestSlowTapsSelectWithoutPlaying(t *testing.T) {
	g, _ := newTestGUI(t)
	now := time.Now()
	g.tiles[9].now = func() time.Time { return now }
	test.Tap(g.tiles[9])
	now = now.Add(2 * time.Second)
	test.Tap(g.tiles[9])
	if g.playing >= 0 {
		t.Error("two slow taps started playback")
	}
	if g.selected != sp404sx.PadIndex('A', 10) {
		t.Errorf("selected = %d", g.selected)
	}
}

func TestQuickTapsOnDifferentPadsDoNotPlay(t *testing.T) {
	g, _ := newTestGUI(t)
	test.Tap(g.tiles[0])
	test.Tap(g.tiles[9])
	if g.playing >= 0 {
		t.Error("taps on two different pads started playback")
	}
}

func TestSelectingAnotherPadStopsPlayback(t *testing.T) {
	g, _ := newTestGUI(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9]) // A10 plays
	test.Tap(g.tiles[0])  // select A1
	if g.playing >= 0 || fp.stopped == 0 {
		t.Errorf("playing=%d stopped=%d after selecting another pad", g.playing, fp.stopped)
	}
	if g.selected != 0 {
		t.Errorf("selected = %d, want A1", g.selected)
	}
}

func TestTapOnPlayingPadKeepsPlaying(t *testing.T) {
	g, _ := newTestGUI(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9])
	g.tiles[9].now = func() time.Time { return time.Now().Add(time.Hour) } // not a double click
	test.Tap(g.tiles[9])
	if g.playing != sp404sx.PadIndex('A', 10) || fp.stopped != 0 {
		t.Errorf("playing=%d stopped=%d", g.playing, fp.stopped)
	}
}

func TestBankChangeStopsPlayback(t *testing.T) {
	g, _ := newTestGUI(t)
	doubleTap(g.tiles[9])
	g.banks.SetSelected("B")
	if g.playing >= 0 {
		t.Error("bank change did not stop playback")
	}
}
