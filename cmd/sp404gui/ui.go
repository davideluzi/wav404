package main

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/davideluzi/wav404/sp404sx"
)

const noCardText = "No card open: insert an SP-404SX card or use Open folder…"

type gui struct {
	win      fyne.Window
	card     *sp404sx.Card
	bank     int
	selected int // pad index, -1 = none
	playing  int // pad index being previewed, -1 = none
	playGen  int // identifies the current playback; stale end callbacks are ignored
	player   audioPlayer

	tiles                          [sp404sx.PadsPerBank]*padTile
	banks                          *widget.RadioGroup
	cards                          *widget.Select
	info                           *widget.Label
	trim                           *widget.Check
	importBtn, exportBtn, clearBtn *widget.Button
	playBtn                        *widget.Button
	padPanel

	// replaceable in tests
	confirm func(title, message string, ok func())
	runTask func(label string, task func() error, done func(error))
}

func newGUI(w fyne.Window) *gui {
	g := &gui{win: w, selected: -1, playing: -1, player: &otoPlayer{}}
	g.confirm = func(title, message string, ok func()) {
		dialog.ShowConfirm(title, message, func(yes bool) {
			if yes {
				ok()
			}
		}, w)
	}
	g.runTask = g.runInBackground

	for i := range g.tiles {
		index := func() int { return g.bank*sp404sx.PadsPerBank + i }
		g.tiles[i] = newPadTile(func() { g.selectPad(index()) }, func() { g.playPad(index()) })
	}
	var letters []string
	for b := range sp404sx.NumBanks {
		letters = append(letters, string(rune('A'+b)))
	}
	g.banks = widget.NewRadioGroup(letters, func(s string) {
		if s == "" {
			g.banks.SetSelected(letters[g.bank]) // a bank is always selected
			return
		}
		g.stopPlayback()
		g.bank = int(s[0] - 'A')
		g.selected = -1
		g.refresh()
	})
	g.banks.Horizontal = true

	g.cards = widget.NewSelect(nil, func(root string) {
		if g.card == nil || g.card.Root != root {
			g.showError(g.openCard(root))
		}
	})
	g.cards.PlaceHolder = "No SP-404SX card found"
	g.info = widget.NewLabel(noCardText)
	g.info.Wrapping = fyne.TextWrapWord
	g.trim = widget.NewCheck("Only the trimmed part", nil)
	g.importBtn = widget.NewButtonWithIcon("Import…", theme.DownloadIcon(), g.importDialog)
	g.exportBtn = widget.NewButtonWithIcon("Export…", theme.UploadIcon(), g.exportDialog)
	g.clearBtn = widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), g.clearSelected)
	g.playBtn = widget.NewButtonWithIcon("Play", theme.MediaPlayIcon(), g.togglePlay)
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

func (g *gui) content() fyne.CanvasObject {
	openFolder := widget.NewButtonWithIcon("Open folder…", theme.FolderOpenIcon(), g.openFolderDialog)
	rescan := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), g.scanCards)
	top := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel("Card"), container.NewHBox(rescan, openFolder), g.cards),
		container.NewHBox(widget.NewLabel("Bank"), g.banks),
	)
	grid := container.NewGridWithColumns(4)
	for _, t := range g.tiles {
		grid.Add(t)
	}
	actions := container.NewHBox(g.playBtn, g.importBtn, g.exportBtn, g.trim, layout.NewSpacer(), g.clearBtn)
	bottom := container.NewVBox(g.info, g.panelContent(), widget.NewSeparator(), actions)
	return container.NewBorder(top, bottom, nil, nil, container.NewPadded(grid))
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
	for i, t := range g.tiles {
		index := g.bank*sp404sx.PadsPerBank + i
		t.set(tileFor(g.card, index), index == g.selected, index == g.playing)
	}
	enable := func(b *widget.Button, on bool) {
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
	if g.playing >= 0 {
		g.playBtn.SetText("Stop")
		g.playBtn.SetIcon(theme.MediaStopIcon())
	} else {
		g.playBtn.SetText("Play")
		g.playBtn.SetIcon(theme.MediaPlayIcon())
	}

	switch {
	case g.card == nil:
		g.info.SetText(noCardText)
	case !hasPad:
		g.info.SetText("Select a pad, or drop WAV/AIFF files onto a pad.")
	case g.card.Info.Pads[g.selected].Empty():
		g.info.SetText(sp404sx.PadName(g.selected) + " is empty: Import… or drop a file onto it.")
	default:
		t := tileFor(g.card, g.selected)
		g.info.SetText(strings.TrimSpace(fmt.Sprintf("%s   %s   %s   %s",
			t.title, sp404sx.SampleFileName(g.selected), t.detail, t.flags)))
	}
}

