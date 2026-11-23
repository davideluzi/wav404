package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/davideluzi/wav404/sp404sx"
)

// Size of the hardware-style window in the mockup.
const mockW, mockH = 1040, 604

// hardwareMockup lays out the hardware-style panel for bank J of card c,
// with J5 selected and J7 playing. Static: nothing reacts yet.
func hardwareMockup(c *sp404sx.Card) fyne.CanvasObject {
	const bank, selected, playing = 9, 9*12 + 4, 9*12 + 6
	place := func(o fyne.CanvasObject, x, y, w, h float32) fyne.CanvasObject {
		o.Move(fyne.NewPos(x, y))
		o.Resize(fyne.NewSize(w, h))
		return o
	}
	text := func(t *canvas.Text, x, y float32) fyne.CanvasObject {
		t.Move(fyne.NewPos(x, y))
		return t
	}
	var objs []fyne.CanvasObject
	add := func(o ...fyne.CanvasObject) { objs = append(objs, o...) }

	// utility strip: the functions the hardware does not have
	strip := canvas.NewRectangle(colBackground)
	add(place(strip, 0, 0, mockW, 52))
	cards := widget.NewSelect([]string{c.Root}, nil)
	cards.SetSelected(c.Root)
	add(text(silk("CARD", 12, hwSilkDim), 20, 17),
		place(cards, 64, 8, 470, 36),
		place(widget.NewButton("Open folder…", nil), 542, 8, 130, 36),
		place(widget.NewButton("Import…", nil), 722, 8, 100, 36),
		place(widget.NewButton("Export…", nil), 828, 8, 100, 36),
		place(widget.NewButton("Clear", nil), 934, 8, 86, 36))

	// brushed panel with screws
	add(place(rasterOf(drawBrushedMetal), 0, 52, mockW, mockH-52))
	for _, p := range [][2]float32{{12, 64}, {mockW - 28, 64}, {12, mockH - 28}, {mockW - 28, mockH - 28}} {
		add(place(rasterOf(drawScrew), p[0], p[1], 16, 16))
	}

	pad := c.Info.Pads[selected]

	// SAMPLE MODE row
	add(text(silk("SAMPLE MODE", 11, hwSilkDim), 40, 72))
	for i, m := range []struct {
		name string
		on   bool
	}{{"LO-FI", pad.Lofi}, {"GATE", pad.Gate}, {"LOOP", pad.Loop}, {"REVERSE", pad.Reverse}} {
		k := newHWKey(m.name, 12, 7)
		k.lit = m.on
		add(place(k, 40+float32(i)*108, 90, 98, 42))
	}
	st := newHWKey("STEREO", 12, 7)
	st.lit = pad.Channels == 2
	add(text(silk("CHANNELS", 11, hwSilkDim), 514, 72), place(st, 514, 90, 98, 42))

	// BANK row
	add(text(silk("BANK", 11, hwSilkDim), 40, 148))
	for b := range sp404sx.NumBanks {
		k := newHWKey(string(rune('A'+b)), 14, 7)
		k.lit = b == bank
		add(place(k, 40+float32(b)*57.6, 166, 52, 42))
	}

	// pads
	for i := range sp404sx.PadsPerBank {
		index := bank*sp404sx.PadsPerBank + i
		t := tileFor(c, index)
		k := newHWKey(fmt.Sprint(i+1), 26, 10)
		k.full = !t.empty
		k.ring = index == selected
		k.lit = index == playing
		if t.empty {
			k.detail = "empty"
		} else {
			k.detail = fmt.Sprintf("%.2f s", float64(c.Info.Pads[index].Frames())/sp404sx.SampleRate)
		}
		col, row := i%4, i/4
		add(place(k, 34+float32(col)*146, 228+float32(row)*118, 140, 112))
	}

	// right column: display + knob
	add(text(silk("BPM", 11, hwSilkDim), 660, 72))
	disp := newHWDisplay(4)
	disp.text = fmt.Sprintf("%.1f", float64(pad.UserTempo)/10)
	add(place(disp, 660, 90, 220, 92))
	add(text(silk(fmt.Sprintf("ORIGINAL %.1f", float64(pad.OriginalTempo)/10), 11, hwSilkDim), 660, 188))
	knob := newHWKnob()
	knob.value = float64(pad.Volume) / 127
	add(text(silk("VOLUME", 11, hwSilkDim), 912, 72), place(knob, 898, 86, 104, 104))
	add(text(silk(fmt.Sprint(pad.Volume), 14, hwSilk), 938, 188))

	// waveform screen
	add(text(silk("TRIM", 11, hwSilkDim), 660, 228))
	add(place(rasterOf(drawScreen), 652, 246, 360, 150))
	wave := newWaveView(func(int, int) {})
	wave.pal = wavePalette{hwGlass, hwAmber, hwAmberDim, hwSilk}
	if s, err := c.ReadSample(selected); err == nil {
		st, en := pad.TrimFrames()
		wave.setSample(s, st, en)
	}
	add(place(wave, 660, 254, 344, 134))
	st0, en0 := pad.TrimFrames()
	sec := func(f int) float64 { return float64(f) / sp404sx.SampleRate }
	add(text(silk(fmt.Sprintf("START %.3f   END %.3f   LENGTH %.3f", sec(st0), sec(en0), sec(en0-st0)), 11, hwSilk), 660, 404))
	whole := newHWKey("WHOLE", 11, 6)
	add(place(whole, 930, 398, 82, 30))

	// transport
	play := newHWKey("PLAY", 18, 9)
	play.lit = true
	add(place(play, 652, 452, 170, 64))
	trimOnly := newHWKey("TRIM ONLY", 11, 6)
	add(text(silk("EXPORT", 11, hwSilkDim), 842, 452), place(trimOnly, 842, 470, 110, 40))

	// pad info line
	add(text(silk(fmt.Sprintf("%s   %s   %s", sp404sx.PadName(selected), sp404sx.SampleFileName(selected),
		map[bool]string{true: "STEREO", false: "MONO"}[pad.Channels == 2]), 12, hwSilkDim), 660, 548))

	return container.NewWithoutLayout(objs...)
}
