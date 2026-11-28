// Command sp404 manages the samples on an SP-404SX SD card.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/davideluzi/wav404/sp404sx"
)

const usage = `usage:
  sp404 list   <card>                         list assigned pads
  sp404 import <card> <pad> <file.wav|aif>    import a file into a pad (e.g. A1, J12)
  sp404 export <card> <pad> <out.wav> [-trim] export a pad as a plain WAV
  sp404 clear  <card> <pad>                   delete the sample on a pad

<card> is the card root, e.g. /Volumes/SP-404SX`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sp404:", err)
		os.Exit(1)
	}
}

var errUsage = errors.New(usage)

func run(args []string, out io.Writer) error {
	trim := false
	var pos []string
	for _, a := range args {
		if a == "-trim" || a == "--trim" {
			trim = true
		} else {
			pos = append(pos, a)
		}
	}
	if len(pos) == 0 {
		return errUsage
	}
	want := map[string]int{"list": 2, "import": 4, "export": 4, "clear": 3}
	n, ok := want[pos[0]]
	if !ok || len(pos) != n {
		return errUsage
	}

	card, err := sp404sx.OpenCard(pos[1])
	if err != nil {
		return err
	}
	if pos[0] == "list" {
		return list(card, out)
	}
	pad, err := sp404sx.ParsePad(pos[2])
	if err != nil {
		return err
	}
	switch pos[0] {
	case "import":
		if err := card.ImportFile(pad, pos[3]); err != nil {
			return err
		}
		p := card.Info.Pads[pad]
		fmt.Fprintf(out, "%s ← %s (%.2fs, %.1f BPM)\n", sp404sx.PadName(pad), pos[3],
			float64(p.Frames())/sp404sx.SampleRate, float64(p.Tempo)/10)
	case "export":
		if err := card.ExportFile(pad, pos[3], trim); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s → %s\n", sp404sx.PadName(pad), pos[3])
	default: // clear
		if err := card.Clear(pad); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s cleared\n", sp404sx.PadName(pad))
	}
	return nil
}

func list(c *sp404sx.Card, out io.Writer) error {
	for i, p := range c.Info.Pads {
		if p.Empty() {
			continue
		}
		ch := "stereo"
		if p.Channels == 1 {
			ch = "mono"
		}
		var flags []string
		for _, f := range []struct {
			on   bool
			name string
		}{{p.Loop, "loop"}, {p.Gate, "gate"}, {p.Reverse, "reverse"}, {p.Lofi, "lofi"}} {
			if f.on {
				flags = append(flags, f.name)
			}
		}
		fmt.Fprintf(out, "%-3s %s  %-6s %7.2fs  %5.1f BPM  vol %3d  %s\n",
			sp404sx.PadName(i), sp404sx.SampleFileName(i), ch,
			float64(p.Frames())/sp404sx.SampleRate, float64(p.Tempo)/10, p.Volume,
			strings.Join(flags, " "))
	}
	return nil
}
