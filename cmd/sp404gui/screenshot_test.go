package main

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
)

// TestRenderScreenshot writes the main window (fake card, bank J, J5
// selected and playing) to a PNG: WAV404_SCREENSHOT=/path/out.png go test ./cmd/sp404gui -run Screenshot
func TestRenderScreenshot(t *testing.T) {
	out := os.Getenv("WAV404_SCREENSHOT")
	if out == "" {
		t.Skip("WAV404_SCREENSHOT not set")
	}
	g, _ := newTestGUI(t)
	fyne.CurrentApp().Settings().SetTheme(wavTheme{})
	g.win.SetPadded(false)
	selectJ5(g)
	if os.Getenv("WAV404_SCREENSHOT_PLAY") != "" {
		doubleTap(g.tiles[4]) // J5 selected and playing
	}
	size := fyne.NewSize(1040, 604)
	if os.Getenv("WAV404_SCREENSHOT_BIG") != "" {
		size = fyne.NewSize(1500, 860)
	}
	g.win.Resize(size)
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, g.win.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
