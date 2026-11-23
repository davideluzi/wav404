package main

// Software rendering for the hardware-style panel: anti-aliased rounded
// shapes, brushed metal, a 7-segment display and a knob. Everything is
// drawn in code (no bitmaps), so the look is the same on every platform.

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Hardware palette: graphite metal, dark rubber, amber light.
var (
	hwMetal      = color.NRGBA{0x2a, 0x2d, 0x32, 0xff}
	hwRubber     = color.NRGBA{0x3a, 0x3e, 0x45, 0xff}
	hwRubberFull = color.NRGBA{0x52, 0x57, 0x5f, 0xff}
	hwPadEmpty   = color.NRGBA{0x30, 0x33, 0x39, 0xff} // a pad with no sample
	hwAmber      = color.NRGBA{0xff, 0xb0, 0x20, 0xff}
	hwAmberDeep  = color.NRGBA{0xe0, 0x88, 0x00, 0xff}
	hwAmberDim   = color.NRGBA{0x4a, 0x36, 0x14, 0xff}
	hwAmberGhost = color.NRGBA{0x2e, 0x22, 0x0d, 0xff} // unlit LED segment
	hwGlass      = color.NRGBA{0x10, 0x0d, 0x0a, 0xff}
	hwSilk       = color.NRGBA{0xc4, 0xc8, 0xce, 0xff}
	hwSilkDim    = color.NRGBA{0x8a, 0x90, 0x98, 0xff}
)

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// mix blends a towards b by t (0..1), alpha included.
func mix(a, b color.NRGBA, t float64) color.NRGBA {
	t = clamp01(t)
	f := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*t)) }
	return color.NRGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), f(a.A, b.A)}
}

func shade(c color.NRGBA, k float64) color.NRGBA {
	if k >= 0 {
		return mix(c, color.NRGBA{255, 255, 255, c.A}, k)
	}
	return mix(c, color.NRGBA{0, 0, 0, c.A}, -k)
}

// blend draws c over the pixel at (x, y) with coverage a (0..1).
func blend(img *image.NRGBA, x, y int, c color.NRGBA, a float64) {
	if a <= 0 || !(image.Point{x, y}.In(img.Rect)) {
		return
	}
	a *= float64(c.A) / 255
	i := img.PixOffset(x, y)
	p := img.Pix[i : i+4 : i+4]
	da := float64(p[3]) / 255
	oa := a + da*(1-a)
	if oa == 0 {
		return
	}
	for k, v := range []uint8{c.R, c.G, c.B} {
		p[k] = uint8(math.Round((float64(v)*a + float64(p[k])*da*(1-a)) / oa))
	}
	p[3] = uint8(math.Round(oa * 255))
}

// sdRoundRect is the signed distance from (px, py) to a rounded rectangle
// centred at (cx, cy) with half sizes hw, hh and corner radius r.
func sdRoundRect(px, py, cx, cy, hw, hh, r float64) float64 {
	qx := math.Abs(px-cx) - hw + r
	qy := math.Abs(py-cy) - hh + r
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

// fillShape paints every pixel inside sd (distance < 0) with col(x, y),
// anti-aliased over one pixel.
func fillShape(img *image.NRGBA, sd func(x, y float64) float64, col func(x, y float64) color.NRGBA) {
	fillShapeIn(img, img.Rect, sd, col)
}

// fillShapeIn is fillShape limited to the pixels of rect (the shape's
// bounding box), for small shapes in a large image.
func fillShapeIn(img *image.NRGBA, rect image.Rectangle, sd func(x, y float64) float64, col func(x, y float64) color.NRGBA) {
	b := rect.Intersect(img.Rect)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			if a := clamp01(0.5 - sd(px, py)); a > 0 {
				blend(img, x, y, col(px, py), a)
			}
		}
	}
}

// hash gives a stable pseudo-random value in [0, 1) for integer coords.
func hash(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	return float64(h^(h>>16)) / float64(1<<32)
}

// drawBrushedMetal fills img with horizontally brushed graphite and a soft
// vignette.
func drawBrushedMetal(img *image.NRGBA) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := range h {
		row := hash(0, y)*0.06 - 0.03
		for x := range w {
			streak := (hash(x/9, y) - 0.5) * 0.05
			fine := (hash(x, y) - 0.5) * 0.025
			dx, dy := float64(x)/float64(w)-0.5, float64(y)/float64(h)-0.5
			vignette := -0.18 * (dx*dx + dy*dy) * 2
			img.SetNRGBA(x, y, shade(hwMetal, row+streak+fine+vignette))
		}
	}
}

