package sp404sx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Card is an SP-404SX SD card (or a copy of one) rooted at Root.
// Assign and Clear write to the card immediately.
type Card struct {
	Root string
	Info *PadInfo
}

func (c *Card) smplDir() string { return filepath.Join(c.Root, "ROLAND", "SP-404SX", "SMPL") }

func (c *Card) padInfoPath() string { return filepath.Join(c.smplDir(), "PAD_INFO.BIN") }

// SamplePath is the path of the WAV file for a pad index.
func (c *Card) SamplePath(index int) string { return filepath.Join(c.smplDir(), SampleFileName(index)) }

// OpenCard reads PAD_INFO.BIN from a card root. It never writes.
func OpenCard(root string) (*Card, error) {
	c := &Card{Root: root}
	raw, err := os.ReadFile(c.padInfoPath())
	if err != nil {
		return nil, fmt.Errorf("not an SP-404SX card: %w", err)
	}
	if c.Info, err = ParsePadInfo(raw); err != nil {
		return nil, err
	}
	return c, nil
}

func checkIndex(index int) error {
	if index < 0 || index >= NumPads {
		return fmt.Errorf("pad index %d out of range", index)
	}
	return nil
}

// ReadSample decodes the WAV file of a pad.
func (c *Card) ReadSample(index int) (*Sample, error) {
	if err := checkIndex(index); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(c.SamplePath(index))
	if err != nil {
		return nil, err
	}
	return DecodeSample(raw)
}

// Assign writes s as the sample of pad index, then records it in
// PAD_INFO.BIN with the converter's import defaults.
func (c *Card) Assign(index int, s *Sample) error {
	if err := checkIndex(index); err != nil {
		return err
	}
	cp := *s
	cp.PadIndex = index
	if err := writeFileAtomic(c.SamplePath(index), cp.Encode()); err != nil {
		return err
	}
	return c.setPad(index, cp.NewPad())
}

// Clear marks the pad empty in PAD_INFO.BIN, then deletes its sample file.
// In this order an interruption can leave an orphan file, but never a pad
// that points to a missing one.
func (c *Card) Clear(index int) error {
	if err := checkIndex(index); err != nil {
		return err
	}
	if err := c.setPad(index, EmptyPad()); err != nil {
		return err
	}
	if err := os.Remove(c.SamplePath(index)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SetPad changes the playback settings of an assigned pad (volume, flags,
// user tempo, trim range) and writes PAD_INFO.BIN. Fields that describe the
// sample file (offsets, channels, format, original tempo) cannot change.
// Values are checked against the converter's limits.
func (c *Card) SetPad(index int, p Pad) error {
	if err := checkIndex(index); err != nil {
		return err
	}
	cur := c.Info.Pads[index]
	switch {
	case cur.Empty():
		return fmt.Errorf("pad %s is empty", PadName(index))
	case p.Start != cur.Start || p.End != cur.End || p.Channels != cur.Channels ||
		p.Format != cur.Format || p.OriginalTempo != cur.OriginalTempo:
		return fmt.Errorf("pad %s: the sample's own fields cannot be changed", PadName(index))
	case p.Volume > 127:
		return fmt.Errorf("volume %d out of range 0–127", p.Volume)
	case p.UserTempo < 400 || p.UserTempo > 2000:
		return fmt.Errorf("tempo %.1f out of range 40.0–200.0 BPM", float64(p.UserTempo)/10)
	case p.UserStart < p.Start || p.UserEnd < p.UserStart || p.UserEnd > p.End:
		return fmt.Errorf("trim range %d–%d outside the sample", p.UserStart, p.UserEnd)
	case !p.trimAligned():
		return fmt.Errorf("trim range %d–%d not on 4-byte boundaries", p.UserStart, p.UserEnd)
	case p.TempoMode > 2:
		return fmt.Errorf("tempo mode %d out of range 0–2", p.TempoMode)
	}
	return c.setPad(index, p)
}

// setPad updates one record and writes PAD_INFO.BIN; on failure the
// in-memory state is left unchanged.
func (c *Card) setPad(index int, p Pad) error {
	next := *c.Info
	next.Pads[index] = p
	if err := writeFileAtomic(c.padInfoPath(), next.MarshalBinary()); err != nil {
		return err
	}
	c.Info.Pads[index] = p
	return nil
}

// writeFileAtomic writes to a temp file in the same directory, then renames,
// so an interrupted write never leaves a truncated file on the card.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".wav404-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	err = writeChunked(tmp, data)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// writeChunk bounds a single write call. macOS mounts FAT cards through
// FSKit, and one write of tens of MB to a slow SD card fails with
// "operation timed out" before it completes.
const writeChunk = 1 << 20

// writeChunked writes data in calls of at most writeChunk bytes.
func writeChunked(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n := min(len(data), writeChunk)
		if _, err := w.Write(data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
