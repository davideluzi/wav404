package main

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// rasterOf wraps a drawing function as a canvas object that redraws at the
// on-screen pixel size. key() names the current look: images are cached
// per key and size, so only a new look is actually drawn. variants (may be
// nil) lists the other looks this object can take; they are prepared in
// the background at the same size.
func rasterOf(key func() string, draw func(img *image.NRGBA), variants func() []variant) *canvas.Raster {
	var r *canvas.Raster
	r = canvas.NewRaster(func(w, h int) image.Image {
		img := cached(key(), w, h, draw)
		if variants != nil {
			notePrewarm(r, w, h, variants())
		}
		return img
	})
	return r
}

// staticRaster is a raster whose look never changes.
func staticRaster(name string, draw func(img *image.NRGBA)) *canvas.Raster {
	return rasterOf(func() string { return name }, draw, nil)
}

// keyVariants lists the looks of a key as variants for prewarming.
func keyVariants(styles ...keyStyle) []variant {
	vs := make([]variant, len(styles))
	for i, s := range styles {
		vs[i] = variant{s.key(), func(img *image.NRGBA) { drawKey(img, s) }}
	}
	return vs
}

// keyGeometry converts a design corner radius and inset (points at scale
// 1) to the fractions keyStyle uses, for a key of the given size.
func keyGeometry(size fyne.Size, radius, inset, scale float32) (float64, float64) {
	m := min(size.Width, size.Height)
	if m <= 0 {
		return 0, 0
	}
	return float64(radius * scale / m), float64(inset * scale / m)
}

// silk is panel lettering ("BANK", "SAMPLE MODE", captions).
func silk(text string, size float32, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// scalable widgets resize their lettering with the panel.
type scalable interface{ setScale(s float32) }

// hwKey is a rubber key (small, with a word on it). When lit its face
// glows amber; when disabled it is dimmed and ignores taps.
type hwKey struct {
	widget.BaseWidget
	label, detail string
	lit           bool
	full          bool // a pad holding a sample (lighter rubber)
	disabled      bool
	labelSize     float32
	radius        float64
	scale         float32
	onTap         func()
}

func newHWKey(label string, labelSize float32, radius float64) *hwKey {
	k := &hwKey{label: label, labelSize: labelSize, radius: radius, scale: 1}
	k.ExtendBaseWidget(k)
	return k
}

func (k *hwKey) Tapped(*fyne.PointEvent) {
	if !k.disabled && k.onTap != nil {
		k.onTap()
	}
}

func (k *hwKey) Disable()       { k.disabled = true; k.Refresh() }
func (k *hwKey) Enable()        { k.disabled = false; k.Refresh() }
func (k *hwKey) Disabled() bool { return k.disabled }

func (k *hwKey) setScale(s float32) {
	if k.scale != s {
		k.scale = s
		k.Refresh()
	}
}

func (k *hwKey) style() keyStyle { return k.styleFor(k.lit, k.disabled) }

func (k *hwKey) styleFor(lit, disabled bool) keyStyle {
	s := keyStyle{face: hwRubber}
	s.radius, s.inset = keyGeometry(k.Size(), float32(k.radius), 2, k.scale)
	if k.full {
		s.face = hwRubberFull
	}
	if lit && !disabled {
		s.face, s.glow = hwAmber, 1
	}
	if disabled {
		s.face = shade(hwRubber, -0.25)
	}
	return s
}

func (k *hwKey) CreateRenderer() fyne.WidgetRenderer {
	r := &hwKeyRenderer{k: k, label: silk("", k.labelSize, hwSilk), detail: silk("", 11, hwSilkDim)}
	r.body = rasterOf(func() string { return k.style().key() },
		func(img *image.NRGBA) { drawKey(img, k.style()) },
		func() []variant {
			return keyVariants(k.styleFor(false, false), k.styleFor(true, false), k.styleFor(false, true))
		})
	r.Refresh()
	return r
}

type hwKeyRenderer struct {
	k             *hwKey
	body          *canvas.Raster
	label, detail *canvas.Text
}

func (r *hwKeyRenderer) Layout(s fyne.Size) {
	r.body.Resize(s)
	lm, dm := r.label.MinSize(), r.detail.MinSize()
	if r.k.detail == "" {
		r.label.Move(fyne.NewPos((s.Width-lm.Width)/2, (s.Height-lm.Height)/2))
		return
	}
	r.label.Move(fyne.NewPos((s.Width-lm.Width)/2, s.Height*0.42-lm.Height/2))
	r.detail.Move(fyne.NewPos((s.Width-dm.Width)/2, s.Height*0.72-dm.Height/2))
}

func (r *hwKeyRenderer) MinSize() fyne.Size { return fyne.NewSize(40, 32) }

func (r *hwKeyRenderer) Refresh() {
	k := r.k
	r.label.Text, r.detail.Text = k.label, k.detail
	r.label.TextSize, r.detail.TextSize = k.labelSize*k.scale, 11*k.scale
	r.label.Color, r.detail.Color = hwSilk, hwSilkDim
	switch {
	case k.disabled:
		r.label.Color = shade(hwSilkDim, -0.35)
	case k.lit:
		r.label.Color, r.detail.Color = color.NRGBA{0x2a, 0x1a, 0x00, 0xff}, color.NRGBA{0x4a, 0x30, 0x00, 0xff}
	case !k.full && k.detail != "":
		r.label.Color = hwSilkDim // empty pad
	}
	r.Layout(k.Size())
	r.body.Refresh()
	r.label.Refresh()
	r.detail.Refresh()
}

func (r *hwKeyRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.body, r.label, r.detail}
}

