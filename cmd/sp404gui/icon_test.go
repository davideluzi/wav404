package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"testing"
)

// TestRenderIcon draws the app icon (1024×1024, macOS proportions: an
// 824 px rounded square on a transparent canvas) with the panel's own
// graphics: brushed metal and four rubber pads, one lit amber.
//
//	WAV404_ICON=$PWD/cmd/sp404gui/Icon.png go test ./cmd/sp404gui -run Icon
func TestRenderIcon(t *testing.T) {
	out := os.Getenv("WAV404_ICON")
	if out == "" {
		t.Skip("WAV404_ICON not set")
	}
	const size, body, radius = 1024, 824, 185
	icon := image.NewNRGBA(image.Rect(0, 0, size, size))
	c := float64(size) / 2
	half := float64(body) / 2
	inBody := func(x, y float64) float64 { return sdRoundRect(x, y, c, c, half, half, radius) }

	// soft drop shadow, then the metal body
	fillShape(icon, func(x, y float64) float64 { return sdRoundRect(x, y, c, c+10, half, half, radius) - 8 },
		func(x, y float64) color.NRGBA { return color.NRGBA{0, 0, 0, 0x50} })
	metal := image.NewNRGBA(icon.Rect)
	drawBrushedMetal(metal)
	fillShape(icon, inBody, func(x, y float64) color.NRGBA { return metal.NRGBAAt(int(x), int(y)) })
	// thin light rim
	fillShape(icon, func(x, y float64) float64 { return math.Abs(inBody(x, y)+2) - 2 },
		func(x, y float64) color.NRGBA { return color.NRGBA{255, 255, 255, 0x22} })

	// 2×2 pads, the top-left one lit
	const pad, gap = 300, 36
	origin := (size - (2*pad + gap)) / 2
	for i := range 4 {
		key := image.NewNRGBA(image.Rect(0, 0, pad, pad))
		s := keyStyle{face: hwRubberFull, radius: 0.12, inset: 0.03}
		if i == 0 {
			s.face, s.glow = hwAmber, 1
		}
		drawKey(key, s)
		x, y := origin+(i%2)*(pad+gap), origin+(i/2)*(pad+gap)
		draw.Draw(icon, image.Rect(x, y, x+pad, y+pad), key, image.Point{}, draw.Over)
	}

	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, icon); err != nil {
		t.Fatal(err)
	}
}
