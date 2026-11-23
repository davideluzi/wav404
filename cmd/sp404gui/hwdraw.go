package main

// Software rendering for the hardware-style panel: anti-aliased rounded
// shapes, brushed metal, a 7-segment display and a knob. Everything is
// drawn in code (no bitmaps), so the look is the same on every platform.

import (
	"image"
	"image/color"
	"math"
)

// Hardware palette: graphite metal, dark rubber, amber light.
var (
	hwMetal      = color.NRGBA{0x2a, 0x2d, 0x32, 0xff}
	hwRubber     = color.NRGBA{0x3a, 0x3e, 0x45, 0xff}
	hwRubberFull = color.NRGBA{0x52, 0x57, 0x5f, 0xff}
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
	b := img.Rect
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

// keyStyle describes a rubber key or pad.
type keyStyle struct {
	face   color.NRGBA // base colour of the key top
	radius float64
	ring   *color.NRGBA // optional outline (selection)
	glow   float64      // 0..1 amber halo around the key (lit / playing)
}

// drawKey renders a rubber key filling the image: a recess shadow, a
// slightly domed face with a top highlight, an optional ring and glow.
func drawKey(img *image.NRGBA, s keyStyle) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	inset := 4.0
	cx, cy := w/2, h/2
	hw, hh := w/2-inset, h/2-inset
	r := math.Min(s.radius, math.Min(hw, hh))

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
	if s.ring != nil {
		fillShape(img, func(x, y float64) float64 {
			d := sdRoundRect(x, y, cx, cy, hw, hh, r)
			return math.Abs(d+1.2) - 1.2
		}, func(x, y float64) color.NRGBA { return *s.ring })
	}
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
			fillShape(img, func(x, y float64) float64 {
				lx := x - ox - (dh-(y-pad))*slant // undo the lean
				return sdRoundRect(lx, y-pad, sg[0], sg[1], sg[2], sg[3], t/2)
			}, func(x, y float64) color.NRGBA { return col })
		}
		dotCol := hwAmberGhost
		if c.dot {
			dotCol = hwAmber
		}
		fillShape(img, func(x, y float64) float64 {
			return math.Hypot(x-(ox+dw+gap*0.45), y-(pad+dh-t/2)) - t*0.55
		}, func(x, y float64) color.NRGBA { return dotCol })
	}
}

// drawKnob renders a knob for value 0..1 over a 270° sweep, with an amber
// arc showing the value.
func drawKnob(img *image.NRGBA, value float64) {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	cx, cy := w/2, h/2
	rOuter := math.Min(w, h)/2 - 2
	rKnob := rOuter * 0.72
	start, sweep := math.Pi*0.75, math.Pi*1.5 // from 7:30 clockwise to 4:30
	angleOf := func(x, y float64) float64 {
		a := math.Atan2(y-cy, x-cx) - start
		for a < 0 {
			a += 2 * math.Pi
		}
		return a
	}
	arc := func(limit float64, col color.NRGBA) {
		fillShape(img, func(x, y float64) float64 {
			d := math.Abs(math.Hypot(x-cx, y-cy)-rOuter*0.9) - rOuter*0.06
			if angleOf(x, y) > limit {
				return 1
			}
			return d
		}, func(x, y float64) color.NRGBA { return col })
	}
	arc(sweep, hwAmberDim)
	arc(sweep*clamp01(value), hwAmber)
	// shadow, body, rim highlight
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
	// indicator line
	a := start + sweep*clamp01(value)
	ex, ey := cx+math.Cos(a)*rKnob*0.68, cy+math.Sin(a)*rKnob*0.68
	sx, sy := cx+math.Cos(a)*rKnob*0.25, cy+math.Sin(a)*rKnob*0.25
	fillShape(img, func(x, y float64) float64 {
		// distance to segment s–e
		vx, vy := ex-sx, ey-sy
		t := clamp01(((x-sx)*vx + (y-sy)*vy) / (vx*vx + vy*vy))
		return math.Hypot(x-(sx+vx*t), y-(sy+vy*t)) - 2
	}, func(x, y float64) color.NRGBA { return hwSilk })
}

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
