package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/davideluzi/wav404/sp404sx"
)

// padPanel edits the settings of the selected pad. Every change is written
// to the card at once (the volume slider when it is released).
type padPanel struct {
	volume                    *widget.Slider
	volumeLabel               *widget.Label
	loop, gate, reverse, lofi *widget.Check
	bpm                       *widget.Entry
	origBPM                   *widget.Label
	resetBPM                  *widget.Button
	wave                      *waveView
	trimLabel                 *widget.Label
	resetTrim                 *widget.Button
	waveIndex                 int // pad whose sample the wave holds, -1 = none

	loading  bool        // true while the widgets are being filled in
	shownPad int         // pad the widgets show: -1 none, panelStale = reload
	shown    sp404sx.Pad // its record when it was loaded
}

const panelStale = -2

func (g *gui) buildPanel() {
	g.shownPad, g.waveIndex = panelStale, -1
	g.wave = newWaveView(g.editTrim)
	g.trimLabel = widget.NewLabel("")
	g.resetTrim = widget.NewButtonWithIcon("Whole sample", theme.ContentUndoIcon(), func() {
		g.editTrim(0, g.wave.frames())
	})
	g.volume = widget.NewSlider(0, 127)
	g.volume.Step = 1
	g.volumeLabel = widget.NewLabel("")
	g.volume.OnChanged = func(v float64) { g.volumeLabel.SetText(strconv.Itoa(int(v))) }
	g.volume.OnChangeEnded = func(v float64) {
		g.editPad(func(p *sp404sx.Pad) { p.Volume = uint8(v) })
	}
	check := func(label string, set func(*sp404sx.Pad, bool)) *widget.Check {
		return widget.NewCheck(label, func(on bool) { g.editPad(func(p *sp404sx.Pad) { set(p, on) }) })
	}
	g.loop = check("Loop", func(p *sp404sx.Pad, on bool) { p.Loop = on })
	g.gate = check("Gate", func(p *sp404sx.Pad, on bool) { p.Gate = on })
	g.reverse = check("Reverse", func(p *sp404sx.Pad, on bool) { p.Reverse = on })
	g.lofi = check("Lo-fi", func(p *sp404sx.Pad, on bool) { p.Lofi = on })

	g.bpm = widget.NewEntry()
	g.bpm.OnSubmitted = func(text string) {
		tempo, err := parseBPM(text)
		if err != nil {
			g.shownPad = panelStale // restore the saved value
			g.refresh()
			g.showError(err)
			return
		}
		g.editPad(func(p *sp404sx.Pad) { p.UserTempo = tempo })
	}
	g.origBPM = widget.NewLabel("")
	g.resetBPM = widget.NewButtonWithIcon("", theme.HistoryIcon(), func() {
		g.editPad(func(p *sp404sx.Pad) { p.UserTempo = p.OriginalTempo })
	})
}

func (g *gui) panelContent() fyne.CanvasObject {
	bpmBox := container.NewGridWrap(fyne.NewSize(80, g.bpm.MinSize().Height), g.bpm)
	return container.NewVBox(
		g.wave,
		container.NewBorder(nil, nil, nil, g.resetTrim, g.trimLabel),
		container.NewBorder(nil, nil, widget.NewLabel("Volume"), g.volumeLabel, g.volume),
		container.NewHBox(g.loop, g.gate, g.reverse, g.lofi,
			widget.NewSeparator(),
			widget.NewLabel("BPM"), bpmBox, g.resetBPM, g.origBPM),
	)
}

// parseBPM reads "130" or "130.5" as BPM × 10.
func parseBPM(text string) (uint32, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.Replace(text, ",", ".", 1)), 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a tempo", text)
	}
	if v < 40 || v > 200 {
		return 0, errors.New("tempo must be between 40.0 and 200.0 BPM")
	}
	return uint32(math.Round(v * 10)), nil
}

// loadPanel shows the selected pad, unless it is already shown unchanged
// (so a refresh does not wipe a tempo being typed).
func (g *gui) loadPanel() {
	index := -1
	var p sp404sx.Pad
	if g.card != nil && g.selected >= 0 && !g.card.Info.Pads[g.selected].Empty() {
		index, p = g.selected, g.card.Info.Pads[g.selected]
	}
	if index == g.shownPad && p == g.shown {
		return
	}
	g.shownPad, g.shown = index, p

	g.loading = true
	defer func() { g.loading = false }()
	g.loadWave(index, p)
	for _, w := range []fyne.Disableable{g.volume, g.loop, g.gate, g.reverse, g.lofi, g.bpm, g.resetBPM} {
		if index < 0 {
			w.Disable()
		} else {
			w.Enable()
		}
	}
	if index < 0 {
		p = sp404sx.EmptyPad()
		p.Gate = false
	}
	g.volume.SetValue(float64(p.Volume))
	g.volumeLabel.SetText(strconv.Itoa(int(p.Volume)))
	g.loop.SetChecked(p.Loop)
	g.gate.SetChecked(p.Gate)
	g.reverse.SetChecked(p.Reverse)
	g.lofi.SetChecked(p.Lofi)
	if index < 0 {
		g.bpm.SetText("")
		g.origBPM.SetText("")
		return
	}
	g.bpm.SetText(fmt.Sprintf("%.1f", float64(p.UserTempo)/10))
	g.origBPM.SetText(fmt.Sprintf("original %.1f", float64(p.OriginalTempo)/10))
}

// loadWave shows pad index (-1: none) with its trim. The sample file is
// read again only when the pad changes or after invalidatePanel.
func (g *gui) loadWave(index int, p sp404sx.Pad) {
	if index != g.waveIndex {
		g.waveIndex = index
		g.wave.sample = nil
		if index >= 0 {
			if s, err := g.card.ReadSample(index); err == nil {
				g.wave.sample = s
			}
		}
	}
	s := g.wave.sample
	switch {
	case index < 0:
		g.wave.setSample(nil, 0, 0)
		g.trimLabel.SetText("")
		g.resetTrim.Disable()
		return
	case s == nil:
		g.wave.setSample(nil, 0, 0)
		g.trimLabel.SetText("Sample file not found or unreadable on the card.")
		g.resetTrim.Disable()
		return
	}
	start, end := p.TrimFrames()
	g.wave.setSample(s, start, end)
	sec := func(f int) float64 { return float64(f) / sp404sx.SampleRate }
	g.trimLabel.SetText(fmt.Sprintf("Start %.3f s   End %.3f s   Length %.3f s",
		sec(start), sec(end), sec(end-start)))
	g.resetTrim.Enable()
}

// invalidatePanel forces the panel, sample included, to reload: needed
// after the sample file changes (import, clear, another card).
func (g *gui) invalidatePanel() {
	g.shownPad, g.waveIndex = panelStale, -1
}

// editTrim saves a new trim range (frames) for the selected pad.
func (g *gui) editTrim(start, end int) {
	var err error
	g.editPad(func(p *sp404sx.Pad) { err = p.SetTrim(start, end) })
	g.showError(err)
}

// editPad applies change to the selected pad and saves it.
func (g *gui) editPad(change func(*sp404sx.Pad)) {
	if g.loading || g.card == nil || g.selected < 0 {
		return
	}
	p := g.card.Info.Pads[g.selected]
	change(&p)
	if p == g.card.Info.Pads[g.selected] {
		g.shownPad = panelStale // e.g. "89" typed for 89.0: show it normalized
		g.refresh()
		return
	}
	if err := g.card.SetPad(g.selected, p); err != nil {
		g.shownPad = panelStale // show the saved values again
		g.showError(err)
	}
	g.refresh()
}
