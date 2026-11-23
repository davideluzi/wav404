package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/davideluzi/wav404/sp404sx"
)

// newTestWave shows 1000 mono... stereo frames on a 100 px wide view, so
// 1 px = 10 frames.
func newTestWave(t *testing.T) (*waveView, *[2]int) {
	t.Helper()
	test.NewTempApp(t)
	got := &[2]int{-1, -1}
	w := newWaveView(func(start, end int) { got[0], got[1] = start, end })
	w.Resize(fyne.NewSize(100, 50))
	w.setSample(&sp404sx.Sample{Channels: 2, PCM: make([]int16, 2*1000)}, 0, 1000)
	return w, got
}

// drag moves from x0 to x1 the way the driver reports it: each event has
// the current position and the delta since the previous one.
func drag(w *waveView, x0, x1 float32) {
	w.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(x1, 25)},
		Dragged:    fyne.NewDelta(x1-x0, 0),
	})
	w.DragEnd()
}

func TestDragNearStartMovesStart(t *testing.T) {
	w, got := newTestWave(t)
	drag(w, 2, 30)
	if w.start != 300 || w.end != 1000 {
		t.Errorf("markers %d–%d, want 300–1000", w.start, w.end)
	}
	if got[0] != 300 || got[1] != 1000 {
		t.Errorf("onTrim(%d, %d), want (300, 1000)", got[0], got[1])
	}
}

func TestDragNearEndMovesEnd(t *testing.T) {
	w, _ := newTestWave(t)
	drag(w, 99, 60)
	if w.start != 0 || w.end != 600 {
		t.Errorf("markers %d–%d, want 0–600", w.start, w.end)
	}
}

func TestMarkersCannotCross(t *testing.T) {
	w, _ := newTestWave(t)
	drag(w, 98, 40) // end → 400
	drag(w, 1, 90)  // start dragged past the end
	if w.start >= w.end {
		t.Errorf("markers crossed: %d–%d", w.start, w.end)
	}
	drag(w, 39, -50) // end dragged before 0 / the start
	if w.start >= w.end || w.start < 0 {
		t.Errorf("markers %d–%d", w.start, w.end)
	}
}

func TestDragOutsideViewClamps(t *testing.T) {
	w, _ := newTestWave(t)
	drag(w, 2, -40)
	drag(w, 99, 400)
	if w.start != 0 || w.end != 1000 {
		t.Errorf("markers %d–%d, want 0–1000", w.start, w.end)
	}
}

func TestNoTrimCallbackWithoutSample(t *testing.T) {
	w, got := newTestWave(t)
	w.setSample(nil, 0, 0)
	drag(w, 2, 30)
	if got[0] != -1 {
		t.Error("onTrim called with no sample shown")
	}
}

func TestOnTrimOnlyOnRelease(t *testing.T) {
	w, got := newTestWave(t)
	w.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(30, 25)}, Dragged: fyne.NewDelta(28, 0)})
	if got[0] != -1 {
		t.Error("onTrim called during the drag")
	}
	w.DragEnd()
	if got[0] != 300 {
		t.Errorf("onTrim start = %d after release", got[0])
	}
}
