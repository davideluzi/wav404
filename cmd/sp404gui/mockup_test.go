package main

import (
	"image/png"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/davideluzi/wav404/sp404sx"
)

// TestRenderHardwareMockup writes the mockup to a PNG.
// WAV404_MOCKUP=/path/out.png go test ./cmd/sp404gui -run Mockup
func TestRenderHardwareMockup(t *testing.T) {
	out := os.Getenv("WAV404_MOCKUP")
	if out == "" {
		t.Skip("WAV404_MOCKUP not set")
	}
	a := test.NewTempApp(t)
	a.Settings().SetTheme(wavTheme{})
	c, err := sp404sx.OpenCard(fakeCard(t))
	if err != nil {
		t.Fatal(err)
	}
	w := a.NewWindow("mockup")
	w.SetPadded(false)
	w.SetContent(hardwareMockup(c))
	w.Resize(fyne.NewSize(mockW, mockH))
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
