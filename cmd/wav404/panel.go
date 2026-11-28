package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/davideluzi/wav404/sp404sx"
)

// padPanel edits the settings of the selected pad. Every change is written
// to the card at once (the volume knob when it is released).
type padPanel struct {
	volume                    *hwKnob
	volumeLabel               *canvas.Text
	loop, gate, reverse, lofi *hwToggle
	channels                  *hwKey // STEREO / MONO indicator
	bpmDisplay                *hwDisplay
	bpm                       *widget.Entry // tempo editor, shown when the display is tapped
	bpmPopup                  *widget.PopUp
	autoBPM                   *canvas.Text // tempo the device would compute for the trim
	resetBPM                  *hwKey
	wave                      *waveView
	trimLabel                 *canvas.Text
	resetTrim                 *hwKey
	waveIndex                 int // pad whose sample the wave holds, -1 = none
	waveLoading               bool

	loading  bool        // true while the widgets are being filled in
	shownPad int         // pad the widgets show: -1 none, panelStale = reload
	shown    sp404sx.Pad // its record when it was loaded
}

const panelStale = -2

func setText(t *canvas.Text, s string) {
	if t.Text != s {
		t.Text = s
		t.Refresh()
	}
}

func (g *gui) buildPanel() {
	g.shownPad, g.waveIndex = panelStale, -1
	g.wave = newWaveView(g.editTrim)
	g.trimLabel = silk("", 11, hwSilk)
	g.resetTrim = newHWKey("WHOLE", 11, 6)
	g.resetTrim.onTap = func() { g.editTrim(0, g.wave.frames()) }

	g.volume = newHWKnob(0, 127)
	g.volumeLabel = silk("", 14, hwSilk)
	g.volume.OnChanged = func(v float64) { setText(g.volumeLabel, strconv.Itoa(int(v))) }
	g.volume.OnChangeEnded = func(v float64) {
		g.editPad(func(p *sp404sx.Pad) { p.Volume = uint8(v) })
	}
	toggle := func(label string, set func(*sp404sx.Pad, bool)) *hwToggle {
		return newHWToggle(label, func(on bool) { g.editPad(func(p *sp404sx.Pad) { set(p, on) }) })
	}
	g.lofi = toggle("LO-FI", func(p *sp404sx.Pad, on bool) { p.Lofi = on })
	g.gate = toggle("GATE", func(p *sp404sx.Pad, on bool) { p.Gate = on })
	g.loop = toggle("LOOP", func(p *sp404sx.Pad, on bool) { p.Loop = on })
	g.reverse = toggle("REVERSE", func(p *sp404sx.Pad, on bool) { p.Reverse = on })
	g.channels = newHWKey("STEREO", 12, 7)

	g.bpmDisplay = newHWDisplay(4)
	g.bpmDisplay.onTap = g.editBPM
	g.bpm = widget.NewEntry()
	g.bpm.OnSubmitted = func(text string) {
		if g.bpmPopup != nil {
			g.bpmPopup.Hide()
		}
		tempo, err := parseBPM(text)
		if err != nil {
			g.shownPad = panelStale // restore the saved value
			g.refresh()
			g.showError(err)
			return
		}
		g.editPad(func(p *sp404sx.Pad) { p.SetTempo(tempo) })
	}
	g.autoBPM = silk("", 11, hwSilkDim)
	g.resetBPM = newHWKey("AUTO", 10, 5)
	g.resetBPM.onTap = func() {
		g.editPad(func(p *sp404sx.Pad) { p.SetTempo(p.AutoTempo()) })
	}
}

// editBPM opens a small tempo field over the display; Enter saves.
func (g *gui) editBPM() {
	if g.bpm.Disabled() {
		return
	}
	if g.bpmPopup == nil {
		box := container.NewBorder(nil, nil, widget.NewLabel("BPM"), nil, g.bpm)
		g.bpmPopup = widget.NewPopUp(container.NewGridWrap(fyne.NewSize(200, box.MinSize().Height), box), g.win.Canvas())
	}
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(g.bpmDisplay)
	g.bpmPopup.ShowAtPosition(pos.AddXY(10, g.bpmDisplay.Size().Height-6))
	g.win.Canvas().Focus(g.bpm)
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
	setText(g.volumeLabel, strconv.Itoa(int(p.Volume)))
	g.loop.SetChecked(p.Loop)
	g.gate.SetChecked(p.Gate)
	g.reverse.SetChecked(p.Reverse)
	g.lofi.SetChecked(p.Lofi)
	g.channels.label = map[bool]string{true: "STEREO", false: "MONO"}[p.Channels == 2]
	g.channels.lit = index >= 0
	g.channels.Refresh()
	if index < 0 {
		g.bpm.SetText("")
		g.bpmDisplay.setText("")
		setText(g.volumeLabel, "")
		setText(g.autoBPM, "")
		return
	}
	tempo := fmt.Sprintf("%.1f", float64(p.Tempo)/10)
	g.bpm.SetText(tempo)
	g.bpmDisplay.setText(tempo)
	setText(g.autoBPM, fmt.Sprintf("AUTO %.1f", float64(p.AutoTempo())/10))
}

// loadWave shows pad index (-1: none) with its trim. The sample file is
// read again only when the pad changes or after invalidatePanel.
func (g *gui) loadWave(index int, p sp404sx.Pad) {
	if index != g.waveIndex {
		g.waveIndex, g.wave.sample, g.waveLoading = index, nil, false
		if index >= 0 {
			if s, ok := g.cachedSample(index); ok {
				g.wave.sample = s
			} else {
				g.waveLoading = true
				g.fetchSample(g.card, index, func(s *sp404sx.Sample, err error) {
					if g.waveIndex != index {
						return // another pad was selected meanwhile
					}
					g.wave.sample, g.waveLoading = s, false
					g.shownPad = panelStale
					g.refresh()
				})
			}
		}
	}
	s := g.wave.sample
	switch {
	case index < 0:
		g.wave.setSample(nil, 0, 0)
		setText(g.trimLabel, "")
		g.resetTrim.Disable()
		return
	case g.waveLoading:
		g.wave.setSample(nil, 0, 0)
		setText(g.trimLabel, "LOADING SAMPLE…")
		g.resetTrim.Disable()
		return
	case s == nil:
		g.wave.setSample(nil, 0, 0)
		setText(g.trimLabel, "SAMPLE FILE NOT FOUND ON THE CARD")
		g.resetTrim.Disable()
		return
	}
	start, end := p.TrimFrames()
	g.wave.setSample(s, start, end)
	sec := func(f int) float64 { return float64(f) / sp404sx.SampleRate }
	setText(g.trimLabel, fmt.Sprintf("START %.3f   END %.3f   LENGTH %.3f", sec(start), sec(end), sec(end-start)))
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