// drop imports the dropped files into the pad under the cursor (or the
// selected pad) and the pads after it.
func (g *gui) drop(pos fyne.Position, uris []fyne.URI) {
	if g.card == nil {
		g.showError(errors.New("open a card first"))
		return
	}
	first := g.selected
	driver := fyne.CurrentApp().Driver()
	for i, t := range g.tiles {
		p := driver.AbsolutePositionForObject(t)
		s := t.Size()
		if pos.X >= p.X && pos.X < p.X+s.Width && pos.Y >= p.Y && pos.Y < p.Y+s.Height {
			first = g.bank*sp404sx.PadsPerBank + i
		}
	}
	if first < 0 {
		g.showError(errors.New("drop the files onto a pad"))
		return
	}
	var paths []string
	for _, u := range uris {
		if u.Scheme() == "file" {
			paths = append(paths, u.Path())
		}
	}
	g.importInto(first, paths)
}

// importInto imports paths into consecutive pads starting at first,
// asking before overwriting assigned pads.
func (g *gui) importInto(first int, paths []string) {
	if len(paths) == 0 {
		return
	}
	pads, overwrite := dropTargets(g.card, first, len(paths))
	run := func() {
		g.stopPlayback()
		g.runTask("Importing…", func() error {
			for i, pad := range pads {
				if err := g.card.ImportFile(pad, paths[i]); err != nil {
					return err
				}
			}
			return nil
		}, func(err error) {
			g.invalidatePanel()
			g.selected = pads[len(pads)-1]
			g.refresh()
			g.showError(err)
		})
	}
	if len(overwrite) == 0 {
		run()
		return
	}
	g.confirm("Replace samples?",
		fmt.Sprintf("These pads already have a sample that will be replaced:\n%s", strings.Join(overwrite, ", ")), run)
}

func (g *gui) importDialog() {
	if g.selected < 0 {
		return
	}
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			g.showError(err)
			return
		}
		r.Close()
		g.importInto(g.selected, []string{r.URI().Path()})
	}, g.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".wav", ".WAV", ".aif", ".AIF", ".aiff", ".AIFF"}))
	d.Show()
}

func (g *gui) exportDialog() {
	if g.selected < 0 {
		return
	}
	pad, trim := g.selected, g.trim.Checked
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			g.showError(err)
			return
		}
		w.Close()
		path := w.URI().Path()
		g.runTask("Exporting…", func() error { return g.card.ExportFile(pad, path, trim) }, g.showError)
	}, g.win)
	d.SetFileName(sp404sx.PadName(pad) + ".wav")
	d.Show()
}

func (g *gui) clearSelected() {
	if g.selected < 0 || g.card.Info.Pads[g.selected].Empty() {
		return
	}
	pad := g.selected
	g.confirm("Clear pad?", fmt.Sprintf("Delete the sample on pad %s from the card?", sp404sx.PadName(pad)), func() {
		g.stopPlayback()
		g.invalidatePanel()
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
	g.selected = index
	s, err := g.card.ReadSample(index)
	if err == nil {
		g.playGen++
		gen := g.playGen
		err = g.player.play(previewPCM(s, g.card.Info.Pads[index]), func() {
			fyne.Do(func() {
				if gen == g.playGen {
					g.playing = -1
					g.refresh()
				}
			})
		})
	}
	if err == nil {
		g.playing = index
	}
	g.refresh()
	g.showError(err)
}

func (g *gui) stopPlayback() {
	if g.playing >= 0 {
		g.player.stop()
		g.playing = -1
		g.playGen++
	}
}

func (g *gui) openFolderDialog() {
	dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			g.showError(err)
			return
		}
		g.showError(g.openCard(u.Path()))
	}, g.win)
}

// runInBackground runs task off the UI thread behind a modal progress
// dialog, then calls done on the UI thread.
func (g *gui) runInBackground(label string, task func() error, done func(error)) {
	bar := widget.NewProgressBarInfinite()
	d := dialog.NewCustomWithoutButtons(label, bar, g.win)
	d.Show()
	go func() {
		err := task()
		fyne.Do(func() {
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
