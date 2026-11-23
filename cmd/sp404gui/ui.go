package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"

	"github.com/davideluzi/wav404/sp404sx"
)

const noCardText = "No card open: insert an SP-404SX card or use Open folder…"

type gui struct {
	win      fyne.Window
	card     *sp404sx.Card
	bank     int
	selected int // pad index, -1 = none
	playing  int // pad index being previewed, -1 = none
	dropPad  int // pad under a dragged file, -1 = none
	playGen  int // identifies the current playback; stale end callbacks are ignored
	player   audioPlayer
	picker   picker
	sampleLoader

	tiles                          [sp404sx.PadsPerBank]*padTile
	banks                          *bankSelector
	cards                          *widget.Select
	info                           *canvas.Text
	trim                           *hwToggle // export only the trimmed part
	importBtn, exportBtn, clearBtn *widget.Button
	playBtn                        *hwKey
	padPanel

	// replaceable in tests
	confirm func(title, message, action string, ok func())
	runTask func(label string, task func(progressFunc) error, done func(error))
}

func newGUI(w fyne.Window) *gui {
	g := &gui{win: w, selected: -1, playing: -1, dropPad: -1, player: &otoPlayer{},
		picker: choosePicker(w, zenity.IsAvailable())}
	// a custom dialog: no question-mark icon, and the button says what it does
	g.confirm = func(title, message, action string, ok func()) {
		dialog.ShowCustomConfirm(title, action, "Cancel", widget.NewLabel(message), func(yes bool) {
			if yes {
				ok()
			}
		}, w)
	}
	g.runTask = g.runInBackground
	g.initLoader()

	for i := range g.tiles {
		index := func() int { return g.bank*sp404sx.PadsPerBank + i }
		g.tiles[i] = newPadTile(i+1, func() { g.selectPad(index()) }, func() { g.playPad(index()) })
	}
	g.banks = newBankSelector(func(s string) {
		g.stopPlayback()
		g.bank = int(s[0] - 'A')
		g.dropPad = -1
		g.selected = -1
		g.refresh()
	})

	g.cards = widget.NewSelect(nil, func(root string) {
		if g.card == nil || g.card.Root != root {
			g.showError(g.openCard(root))
		}
	})
	g.cards.PlaceHolder = "No SP-404SX card found"
	g.info = silk(noCardText, 12, hwSilkDim)
	g.trim = newHWToggle("TRIM ONLY", nil)
	g.trim.labelSize = 11
	g.importBtn = widget.NewButtonWithIcon("Import…", theme.DownloadIcon(), g.importDialog)
	g.exportBtn = widget.NewButtonWithIcon("Export…", theme.UploadIcon(), g.exportDialog)
	g.clearBtn = widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), g.clearSelected)
	g.playBtn = newHWKey("PLAY", 18, 9)
	g.playBtn.onTap = g.togglePlay
	g.buildPanel()
	w.SetOnDropped(g.drop)
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeySpace {
			g.togglePlay()
		}
	})
	g.banks.SetSelected("A") // also runs the first refresh
	return g
}

// Design size of the hardware panel; it scales uniformly with the window.
const panelW, panelH = 1040, 552

