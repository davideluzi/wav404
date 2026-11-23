package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

func TestHWKeyTapAndDisable(t *testing.T) {
	test.NewTempApp(t)
	taps := 0
	k := newHWKey("WHOLE", 11, 6)
	k.onTap = func() { taps++ }
	test.Tap(k)
	k.Disable()
	test.Tap(k)
	if taps != 1 || !k.Disabled() {
		t.Errorf("taps=%d disabled=%v", taps, k.Disabled())
	}
	k.Enable()
	test.Tap(k)
	if taps != 2 {
		t.Errorf("taps=%d after Enable", taps)
	}
}

func TestHWToggleBehavesLikeACheck(t *testing.T) {
	test.NewTempApp(t)
	var got []bool
	k := newHWToggle("LOOP", func(on bool) { got = append(got, on) })
	test.Tap(k)
	test.Tap(k)
	k.SetChecked(true) // programmatic set notifies too, like widget.Check
	if len(got) != 3 || !got[0] || got[1] || !got[2] || !k.Checked || !k.lit {
		t.Errorf("changes %v, checked %v, lit %v", got, k.Checked, k.lit)
	}
	k.Disable()
	test.Tap(k)
	if !k.Checked || len(got) != 3 {
		t.Error("a disabled toggle reacted to a tap")
	}
}

func TestBankSelector(t *testing.T) {
	test.NewTempApp(t)
	var got []string
	b := newBankSelector(func(s string) { got = append(got, s) })
	b.SetSelected("C")
	test.Tap(b.keys[9]) // J
	if b.Selected != "J" || len(got) != 2 || got[0] != "C" || got[1] != "J" {
		t.Errorf("selected %q, changes %v", b.Selected, got)
	}
	if !b.keys[9].lit || b.keys[2].lit {
		t.Error("only the selected bank key should be lit")
	}
	test.Tap(b.keys[9]) // tapping the current bank changes nothing
	if len(got) != 2 {
		t.Errorf("changes %v after re-tapping J", got)
	}
}

func TestKnobDragsVertically(t *testing.T) {
	test.NewTempApp(t)
	var changed, ended []float64
	k := newHWKnob(0, 127)
	k.OnChanged = func(v float64) { changed = append(changed, v) }
	k.OnChangeEnded = func(v float64) { ended = append(ended, v) }
	k.Resize(fyne.NewSize(100, 100))
	k.SetValue(100)
	changed, ended = nil, nil

	k.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, 20)}) // down: lower
	if k.Value >= 100 || len(changed) != 1 || len(ended) != 0 {
		t.Fatalf("after drag down: value %v, changed %v, ended %v", k.Value, changed, ended)
	}
	k.DragEnd()
	if len(ended) != 1 || ended[0] != k.Value {
		t.Errorf("ended %v", ended)
	}
	k.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, -10000)}) // far up: clamps
	k.DragEnd()
	if k.Value != 127 {
		t.Errorf("value %v, want clamped to 127", k.Value)
	}
	if k.Value != float64(int(k.Value)) {
		t.Errorf("value %v not on a whole step", k.Value)
	}
	k.Disable()
	k.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, 50)})
	if k.Value != 127 {
		t.Error("disabled knob moved")
	}
}

func TestScaledLayout(t *testing.T) {
	test.NewTempApp(t)
	box := canvas.NewRectangle(nil)
	label := canvas.NewText("BPM", nil)
	label.TextSize = 10
	l := newScaledLayout(fyne.NewSize(100, 50))
	l.place(box, 10, 10, 20, 10)
	l.place(label, 50, 0, 0, 0)
	c := container.New(l, l.objects()...)

	c.Resize(fyne.NewSize(200, 100)) // exactly 2×
	if box.Position() != fyne.NewPos(20, 20) || box.Size() != fyne.NewSize(40, 20) {
		t.Errorf("2×: box at %v size %v", box.Position(), box.Size())
	}
	if label.TextSize != 20 {
		t.Errorf("2×: text size %v", label.TextSize)
	}

	c.Resize(fyne.NewSize(400, 100)) // wider: scale 2, centred horizontally
	if box.Position() != fyne.NewPos(120, 20) {
		t.Errorf("wide: box at %v, want centred at (120, 20)", box.Position())
	}
	if l.MinSize(nil) != fyne.NewSize(70, 35) { // may shrink to 70%
		t.Errorf("min size %v", l.MinSize(nil))
	}
}
