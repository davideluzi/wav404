package main

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// rasterOf wraps a drawing function as a canvas object that redraws at the
// on-screen pixel size.
func rasterOf(draw func(img *image.NRGBA)) *canvas.Raster {
	return canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw(img)
		return img
	})
}

// silk is panel lettering ("BANK", "SAMPLE MODE", captions).
func silk(text string, size float32, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// hwKey is a rubber key (small, with a word on it) or a pad (large, with a
// number and a line of detail). When lit its face glows amber.
type hwKey struct {
	widget.BaseWidget
	label, detail string
	lit, ring     bool
	full          bool // pad holds a sample (lighter rubber)
	labelSize     float32
	radius        float64
}

func newHWKey(label string, labelSize float32, radius float64) *hwKey {
	k := &hwKey{label: label, labelSize: labelSize, radius: radius}
	k.ExtendBaseWidget(k)
	return k
}

func (k *hwKey) style() keyStyle {
	s := keyStyle{face: hwRubber, radius: k.radius}
	if k.full {
		s.face = hwRubberFull
	}
	if k.lit {
		s.face, s.glow = hwAmber, 1
	}
	if k.ring {
		c := hwAmber
		s.ring = &c
	}
	return s
}

func (k *hwKey) CreateRenderer() fyne.WidgetRenderer {
	r := &hwKeyRenderer{k: k, label: silk("", k.labelSize, hwSilk), detail: silk("", 11, hwSilkDim)}
	r.body = rasterOf(func(img *image.NRGBA) { drawKey(img, k.style()) })
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
	r.label.Color, r.detail.Color = hwSilk, hwSilkDim
	if k.lit {
		r.label.Color, r.detail.Color = color.NRGBA{0x2a, 0x1a, 0x00, 0xff}, color.NRGBA{0x4a, 0x30, 0x00, 0xff}
	} else if !k.full && k.detail != "" {
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

// hwDisplay is an amber 7-segment readout.
type hwDisplay struct {
	widget.BaseWidget
	text  string
	cells int
}

func newHWDisplay(cells int) *hwDisplay {
	d := &hwDisplay{cells: cells}
	d.ExtendBaseWidget(d)
	return d
}

func (d *hwDisplay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(rasterOf(func(img *image.NRGBA) { drawSevenSeg(img, d.text, d.cells) }))
}

// hwKnob is a rotary control for a 0..1 value.
type hwKnob struct {
	widget.BaseWidget
	value float64
}

func newHWKnob() *hwKnob {
	k := &hwKnob{}
	k.ExtendBaseWidget(k)
	return k
}

func (k *hwKnob) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(rasterOf(func(img *image.NRGBA) { drawKnob(img, k.value) }))
}