func (g *gui) content() fyne.CanvasObject {
	openFolder := widget.NewButtonWithIcon("Open folder…", theme.FolderOpenIcon(), g.openFolderDialog)
	rescan := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), g.scanCards)
	strip := container.NewStack(canvas.NewRectangle(colBackground), container.NewPadded(
		container.NewBorder(nil, nil, silk("CARD", 12, hwSilkDim),
			container.NewHBox(rescan, openFolder, widget.NewSeparator(), g.importBtn, g.exportBtn, g.clearBtn),
			g.cards)))

	l := newScaledLayout(fyne.NewSize(panelW, panelH))
	for _, p := range [][2]float32{{12, 12}, {panelW - 28, 12}, {12, panelH - 28}, {panelW - 28, panelH - 28}} {
		l.place(staticRaster("screw", drawScrew), p[0], p[1], 16, 16)
	}
	caption := func(text string, x, y float32) { l.place(silk(text, 11, hwSilkDim), x, y, 0, 0) }

	// left: the rows above the pads, as on the hardware
	caption("SAMPLE MODE", 40, 20)
	for i, k := range []*hwToggle{g.lofi, g.gate, g.loop, g.reverse} {
		l.place(k, 40+float32(i)*108, 38, 98, 42)
	}
	caption("CHANNELS", 514, 20)
	l.place(g.channels, 514, 38, 98, 42)
	caption("BANK", 40, 96)
	for b, k := range g.banks.keys {
		l.place(k, 40+float32(b)*57.6, 114, 52, 42)
	}
	for i, t := range g.tiles {
		l.place(t, 34+float32(i%4)*146, 176+float32(i/4)*118, 140, 112)
	}

	// right: tempo, volume, trim, transport
	caption("BPM", 660, 20)
	l.place(g.bpmDisplay, 660, 38, 220, 92)
	l.place(g.origBPM, 660, 136, 0, 0)
	l.place(g.resetBPM, 812, 132, 68, 24)
	caption("VOLUME", 912, 20)
	l.place(g.volume, 898, 34, 104, 104)
	l.place(g.volumeLabel, 938, 136, 0, 0)
	caption("TRIM", 660, 176)
	l.place(staticRaster("screen", drawScreen), 652, 194, 360, 150)
	l.place(g.wave, 660, 202, 344, 134)
	l.place(g.trimLabel, 660, 352, 0, 0)
	l.place(g.resetTrim, 930, 346, 82, 30)
	l.place(g.playBtn, 652, 400, 170, 64)
	caption("EXPORT", 842, 400)
	l.place(g.trim, 842, 418, 110, 40)
	l.place(g.info, 660, 496, 0, 0)

	panel := container.NewStack(staticRaster("metal", drawBrushedMetal), container.New(l, l.objects()...))
	return container.NewBorder(strip, nil, nil, nil, panel)
}