func (r *hwKeyRenderer) Destroy() {}

// hwToggle is a latching key, lit when on. Its API mirrors widget.Check.
type hwToggle struct {
	hwKey
	Checked   bool
	OnChanged func(bool)
}

func newHWToggle(label string, onChanged func(bool)) *hwToggle {
	t := &hwToggle{OnChanged: onChanged}
	t.label, t.labelSize, t.radius, t.scale = label, 12, 7, 1
	t.ExtendBaseWidget(t)
	return t
}

func (t *hwToggle) Tapped(*fyne.PointEvent) {
	if !t.disabled {
		t.SetChecked(!t.Checked)
	}
}

// SetChecked changes the state and notifies OnChanged, like widget.Check.
func (t *hwToggle) SetChecked(on bool) {
	if on == t.Checked {
		return
	}
	t.Checked, t.lit = on, on
	t.Refresh()
	if t.OnChanged != nil {
		t.OnChanged(on)
	}
}

// bankSelector is the row of bank keys A–J; exactly one is lit.
type bankSelector struct {
	keys      [10]*hwKey
	Selected  string
	OnChanged func(string)
}

func newBankSelector(onChanged func(string)) *bankSelector {
	b := &bankSelector{OnChanged: onChanged}
	for i := range b.keys {
		letter := string(rune('A' + i))
		b.keys[i] = newHWKey(letter, 14, 7)
		b.keys[i].onTap = func() { b.SetSelected(letter) }
	}
	return b
}

// SetSelected lights the bank's key and notifies OnChanged if it changed.
func (b *bankSelector) SetSelected(letter string) {
	if letter == b.Selected {
		return
	}
	b.Selected = letter
	for i, k := range b.keys {
		k.lit = string(rune('A'+i)) == letter
		k.Refresh()
	}
	if b.OnChanged != nil {
		b.OnChanged(letter)
	}
}

// hwDisplay is an amber 7-segment readout; tapping it calls onTap.
type hwDisplay struct {
	widget.BaseWidget
	text  string
	cells int
	onTap func()
}

func newHWDisplay(cells int) *hwDisplay {
	d := &hwDisplay{cells: cells}
	d.ExtendBaseWidget(d)
	return d
}

func (d *hwDisplay) setText(s string) {
	if d.text != s {
		d.text = s
		d.Refresh()
	}
}

func (d *hwDisplay) Tapped(*fyne.PointEvent) {
	if d.onTap != nil {
		d.onTap()
	}
}

func (d *hwDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(rasterOf(
		func() string { return fmt.Sprintf("seg|%d|%s", d.cells, d.text) },
		func(img *image.NRGBA) { drawSevenSeg(img, d.text, d.cells) }, nil))
}

// hwKnob is a rotary control: drag up to raise, down to lower. Its API
// mirrors widget.Slider (Value, OnChanged, OnChangeEnded).
type hwKnob struct {
	widget.BaseWidget
	Value, Min, Max float64
	OnChanged       func(float64)
	OnChangeEnded   func(float64)
	disabled        bool

	dragging  bool
	dragStart float64 // value when the drag began
	dragDY    float32 // vertical movement so far
}

// knobTravel is how many pixels of drag sweep the whole range.
const knobTravel = 200

func newHWKnob(min, max float64) *hwKnob {
	k := &hwKnob{Min: min, Max: max}
	k.ExtendBaseWidget(k)
	return k
}

func (k *hwKnob) Disable()       { k.disabled = true; k.Refresh() }
func (k *hwKnob) Enable()        { k.disabled = false; k.Refresh() }
func (k *hwKnob) Disabled() bool { return k.disabled }

// SetValue moves the knob and notifies OnChanged.
func (k *hwKnob) SetValue(v float64) {
	v = math.Max(k.Min, math.Min(k.Max, math.Round(v)))
	if v == k.Value {
		return
	}
	k.Value = v
	k.Refresh()
	if k.OnChanged != nil {
		k.OnChanged(v)
	}
}

