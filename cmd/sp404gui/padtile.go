package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// padTile is one rubber pad: its number, the sample length and flags. The
// selected pad gets an amber ring, the playing pad lights up amber.
//
// It deliberately does not implement fyne.DoubleTappable: the driver would
// then hold back every single tap for the whole double-click interval.
// Double clicks are detected here instead, so selection is immediate.
type padTile struct {
	widget.BaseWidget
	num               int // 1–12, printed on the pad
	tile              tile
	selected, playing bool
	onTap, onDouble   func()
	scale             float32

	now     func() time.Time // replaceable in tests
	lastTap time.Time
}

func newPadTile(num int, onTap, onDouble func()) *padTile {
	p := &padTile{num: num, onTap: onTap, onDouble: onDouble, now: time.Now, scale: 1}
	p.ExtendBaseWidget(p)
	return p
}

// set updates the pad; an unchanged pad is not redrawn.
func (p *padTile) set(t tile, selected, playing bool) {
	if t == p.tile && selected == p.selected && playing == p.playing {
		return
	}
	p.tile, p.selected, p.playing = t, selected, playing
	p.Refresh()
}

// Pad geometry at design scale (points).
const padRadius, padInset = 10, 3

func (p *padTile) style() keyStyle { return p.styleFor(!p.tile.empty, p.playing) }

func (p *padTile) styleFor(full, playing bool) keyStyle {
	s := keyStyle{face: hwPadEmpty}
	s.radius, s.inset = keyGeometry(p.Size(), padRadius, padInset, p.scale)
	if full {
		s.face = hwRubberFull
	}
	if playing {
		s.face, s.glow = hwAmber, 1
	}
	return s
}

// Tapped acts on the first click at once; a second click within the
// system double-click interval also triggers onDouble.
func (p *padTile) Tapped(*fyne.PointEvent) {
	t := p.now()
	double := !p.lastTap.IsZero() && t.Sub(p.lastTap) <= doubleClickDelay()
	p.lastTap = t
	if double {
		p.lastTap = time.Time{} // a third click starts over
		p.onDouble()
		return
	}
	p.onTap()
}

func doubleClickDelay() time.Duration {
	if app := fyne.CurrentApp(); app != nil {
		if d := app.Driver().DoubleTapDelay(); d > 0 {
			return d
		}
	}
	return 300 * time.Millisecond
}

func (p *padTile) setScale(s float32) {
	if p.scale != s {
		p.scale = s
		p.Refresh()
	}
}

func (p *padTile) CreateRenderer() fyne.WidgetRenderer {
	r := &padRenderer{p: p, num: silk("", 26, hwSilk), detail: silk("", 11, hwSilkDim), flags: silk("", 9, hwSilkDim)}
	r.body = rasterOf(func() string { return p.style().key() },
		func(img *image.NRGBA) { drawKey(img, p.style()) },
		func() []variant {
			return keyVariants(p.styleFor(true, false), p.styleFor(true, true), p.styleFor(false, false), p.styleFor(false, true))
		})
	// the selection outline is a vector shape: selecting draws no image
	r.ring = canvas.NewRectangle(color.Transparent)
	r.ring.StrokeColor = hwAmber
	r.Refresh()
	return r
}

type padRenderer struct {
	p                  *padTile
	body               *canvas.Raster
	ring               *canvas.Rectangle
	num, detail, flags *canvas.Text
}

func (r *padRenderer) Layout(size fyne.Size) {
	r.body.Resize(size)
	inset, radius := r.p.style().faceRect(float64(min(size.Width, size.Height)))
	r.ring.Move(fyne.NewPos(float32(inset), float32(inset)))
	r.ring.Resize(fyne.NewSize(size.Width-2*float32(inset), size.Height-2*float32(inset)))
	r.ring.CornerRadius = float32(radius)
	r.ring.StrokeWidth = 2 * r.p.scale
	center := func(t *canvas.Text, y float32) {
		m := t.MinSize()
		t.Move(fyne.NewPos((size.Width-m.Width)/2, size.Height*y-m.Height/2))
	}
	center(r.num, 0.38)
	center(r.detail, 0.66)
	center(r.flags, 0.82)
}

func (r *padRenderer) MinSize() fyne.Size { return fyne.NewSize(90, 70) }

func (r *padRenderer) Refresh() {
	p, t := r.p, r.p.tile
	r.num.Text = fmt.Sprint(p.num)
	r.detail.Text = t.detail
	if !t.empty {
		r.detail.Text = strings.SplitN(t.detail, " · ", 2)[0] // length only
	}
	r.flags.Text = strings.ToUpper(t.flags)
	r.num.TextSize, r.detail.TextSize, r.flags.TextSize = 26*p.scale, 11*p.scale, 9*p.scale

	ink, sub := color.Color(hwSilk), color.Color(hwSilkDim)
	switch {
	case p.playing:
		ink, sub = color.NRGBA{0x2a, 0x1a, 0x00, 0xff}, color.NRGBA{0x4a, 0x30, 0x00, 0xff}
	case t.empty:
		ink = hwSilkDim
	}
	r.num.Color, r.detail.Color, r.flags.Color = ink, sub, sub
	r.ring.Hidden = !p.selected
	r.Layout(p.Size())
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

func (r *padRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.body, r.ring, r.num, r.detail, r.flags}
}

func (r *padRenderer) Destroy() {}