// scanCards looks for cards on the usual mount points and opens the first.
func (g *gui) scanCards() {
	found := findCards(mountPoints())
	if g.card != nil && !contains(found, g.card.Root) {
		found = append(found, g.card.Root) // keep a manually opened folder
	}
	g.cards.SetOptions(found)
	if g.card == nil && len(found) > 0 {
		g.cards.SetSelected(found[0])
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (g *gui) openCard(root string) error {
	c, err := sp404sx.OpenCard(root)
	if err != nil {
		return err
	}
	g.stopPlayback()
	g.invalidatePanel()
	g.invalidateSamples()
	g.card, g.selected = c, -1
	if !contains(g.cards.Options, root) {
		g.cards.SetOptions(append(g.cards.Options, root))
	}
	g.cards.SetSelected(root)
	g.refresh()
	return nil
}

// selectPad selects a pad; selecting a different pad stops the preview.
func (g *gui) selectPad(index int) {
	if g.card == nil {
		return
	}
	if index != g.playing {
		g.stopPlayback()
	}
	g.selected = index
	g.refresh()
}

// refresh redraws tiles, the info line and the enabled state of actions.
func (g *gui) refresh() {
	g.refreshTiles()
	enable := func(b fyne.Disableable, on bool) {
		if on {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	hasPad := g.card != nil && g.selected >= 0
	assigned := hasPad && !g.card.Info.Pads[g.selected].Empty()
	enable(g.importBtn, hasPad)
	enable(g.exportBtn, assigned)
	enable(g.clearBtn, assigned)
	g.loadPanel()
	enable(g.playBtn, assigned || g.playing >= 0)
	g.playBtn.label, g.playBtn.lit = "PLAY", g.playing >= 0
	if g.playing >= 0 {
		g.playBtn.label = "STOP"
	}
	g.playBtn.Refresh()

	switch {
	case g.card == nil:
		setText(g.info, noCardText)
	case !hasPad:
		setText(g.info, "Select a pad, or drop a WAV/AIFF file onto a pad.")
	case g.card.Info.Pads[g.selected].Empty():
		setText(g.info, sp404sx.PadName(g.selected)+" is empty: Import… or drop a file onto it.")
	default:
		t := tileFor(g.card, g.selected)
		setText(g.info, strings.TrimSpace(fmt.Sprintf("%s   %s   %s   %s",
			t.title, sp404sx.SampleFileName(g.selected), t.detail, t.flags)))
	}
}

// refreshTiles updates the pads of the current bank; unchanged pads are
// not redrawn.
func (g *gui) refreshTiles() {
	for i, t := range g.tiles {
		index := g.bank*sp404sx.PadsPerBank + i
		t.set(tileFor(g.card, index), index == g.selected, index == g.playing, index == g.dropPad)
	}
}

// dragOver highlights the pad under a file being dragged over the window
// (pos is an absolute canvas position), or none outside the pads.
func (g *gui) dragOver(pos fyne.Position) {
	pad, ok := g.padAt(pos)
	if !ok {
		pad = -1
	}
	if pad != g.dropPad {
		g.dropPad = pad
		g.refreshTiles()
	}
}

// dragOverScreen is dragOver for a position in GLFW screen coordinates,
// as the native drag hook reports it; Fyne maps the mouse the same way.
func (g *gui) dragOverScreen(x, y float64) {
	s := g.win.Canvas().Scale()
	g.dragOver(fyne.NewPos(float32(x)/s, float32(y)/s))
}

// dragGone clears the drop highlight (drag left the window, ended or dropped).
func (g *gui) dragGone() {
	if g.dropPad != -1 {
		g.dropPad = -1
		g.refreshTiles()
	}
}

// padAt returns the pad of the tile under pos (absolute canvas position)
// in the current bank. ok is false with no card or outside the tiles.
func (g *gui) padAt(pos fyne.Position) (pad int, ok bool) {
	if g.card == nil {
		return 0, false
	}
	driver := fyne.CurrentApp().Driver()
	for i, t := range g.tiles {
		p := driver.AbsolutePositionForObject(t)
		s := t.Size()
		if pos.X >= p.X && pos.X < p.X+s.Width && pos.Y >= p.Y && pos.Y < p.Y+s.Height {
			return g.bank*sp404sx.PadsPerBank + i, true
		}
	}
	return 0, false
}

// drop imports a single dropped file into the pad under the cursor.
func (g *gui) drop(pos fyne.Position, uris []fyne.URI) {
	g.dragGone()
	if g.card == nil {
		g.showError(errors.New("open a card first"))
		return
	}
	var paths []string
	for _, u := range uris {
		if u.Scheme() == "file" {
			paths = append(paths, u.Path())
		}
	}
	if len(paths) != 1 {
		g.showError(errors.New("drop one file at a time"))
		return
	}
	pad, ok := g.padAt(pos)
	if !ok {
		g.showError(errors.New("drop the file onto a pad"))
		return
	}
	g.importInto(pad, paths[0])
}

// importInto imports path into pad, asking before replacing a sample.
func (g *gui) importInto(pad int, path string) {
	run := func() {
		g.stopPlayback()
		g.runTask("Importing…", func(report progressFunc) error {
			detail := sp404sx.PadName(pad) + " ← " + filepath.Base(path)
			report(0, detail)
			return g.card.ImportFileProgress(pad, path, func(done, total int64) {
				report(float64(done)/float64(max(total, 1)), detail)
			})
		}, func(err error) {
			g.invalidatePanel()
			g.invalidateSamples()
			g.selected = pad
			g.refresh()
			g.showError(err)
		})
	}
	if g.card.Info.Pads[pad].Empty() {
		run()
		return
	}
	g.confirm("Replace sample?",
		fmt.Sprintf("Pad %s already has a sample that will be replaced.", sp404sx.PadName(pad)),
		"Replace", run)
}

func (g *gui) prefs() fyne.Preferences { return fyne.CurrentApp().Preferences() }

// importDialog imports one file into the selected pad.
// Card and pad are fixed when the dialog opens: the native dialog does not
// block the window, so the selection may change meanwhile.
func (g *gui) importDialog() {
	if g.card == nil || g.selected < 0 {
		return
	}
	card, pad := g.card, g.selected
	g.picker.openAudio(g.prefs().String(prefImportDir), func(path string, err error) {
		if err != nil || path == "" || g.card != card {
			g.showError(err)
			return
		}
		g.prefs().SetString(prefImportDir, filepath.Dir(path))
		g.importInto(pad, path)
	})
}

func (g *gui) exportDialog() {
	if g.card == nil || g.selected < 0 {
		return
	}
	card, pad, trim := g.card, g.selected, g.trim.Checked
	g.picker.saveWAV(g.prefs().String(prefExportDir), sp404sx.PadName(pad)+".wav", func(path string, err error) {
		if err != nil || path == "" {
			g.showError(err)
			return
		}
		g.prefs().SetString(prefExportDir, filepath.Dir(path))
		g.runTask("Exporting…", func(progressFunc) error { return card.ExportFile(pad, path, trim) }, g.showError)
	})
}

func (g *gui) clearSelected() {
	if g.selected < 0 || g.card.Info.Pads[g.selected].Empty() {
		return
	}
	pad := g.selected
	g.confirm("Clear pad?", fmt.Sprintf("Delete the sample on pad %s from the card?", sp404sx.PadName(pad)), "Clear", func() {
		g.stopPlayback()
		g.invalidatePanel()
		g.invalidateSamples()
		g.showError(g.card.Clear(pad))
		g.refresh()
	})
}

// togglePlay stops the current preview, or plays the selected pad.
func (g *gui) togglePlay() {
	if g.playing >= 0 {
		g.stopPlayback()
		g.refresh()
		return
	}
	g.playPad(g.selected)
}

// playPad selects a pad and previews it as the pad would sound.
func (g *gui) playPad(index int) {
	if g.card == nil || index < 0 || g.card.Info.Pads[index].Empty() {
		g.selectPad(index)
		return
	}
	g.stopPlayback()
	g.selected, g.playing = index, index // the pad lights up at once
	g.playGen++
	gen, card := g.playGen, g.card
	start := func(s *sp404sx.Sample, err error) {
		if gen != g.playGen {
			return // stopped or replaced while the sample was loading
		}
		if err == nil {
			err = g.player.play(previewPCM(s, card.Info.Pads[index]), func() {
				fyne.Do(func() {
					if gen == g.playGen {
						g.playing = -1
						g.refresh()
					}
				})
			})
		}
		if err != nil {
			g.playing = -1
			g.refresh()
			g.showError(err)
		}
	}
	g.fetchSample(card, index, start)
	g.refresh()
}

func (g *gui) stopPlayback() {
	if g.playing >= 0 {
		g.player.stop()
		g.playing = -1
		g.playGen++
	}
}

func (g *gui) openFolderDialog() {
	g.picker.openFolder(g.prefs().String(prefCardDir), func(path string, err error) {
		if err != nil || path == "" {
			g.showError(err)
			return
		}
		if err := g.openCard(path); err != nil {
			g.showError(err)
			return
		}
		g.prefs().SetString(prefCardDir, filepath.Dir(path))
	})
}

// runInBackground runs task off the UI thread behind a modal progress
// dialog, then calls done on the UI thread. The bar spins until the task
// first reports progress, then shows the fraction done.
func (g *gui) runInBackground(label string, task func(progressFunc) error, done func(error)) {
	spinner := widget.NewProgressBarInfinite()
	bar := widget.NewProgressBar()
	bar.Hide()
	detail := widget.NewLabel("") // always laid out, so the dialog keeps its height
	detail.Truncation = fyne.TextTruncateEllipsis
	body := container.NewVBox(detail, container.NewStack(spinner, bar))
	d := dialog.NewCustomWithoutButtons(label, body, g.win)
	d.Resize(fyne.NewSize(420, d.MinSize().Height))
	d.Show()
	report := func(fraction float64, text string) {
		fyne.Do(func() {
			if spinner.Visible() {
				spinner.Stop()
				spinner.Hide()
				bar.Show()
			}
			bar.SetValue(fraction)
			if text != detail.Text {
				detail.SetText(text)
			}
		})
	}
	go func() {
		err := task(report)
		fyne.Do(func() {
			spinner.Stop()
			d.Hide()
			done(err)
		})
	}()
}

func (g *gui) showError(err error) {
	if err != nil {
		dialog.ShowError(err, g.win)
	}
}
