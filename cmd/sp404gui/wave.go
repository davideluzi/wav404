package main

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/davideluzi/wav404/sp404sx"
)

// peak is the sample range of one waveform column, in [-1, 1].
type peak struct{ min, max float32 }

// peaks splits the sample into columns and returns the min/max of each,
// over all channels. Every frame lands in a column; with more columns than
// frames, each column shows the frame under it.
func peaks(s *sp404sx.Sample, columns int) []peak {
	if s == nil || columns <= 0 || s.Frames() == 0 {
		return nil
	}
	frames, ch := s.Frames(), s.Channels
	out := make([]peak, columns)
	for c := range out {
		from := c * frames / columns
		to := max((c+1)*frames/columns, from+1)
		lo, hi := s.PCM[from*ch], s.PCM[from*ch]
		for _, v := range s.PCM[from*ch : to*ch] {
			lo, hi = min(lo, v), max(hi, v)
		}
		out[c] = peak{float32(lo) / 32768, float32(hi) / 32768}
	}
	return out
}

// wavePalette colours the waveform: background, audio inside and outside
// the trim range, and the markers.
type wavePalette struct{ bg, in, out, marker color.Color }

var flatWave = wavePalette{colPadEmpty, colAccent, colMuted, colText}

// waveView draws a sample with its trim range. Dragging moves the marker
// nearest to where the drag started; onTrim runs when the drag ends.
type waveView struct {
	widget.BaseWidget
	sample     *sp404sx.Sample
	start, end int // trim, in frames
	minGap     int // smallest range that survives 4-byte alignment
	onTrim     func(start, end int)
	pal        wavePalette

	dragging int // 0 none, 1 start marker, 2 end marker

	cacheW     int // peaks cached for this pixel width
	cachePeaks []peak
}

func newWaveView(onTrim func(start, end int)) *waveView {
	w := &waveView{onTrim: onTrim, pal: flatWave}
	w.ExtendBaseWidget(w)
	return w
}

// setSample shows s (nil: nothing) with the trim range start–end.
func (w *waveView) setSample(s *sp404sx.Sample, start, end int) {
	w.sample, w.start, w.end, w.dragging = s, start, end, 0
	w.cacheW, w.cachePeaks = 0, nil
	w.minGap = 1
	if s != nil && s.Channels == 1 {
		w.minGap = 2 // mono offsets move in steps of 2 frames
	}
	w.Refresh()
}

func (w *waveView) frames() int {
	if w.sample == nil {
		return 0
	}
	return w.sample.Frames()
}

func (w *waveView) frameAt(x float32) int {
	f := int(math.Round(float64(x) / float64(w.Size().Width) * float64(w.frames())))
	return max(0, min(f, w.frames()))
}

func (w *waveView) xOf(frame int) float32 {
	return float32(frame) / float32(w.frames()) * w.Size().Width
}

func (w *waveView) Dragged(e *fyne.DragEvent) {
	if w.frames() == 0 {
		return
	}
	if w.dragging == 0 {
		x0 := e.Position.X - e.Dragged.DX // where the drag began
		w.dragging = 2
		if abs32(x0-w.xOf(w.start)) <= abs32(x0-w.xOf(w.end)) {
			w.dragging = 1
		}
	}
	f := w.frameAt(e.Position.X)
	if w.dragging == 1 {
		w.start = max(0, min(f, w.end-w.minGap))
	} else {
		w.end = min(w.frames(), max(f, w.start+w.minGap))
	}
	w.Refresh()
}

func (w *waveView) DragEnd() {
	if w.dragging != 0 && w.sample != nil {
		w.onTrim(w.start, w.end)
	}
	w.dragging = 0
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (w *waveView) MinSize() fyne.Size { return fyne.NewSize(200, 72) }

func (w *waveView) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRaster(w.draw)
	return widget.NewSimpleRenderer(r)
}

// draw renders the waveform: bright inside the trim range, dim outside,
// with the two markers.
func (w *waveView) draw(width, height int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	fill := func(x, y0, y1 int, c color.Color) {
		for y := max(0, y0); y <= min(height-1, y1); y++ {
			img.Set(x, y, c)
		}
	}
	for x := range width {
		fill(x, 0, height-1, w.pal.bg)
	}
	frames := w.frames()
	if frames == 0 || width == 0 {
		return img
	}
	if w.cacheW != width {
		w.cacheW, w.cachePeaks = width, peaks(w.sample, width)
	}
	mid := float32(height-1) / 2
	startX := w.start * width / frames
	endX := min(w.end*width/frames, width-1)
	for x, p := range w.cachePeaks {
		c := w.pal.out
		if x >= startX && x <= endX {
			c = w.pal.in
		}
		fill(x, int(mid-p.max*mid), int(mid-p.min*mid), c)
	}
	// 2 px markers, drawn inwards so they stay visible at both edges
	for _, x := range []int{startX, min(startX+1, width-1), endX, max(endX-1, 0)} {
		fill(x, 0, height-1, w.pal.marker)
	}
	return img
}