func (k *hwKnob) Dragged(e *fyne.DragEvent) {
	if k.disabled {
		return
	}
	if !k.dragging {
		k.dragging, k.dragStart, k.dragDY = true, k.Value, 0
	}
	k.dragDY += e.Dragged.DY
	k.SetValue(k.dragStart - float64(k.dragDY)/knobTravel*(k.Max-k.Min))
}

func (k *hwKnob) DragEnd() {
	if k.dragging && k.OnChangeEnded != nil {
		k.OnChangeEnded(k.Value)
	}
	k.dragging = false
}

// The knob cap is one cached image per size; the value arc and pointer are
// vector objects, so turning the knob draws no image at all.
func (k *hwKnob) CreateRenderer() fyne.WidgetRenderer {
	r := &knobRenderer{
		k:       k,
		body:    staticRaster("knobcap", drawKnobBody),
		track:   &canvas.Arc{FillColor: hwAmberDim, StartAngle: knobStartAngle, EndAngle: knobStartAngle + knobSweep},
		value:   &canvas.Arc{FillColor: hwAmber, StartAngle: knobStartAngle},
		pointer: &canvas.Line{StrokeColor: hwSilk},
	}
	r.track.CutoutRatio = knobArcInner / knobArcOuter
	r.value.CutoutRatio = r.track.CutoutRatio
	r.Refresh()
	return r
}

type knobRenderer struct {
	k            *hwKnob
	body         *canvas.Raster
	track, value *canvas.Arc
	pointer      *canvas.Line
}

func (r *knobRenderer) position() float64 {
	if r.k.disabled || r.k.Max == r.k.Min {
		return 0
	}
	return (r.k.Value - r.k.Min) / (r.k.Max - r.k.Min)
}

func (r *knobRenderer) Layout(size fyne.Size) {
	r.body.Resize(size)
	c := fyne.NewPos(size.Width/2, size.Height/2)
	rOuter := min(size.Width, size.Height)/2 - 1
	d := 2 * rOuter * knobArcOuter
	for _, a := range []*canvas.Arc{r.track, r.value} {
		a.Resize(fyne.NewSize(d, d))
		a.Move(c.SubtractXY(d/2, d/2)) // positioned by its top-left corner when painted
	}
	angle := (knobStartAngle + knobSweep*r.position()) * math.Pi / 180
	dx, dy := float32(math.Sin(angle)), float32(-math.Cos(angle))
	capR := rOuter * knobCapRatio
	r.pointer.Position1 = c.AddXY(dx*capR*0.25, dy*capR*0.25)
	r.pointer.Position2 = c.AddXY(dx*capR*0.68, dy*capR*0.68)
	r.pointer.StrokeWidth = max(1, capR*0.08)
}

func (r *knobRenderer) MinSize() fyne.Size { return fyne.NewSize(48, 48) }

func (r *knobRenderer) Refresh() {
	r.value.EndAngle = float32(knobStartAngle + knobSweep*r.position())
	r.Layout(r.k.Size())
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

func (r *knobRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.value, r.body, r.pointer}
}

func (r *knobRenderer) Destroy() {}

// scaledLayout places objects at fixed design coordinates and scales the
// whole panel uniformly to fit, centred. Text sizes scale too.
type scaledLayout struct {
	base  fyne.Size
	items []placed
}

type placed struct {
	o          fyne.CanvasObject
	x, y, w, h float32
	textSize   float32 // for *canvas.Text
}

func newScaledLayout(base fyne.Size) *scaledLayout { return &scaledLayout{base: base} }

// place puts o at x, y with size w×h in design units (text keeps its own size).
func (l *scaledLayout) place(o fyne.CanvasObject, x, y, w, h float32) {
	p := placed{o: o, x: x, y: y, w: w, h: h}
	if t, ok := o.(*canvas.Text); ok {
		p.textSize = t.TextSize
	}
	l.items = append(l.items, p)
}

func (l *scaledLayout) objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, len(l.items))
	for i, p := range l.items {
		out[i] = p.o
	}
	return out
}

func (l *scaledLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	s := min(size.Width/l.base.Width, size.Height/l.base.Height)
	ox, oy := (size.Width-l.base.Width*s)/2, (size.Height-l.base.Height*s)/2
	for _, p := range l.items {
		p.o.Move(fyne.NewPos(ox+p.x*s, oy+p.y*s))
		if t, ok := p.o.(*canvas.Text); ok {
			t.TextSize = p.textSize * s
			t.Resize(t.MinSize())
			continue
		}
		p.o.Resize(fyne.NewSize(p.w*s, p.h*s))
		if sc, ok := p.o.(scalable); ok {
			sc.setScale(s)
		}
	}
}

// minPanelScale is how far the panel may shrink below its design size.
const minPanelScale = 0.7

func (l *scaledLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.base.Width*minPanelScale, l.base.Height*minPanelScale)
}
