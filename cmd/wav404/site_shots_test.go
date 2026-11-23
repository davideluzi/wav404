package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
)

// TestRenderSiteShots renders the screenshots used by the website, from
// the synthetic fixture card, at 2× for sharp images:
//
//	WAV404_SITE_SHOTS=$PWD/assets/screenshots go test ./cmd/wav404 -run SiteShots
func TestRenderSiteShots(t *testing.T) {
	dir := os.Getenv("WAV404_SITE_SHOTS")
	if dir == "" {
		t.Skip("WAV404_SITE_SHOTS not set")
	}
	shot := func(g *gui, name string) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, g.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	setup := func() *gui {
		g, _ := newTestGUI(t)
		fyne.CurrentApp().Settings().SetTheme(wavTheme{})
		g.win.SetPadded(false)
		g.win.Canvas().(interface{ SetScale(float32) }).SetScale(2)
		g.win.Resize(fyne.NewSize(1040, 604))
		selectJ5(g)
		// show the usual mount point, not the test's temp folder
		g.cards.Options = []string{"/Volumes/SP-404SX"}
		g.cards.Selected = "/Volumes/SP-404SX"
		g.cards.Refresh()
		return g
	}

	g := setup()
	shot(g, "main")

	g = setup()
	doubleTap(g.tiles[4])
	shot(g, "playing")

	g = setup()
	g.dragOver(center(g.tiles[7]))
	shot(g, "drop")

	g = setup()
	release := make(chan struct{})
	g.runInBackground("Importing…", func(report progressFunc) error {
		report(0.62, "J9 ← breakbeat_92bpm.wav")
		<-release
		return nil
	}, func(error) {})
	time.Sleep(200 * time.Millisecond)
	shot(g, "import")
	close(release)
}