// keyStyle describes a rubber key or pad. Radius and inset are fractions
// of the key's smaller side, so the look is the same at every resolution
// (and a vector outline drawn over it lines up).
type keyStyle struct {
	face   color.NRGBA // base colour of the key top
	radius float64
	inset  float64
	glow   float64 // 0..1 amber halo around the key (lit / playing)
}

// faceRect returns the inset and corner radius of the key face for a key
// whose smaller side is m (pixels or points).
func (s keyStyle) faceRect(m float64) (inset, radius float64) { return s.inset * m, s.radius * m }

// drawKey renders a rubber key filling the image: a recess shadow, a
// slightly domed face with a top highlight, and an optional glow.
func drawKey(img *image.NRGBA, s keyStyle) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	inset, r := s.faceRect(math.Min(w, h))
	cx, cy := w/2, h/2
	hw, hh := w/2-inset, h/2-inset
	r = math.Min(r, math.Min(hw, hh))

	if s.glow > 0 { // halo outside the key
		for y := 0; y < int(h); y++ {
			for x := 0; x < int(w); x++ {
				d := sdRoundRect(float64(x)+0.5, float64(y)+0.5, cx, cy, hw, hh, r)
				if d > 0 {
					blend(img, x, y, hwAmber, s.glow*0.45*math.Exp(-d/2.2))
				}
			}
		}
	}
	// recess: a dark rim the key sits in
	fillShape(img, func(x, y float64) float64 { return sdRoundRect(x, y, cx, cy+1, hw+1.5, hh+1.5, r+1.5) },
		func(x, y float64) color.NRGBA { return color.NRGBA{0, 0, 0, 0x90} })
	// face: vertical gradient, lighter at the top, plus a soft dome
	fillShape(img, func(x, y float64) float64 { return sdRoundRect(x, y, cx, cy, hw, hh, r) },
		func(x, y float64) color.NRGBA {
			t := (y - (cy - hh)) / (2 * hh)
			c := shade(s.face, 0.10-0.22*t)
			dx, dy := (x-cx)/hw, (y-cy)/hh
			return shade(c, 0.06*(1-clamp01(dx*dx+dy*dy)))
		})
	// top edge highlight
	fillShape(img, func(x, y float64) float64 {
		return math.Max(sdRoundRect(x, y, cx, cy, hw, hh, r), -(sdRoundRect(x, y, cx, cy+1.2, hw, hh, r)))
	}, func(x, y float64) color.NRGBA { return color.NRGBA{255, 255, 255, 0x30} })
}

// Seven-segment layout: a, b, c, d, e, f, g.
var segDigits = map[rune][7]bool{
	'0': {true, true, true, true, true, true, false},
	'1': {false, true, true, false, false, false, false},
	'2': {true, true, false, true, true, false, true},
	'3': {true, true, true, true, false, false, true},
	'4': {false, true, true, false, false, true, true},
	'5': {true, false, true, true, false, true, true},
	'6': {true, false, true, true, true, true, true},
	'7': {true, true, true, false, false, false, false},
	'8': {true, true, true, true, true, true, true},
	'9': {true, true, true, true, false, true, true},
	'-': {false, false, false, false, false, false, true},
	' ': {},
}

