package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/davideluzi/wav404/sp404sx"
)

// tile is what a pad shows on screen.
type tile struct {
	title  string // "A1"
	detail string // "0.67 s · 89.0 BPM" or "empty"
	flags  string // "loop gate"
	empty  bool
}

// tileFor describes pad index of card c (c may be nil: no card open).
func tileFor(c *sp404sx.Card, index int) tile {
	t := tile{title: sp404sx.PadName(index), detail: "empty", empty: true}
	if c == nil || c.Info.Pads[index].Empty() {
		return t
	}
	p := c.Info.Pads[index]
	t.empty = false
	t.detail = fmt.Sprintf("%.2f s · %.1f BPM", float64(p.Frames())/sp404sx.SampleRate, float64(p.Tempo)/10)
	var flags []string
	for _, f := range []struct {
		on   bool
		name string
	}{{p.Channels == 1, "mono"}, {p.Loop, "loop"}, {p.Gate, "gate"}, {p.Reverse, "reverse"}, {p.Lofi, "lofi"}} {
		if f.on {
			flags = append(flags, f.name)
		}
	}
	t.flags = strings.Join(flags, " ")
	return t
}

// findCards returns the candidates that contain an SP-404SX card.
func findCards(candidates []string) []string {
	var cards []string
	for _, root := range candidates {
		if _, err := os.Stat(filepath.Join(root, "ROLAND", "SP-404SX", "SMPL", "PAD_INFO.BIN")); err == nil {
			cards = append(cards, root)
		}
	}
	return cards
}

// mountPoints lists where removable volumes usually appear on this OS.
func mountPoints() []string {
	switch runtime.GOOS {
	case "darwin":
		m, _ := filepath.Glob("/Volumes/*")
		return m
	case "windows":
		var m []string
		for d := 'D'; d <= 'Z'; d++ {
			m = append(m, string(d)+`:\`)
		}
		return m
	default:
		a, _ := filepath.Glob("/media/*/*")
		b, _ := filepath.Glob("/run/media/*/*")
		return append(a, b...)
	}
}
