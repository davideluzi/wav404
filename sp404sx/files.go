package sp404sx

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PadName is the label of a pad index, e.g. 0 → "A1", 119 → "J12".
func PadName(index int) string {
	return fmt.Sprintf("%c%d", 'A'+index/PadsPerBank, index%PadsPerBank+1)
}

// ParsePad accepts "A1" … "J12", case-insensitive.
func ParsePad(s string) (int, error) {
	if len(s) >= 2 {
		bank := strings.ToUpper(s[:1])[0]
		n, err := strconv.Atoi(s[1:])
		if bank >= 'A' && bank <= 'J' && err == nil && n >= 1 && n <= PadsPerBank {
			return PadIndex(bank, n), nil
		}
	}
	return 0, fmt.Errorf("invalid pad %q (expected A1–J12)", s)
}

// ImportFile converts a WAV/AIFF file and assigns it to a pad.
// On error the card is left unchanged.
func (c *Card) ImportFile(index int, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s, err := ImportAudio(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return c.Assign(index, s)
}

// ExportFile writes a pad's sample to path as a standard WAV, optionally
// limited to the pad's user start/end range.
func (c *Card) ExportFile(index int, path string, trim bool) error {
	if err := checkIndex(index); err != nil {
		return err
	}
	p := c.Info.Pads[index]
	if p.Empty() {
		return fmt.Errorf("pad %s is empty", PadName(index))
	}
	s, err := c.ReadSample(index)
	if err != nil {
		return err
	}
	if trim {
		if s, err = s.Trim(p); err != nil {
			return err
		}
	}
	return os.WriteFile(path, s.EncodeWAV(), 0o644)
}