// drawSevenSeg renders text (digits, '-', ' ' and '.') right-aligned in
// `cells` digit cells, with unlit segments faintly visible like a real LED.
func drawSevenSeg(img *image.NRGBA, text string, cells int) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	fillShape(img, func(x, y float64) float64 { return sdRoundRect(x, y, w/2, h/2, w/2-1, h/2-1, 6) },
		func(x, y float64) color.NRGBA { return shade(hwGlass, 0.05*(1-y/h)) })

	type cell struct {
		r   rune
		dot bool
	}
	var cs []cell
	for _, r := range text {
		if r == '.' && len(cs) > 0 {
			cs[len(cs)-1].dot = true
			continue
		}
		cs = append(cs, cell{r: r})
	}
	for len(cs) < cells {
		cs = append([]cell{{r: ' '}}, cs...)
	}
	pad := h * 0.16
	dh := h - 2*pad
	dw := dh * 0.55
	gap := dw * 0.42
	total := float64(cells)*dw + float64(cells-1)*gap
	x0 := (w - total) / 2
	t := dh * 0.13       // segment thickness
	g := t * 0.14        // gap at the joints
	slant := 0.07        // italic lean
	hx := dw/2 - t/2 - g // half length of a horizontal segment
	vy := dh/4 - t/4 - g // half length of a vertical segment
	up, lo := dh/4+t/4, 3*dh/4-t/4
	for i, c := range cs[len(cs)-cells:] {
		ox := x0 + float64(i)*(dw+gap)
		on := segDigits[c.r]
		// segment boxes a–g: centre x, centre y, half w, half h
		segs := [7][4]float64{
			{dw / 2, t / 2, hx, t / 2},
			{dw - t/2, up, t / 2, vy},
			{dw - t/2, lo, t / 2, vy},
			{dw / 2, dh - t/2, hx, t / 2},
			{t / 2, lo, t / 2, vy},
			{t / 2, up, t / 2, vy},
			{dw / 2, dh / 2, hx, t / 2},
		}
		for k, sg := range segs {
			col := hwAmberGhost
			if on[k] {
				col = hwAmber
			}
			lean := dh * slant
			box := image.Rect(int(ox+sg[0]-sg[2]-2), int(pad+sg[1]-sg[3]-2),
				int(ox+sg[0]+sg[2]+lean+3), int(pad+sg[1]+sg[3]+3))
			fillShapeIn(img, box, func(x, y float64) float64 {
				lx := x - ox - (dh-(y-pad))*slant // undo the lean
				return sdRoundRect(lx, y-pad, sg[0], sg[1], sg[2], sg[3], t/2)
			}, func(x, y float64) color.NRGBA { return col })
		}
		dotCol := hwAmberGhost
		if c.dot {
			dotCol = hwAmber
		}
		dx, dy := ox+dw+gap*0.45, pad+dh-t/2
		fillShapeIn(img, image.Rect(int(dx-t-2), int(dy-t-2), int(dx+t+3), int(dy+t+3)), func(x, y float64) float64 {
			return math.Hypot(x-dx, y-dy) - t*0.55
		}, func(x, y float64) color.NRGBA { return dotCol })
	}
}

// drawKnobBody renders the knob cap with its shadow; the value arc and
// the pointer are vector objects drawn over it.
func drawKnobBody(img *image.NRGBA) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	cx, cy := w/2, h/2
	rKnob := (math.Min(w, h)/2 - 2) * knobCapRatio
	fillShape(img, func(x, y float64) float64 { return math.Hypot(x-cx, y-cy-2) - rKnob },
		func(x, y float64) color.NRGBA { return color.NRGBA{0, 0, 0, 0xa0} })
	fillShape(img, func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - rKnob },
		func(x, y float64) color.NRGBA {
			t := (y - (cy - rKnob)) / (2 * rKnob)
			return shade(color.NRGBA{0x22, 0x24, 0x28, 0xff}, 0.18-0.25*t)
		})
	fillShape(img, func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - rKnob*0.78 },
		func(x, y float64) color.NRGBA {
			t := (y - (cy - rKnob)) / (2 * rKnob)
			return shade(color.NRGBA{0x2c, 0x2f, 0x34, 0xff}, 0.12-0.2*t)
		})
}

// Knob geometry, as fractions of the knob's outer radius.
const (
	knobCapRatio   = 0.72 // cap radius
	knobArcOuter   = 0.96 // value arc, outer edge
	knobArcInner   = 0.84 // value arc, inner edge
	knobStartAngle = -135 // degrees from 12 o'clock, clockwise
	knobSweep      = 270
)

// drawScrew renders a small panel screw with a slot.
func drawScrew(img *image.NRGBA) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	cx, cy, r := w/2, h/2, math.Min(w, h)/2-1
	fillShape(img, func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - r },
		func(x, y float64) color.NRGBA {
			t := (y - (cy - r)) / (2 * r)
			return shade(color.NRGBA{0x6a, 0x6e, 0x75, 0xff}, 0.3-0.5*t)
		})
	fillShape(img, func(x, y float64) float64 {
		// slot at 30°
		ux, uy := math.Cos(0.52), math.Sin(0.52)
		px, py := x-cx, y-cy
		along, across := px*ux+py*uy, -px*uy+py*ux
		return math.Max(math.Abs(along)-r*0.75, math.Abs(across)-r*0.14)
	}, func(x, y float64) color.NRGBA { return color.NRGBA{0x20, 0x22, 0x25, 0xff} })
}

