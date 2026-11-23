package main

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// padTile is one flat pad: name, length/tempo and flags. The selected pad
// gets an accent outline, the playing pad an accent tint.
//
// It deliberately does not implement fyne.DoubleTappable: the driver would
// then hold back every single tap for the whole double-click interval.
// Double clicks are detected here instead, so selection is immediate.
type padTile struct {
	widget.BaseWidget
	tile              tile
	selected, playing bool
	onTap, onDouble   func()

	now     func() time.Time // replaceable in tests
	lastTap time.Time
}

func newPadTile(onTap, onDouble func()) *padTile {
	p := &padTile{onTap: onTap, onDouble: onDouble, now: time.Now}
	p.ExtendBaseWidget(p)
	return p
}

func (p *padTile) set(t tile, selected, playing bool) {
	p.tile, p.selected, p.playing = t, selected, playing
	p.Refresh()
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

func (p *padTile) CreateRenderer() fyne.WidgetRenderer {
	r := &padRenderer{
		p:      p,
		bg:     canvas.NewRectangle(colPadEmpty),
		title:  canvas.NewText("", colText),
		detail: canvas.NewText("", colMuted),
		flags:  canvas.NewText("", colAccent),
	}
	r.bg.CornerRadius = 6
	r.title.TextStyle = fyne.TextStyle{Bold: true}
	r.title.TextSize = 16
	r.detail.TextSize = 12
	r.flags.TextSize = 11
	r.Refresh()
	return r
}

type padRenderer struct {
	p                    *padTile
	bg                   *canvas.Rectangle
	title, detail, flags *canvas.Text
}

func (r *padRenderer) Layout(size fyne.Size) {
	const pad = 10
	r.bg.Resize(size)
	r.title.Move(fyne.NewPos(pad, pad))
	r.detail.Move(fyne.NewPos(pad, size.Height-pad-r.detail.MinSize().Height-r.flags.MinSize().Height))
	r.flags.Move(fyne.NewPos(pad, size.Height-pad-r.flags.MinSize().Height))
}

func (r *padRenderer) MinSize() fyne.Size { return fyne.NewSize(130, 90) }

func (r *padRenderer) Refresh() {
	t := r.p.tile
	r.title.Text, r.detail.Text, r.flags.Text = t.title, t.detail, t.flags
	r.bg.FillColor = colPadFull
	r.title.Color = colText
	if t.empty {
		r.bg.FillColor = colPadEmpty
		r.title.Color = colMuted
	}
	if r.p.playing {
		r.bg.FillColor = colPadPlaying
	}
	r.bg.StrokeWidth = 0
	if r.p.selected {
		r.bg.StrokeColor, r.bg.StrokeWidth = colAccent, 2
	}
	r.Layout(r.p.Size())
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

func (r *padRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.title, r.detail, r.flags}
}

func (r *padRenderer) Destroy() {}
