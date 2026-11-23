package main

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestCachedReusesImages(t *testing.T) {
	calls := 0
	draw := func(img *image.NRGBA) { calls++ }
	a := cached("test-reuse", 10, 10, draw)
	b := cached("test-reuse", 10, 10, draw)
	c := cached("test-reuse", 12, 10, draw) // another size is another image
	if calls != 2 || a != b || a == c {
		t.Errorf("calls=%d same=%v", calls, a == b)
	}
}

// After the first full render, selecting a pad must not redraw the whole
// panel: at most the two pads whose look changed (old and new selection).
func TestTapRedrawsOnlyWhatChanged(t *testing.T) {
	g, _ := newTestGUI(t)
	selectJ5(g)
	g.win.Canvas().Capture() // warm-up: draws every style once
	before := drawCount.Load()
	test.Tap(g.tiles[5]) // J6
	g.win.Canvas().Capture()
	if n := drawCount.Load() - before; n > 2 {
		t.Errorf("selecting a pad drew %d images, want at most 2", n)
	}
}

// Once the panel has been shown and the variants prepared in the
// background, the first selection, first playback and first flag changes
// need no key or pad image at all.
func TestFirstInteractionsNeedNoKeyDraws(t *testing.T) {
	g, _ := newTestGUI(t)
	g.win.Canvas().Capture()
	flushPrewarm()
	before := drawsOf("key")
	test.Tap(g.tiles[9])  // first selection (A10)
	doubleTap(g.tiles[9]) // first playback
	test.Tap(g.gate)      // first flag change
	g.win.Canvas().Capture()
	if n := drawsOf("key") - before; n != 0 {
		t.Errorf("first interactions drew %d key images, want 0", n)
	}
}

func TestKnobTurnDrawsNothing(t *testing.T) {
	g, _ := newTestGUI(t)
	test.Tap(g.tiles[9])
	g.win.Canvas().Capture()
	before := drawCount.Load()
	g.volume.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, 30)})
	g.win.Canvas().Capture()
	if n := drawCount.Load() - before; n != 0 {
		t.Errorf("turning the knob drew %d images", n)
	}
}

func TestPrewarmDrawsVariantsOfShownKeys(t *testing.T) {
	test.NewTempApp(t)
	k := newHWKey("LOOP", 12, 7)
	k.Resize(fyne.NewSize(98, 42))
	r := k.CreateRenderer().(*hwKeyRenderer)
	r.body.Generator(196, 84) // the driver asks for the unlit look
	flushPrewarm()
	lit := k.style()
	lit.face, lit.glow = hwAmber, 1
	if !isCached(lit.key(), 196, 84) {
		t.Error("lit variant not prepared at the same pixel size")
	}
}