// drawScreen renders a recessed dark glass window (for the waveform).
func drawScreen(img *image.NRGBA) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	fillShape(img, func(x, y float64) float64 { return sdRoundRect(x, y, w/2, h/2, w/2-1, h/2-1, 8) },
		func(x, y float64) color.NRGBA { return color.NRGBA{0, 0, 0, 0xb0} })
	fillShape(img, func(x, y float64) float64 { return sdRoundRect(x, y, w/2, h/2+1, w/2-4, h/2-4, 6) },
		func(x, y float64) color.NRGBA { return shade(hwGlass, 0.04*(1-y/h)) })
}

// Drawn images are cached: a key's look depends only on its size and state,
// and the panel shows few distinct states, so most refreshes cost nothing.
// The cache is bounded in bytes; when full it starts over.
var (
	cacheMu    sync.Mutex
	drawCache  = map[string]*image.NRGBA{}
	cacheBytes int
	// For tests: images drawn while the driver was waiting for them (the
	// cost a user feels), in total and per kind. Background prewarming is
	// counted apart.
	draws        = map[string]int64{}
	drawCount    atomic.Int64
	prewarmCount atomic.Int64
)

const maxCacheBytes = 96 << 20

func cacheKey(key string, w, h int) string { return fmt.Sprintf("%s|%dx%d", key, w, h) }

// cached returns the w×h image for key, drawing it on first use. Callers
// must not modify the returned image.
func cached(key string, w, h int, draw func(*image.NRGBA)) *image.NRGBA {
	return cachedDraw(key, w, h, draw, false)
}

func cachedDraw(key string, w, h int, draw func(*image.NRGBA), background bool) *image.NRGBA {
	k := cacheKey(key, w, h)
	cacheMu.Lock()
	img, ok := drawCache[k]
	cacheMu.Unlock()
	if ok {
		return img
	}
	img = image.NewNRGBA(image.Rect(0, 0, w, h))
	draw(img)
	cacheMu.Lock()
	if cacheBytes+len(img.Pix) > maxCacheBytes {
		clear(drawCache)
		cacheBytes = 0
	}
	drawCache[k] = img
	cacheBytes += len(img.Pix)
	if background {
		prewarmCount.Add(1)
	} else {
		drawCount.Add(1)
		draws[strings.SplitN(key, "|", 2)[0]]++
	}
	cacheMu.Unlock()
	return img
}

func isCached(key string, w, h int) bool {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	_, ok := drawCache[cacheKey(key, w, h)]
	return ok
}

// drawsOf counts the images of one kind ("key", "seg", …) drawn so far.
func drawsOf(kind string) int64 {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	return draws[kind]
}

// key identifies a keyStyle in the image cache.
func (s keyStyle) key() string {
	return fmt.Sprintf("key|%v|%.3f|%.3f|%.2f", s.face, s.radius, s.inset, s.glow)
}

// A variant is another look of a raster, drawn ahead of time.
type variant struct {
	key  string
	draw func(*image.NRGBA)
}

// Variants are prepared in the background once the UI has been quiet for
// prewarmDelay, at the exact pixel size the driver last asked for, so that
// the first press of a key or pad finds its image ready. While the window
// is being resized only the final size is prepared.
var prewarm = struct {
	sync.Mutex
	pending map[any]prewarmJob
	timer   *time.Timer
}{pending: map[any]prewarmJob{}}

type prewarmJob struct {
	w, h     int
	variants []variant
}

const prewarmDelay = 200 * time.Millisecond

func notePrewarm(owner any, w, h int, vs []variant) {
	prewarm.Lock()
	defer prewarm.Unlock()
	prewarm.pending[owner] = prewarmJob{w, h, vs}
	if prewarm.timer == nil {
		prewarm.timer = time.AfterFunc(prewarmDelay, runPrewarm)
	} else {
		prewarm.timer.Reset(prewarmDelay)
	}
}

func runPrewarm() {
	prewarm.Lock()
	jobs := prewarm.pending
	prewarm.pending = map[any]prewarmJob{}
	prewarm.Unlock()
	for _, j := range jobs {
		for _, v := range j.variants {
			cachedDraw(v.key, j.w, j.h, v.draw, true)
		}
	}
}

// flushPrewarm prepares the pending variants now (tests).
func flushPrewarm() {
	prewarm.Lock()
	if prewarm.timer != nil {
		prewarm.timer.Stop()
	}
	prewarm.Unlock()
	runPrewarm()
}
