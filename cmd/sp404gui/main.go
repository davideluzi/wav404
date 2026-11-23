// Command sp404gui is the desktop app for managing SP-404SX sample cards.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	a := app.NewWithID("app.wav404")
	a.Settings().SetTheme(wavTheme{})
	w := a.NewWindow("wav404")
	g := newGUI(w)
	w.SetContent(g.content())
	w.Resize(fyne.NewSize(panelW, panelH+52)) // panel + top strip
	g.scanCards()
	w.ShowAndRun()
}
