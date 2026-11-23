package sp404sx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// fakeCard builds a minimal card in a temp dir from the testdata copies.
func fakeCard(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	smpl := filepath.Join(root, "ROLAND", "SP-404SX", "SMPL")
	if err := os.MkdirAll(smpl, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"PAD_INFO.BIN", "J0000005.WAV", "A0000010.WAV"} {
		b, err := os.ReadFile(filepath.Join("testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(smpl, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestOpenCardAndReadSample(t *testing.T) {
	c, err := OpenCard(fakeCard(t))
	if err != nil {
		t.Fatal(err)
	}
	j5 := PadIndex('J', 5)
	s, err := c.ReadSample(j5)
	if err != nil {
		t.Fatal(err)
	}
	if s.Frames() != c.Info.Pads[j5].Frames() {
		t.Errorf("sample frames %d != pad frames %d", s.Frames(), c.Info.Pads[j5].Frames())
	}
}

func TestOpenCardRejectsNonCard(t *testing.T) {
	if _, err := OpenCard(t.TempDir()); err == nil {
		t.Error("expected error for a directory without PAD_INFO.BIN")
	}
}

func TestAssignWritesSampleAndPad(t *testing.T) {
	root := fakeCard(t)
	c, _ := OpenCard(root)
	before := c.Info.Pads

	src, _ := c.ReadSample(PadIndex('J', 5))
	b2 := PadIndex('B', 2)
	if err := c.Assign(b2, src); err != nil {
		t.Fatal(err)
	}

	c2, err := OpenCard(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c2.ReadSample(b2)
	if err != nil {
		t.Fatal(err)
	}
	if got.PadIndex != b2 {
		t.Errorf("RLND pad index = %d, want %d", got.PadIndex, b2)
	}
	if c2.Info.Pads[b2] != src.NewPad() {
		t.Errorf("pad B2 = %+v, want %+v", c2.Info.Pads[b2], src.NewPad())
	}
	for i := range before {
		if i != b2 && c2.Info.Pads[i] != before[i] {
			t.Errorf("pad %d changed unexpectedly", i)
		}
	}
	// the source sample must not have been modified by Assign
	if src.PadIndex != PadIndex('J', 5) {
		t.Error("Assign mutated the caller's Sample")
	}
}

func TestClearRemovesFileAndResetsPad(t *testing.T) {
	root := fakeCard(t)
	c, _ := OpenCard(root)
	j5 := PadIndex('J', 5)
	if err := c.Clear(j5); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.SamplePath(j5)); !os.IsNotExist(err) {
		t.Errorf("sample file still present: %v", err)
	}
	c2, _ := OpenCard(root)
	if c2.Info.Pads[j5] != EmptyPad() {
		t.Errorf("pad J5 = %+v, want empty", c2.Info.Pads[j5])
	}
}

func TestClearOfEmptyPadKeepsPadInfoIdentical(t *testing.T) {
	root := fakeCard(t)
	c, _ := OpenCard(root)
	if err := c.Clear(PadIndex('A', 1)); err != nil { // A1 is already empty
		t.Fatal(err)
	}
	orig, _ := os.ReadFile("testdata/PAD_INFO.BIN")
	got, _ := os.ReadFile(filepath.Join(root, "ROLAND", "SP-404SX", "SMPL", "PAD_INFO.BIN"))
	if !bytes.Equal(orig, got) {
		t.Error("PAD_INFO.BIN changed")
	}
}

// If PAD_INFO.BIN cannot be written, Clear must not have deleted the sample:
// a pad must never point to a missing file.
func TestClearKeepsFileWhenPadInfoWriteFails(t *testing.T) {
	root := fakeCard(t)
	c, _ := OpenCard(root)
	pi := filepath.Join(root, "ROLAND", "SP-404SX", "SMPL", "PAD_INFO.BIN")
	os.Remove(pi)
	if err := os.Mkdir(pi, 0o755); err != nil { // the rename onto it will fail
		t.Fatal(err)
	}
	j5 := PadIndex('J', 5)
	if err := c.Clear(j5); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(c.SamplePath(j5)); err != nil {
		t.Errorf("sample deleted although PAD_INFO.BIN was not updated: %v", err)
	}
}

// TestRealCard checks every assigned pad of a real card against its WAV.
// Read-only. Run with: SP404_CARD=/Volumes/SP-404SX go test ./sp404sx -run RealCard
func TestRealCard(t *testing.T) {
	root := os.Getenv("SP404_CARD")
	if root == "" {
		t.Skip("SP404_CARD not set")
	}
	c, err := OpenCard(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i, p := range c.Info.Pads {
		if p.Empty() {
			continue
		}
		n++
		raw, err := os.ReadFile(c.SamplePath(i))
		if err != nil {
			t.Errorf("pad %d: %v", i, err)
			continue
		}
		s, err := DecodeSample(raw)
		if err != nil {
			t.Errorf("pad %d: %v", i, err)
			continue
		}
		if int(p.End) != len(raw) || int(p.Channels) != s.Channels || s.PadIndex != i {
			t.Errorf("pad %d: end=%d file=%d ch=%d/%d rlnd=%d", i, p.End, len(raw), p.Channels, s.Channels, s.PadIndex)
		}
		if !bytes.Equal(s.Encode(), raw) {
			t.Errorf("pad %d: re-encode differs", i)
		}
	}
	t.Logf("%d samples verified", n)
}
