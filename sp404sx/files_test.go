package sp404sx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePadAndPadName(t *testing.T) {
	good := map[string]int{"A1": 0, "a1": 0, "J12": 119, "b3": 14, "C10": 33}
	for in, want := range good {
		if got, err := ParsePad(in); err != nil || got != want {
			t.Errorf("ParsePad(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "K1", "A0", "A13", "A", "1A", "Ax"} {
		if _, err := ParsePad(in); err == nil {
			t.Errorf("ParsePad(%q) accepted", in)
		}
	}
	for i := range NumPads {
		if got, _ := ParsePad(PadName(i)); got != i {
			t.Errorf("ParsePad(PadName(%d)) = %d", i, got)
		}
	}
	if PadName(119) != "J12" {
		t.Errorf("PadName(119) = %q", PadName(119))
	}
}

func TestImportFile(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	if err := c.ImportFile(0, "testdata/import/s24.aif"); err != nil {
		t.Fatal(err)
	}
	s, err := c.ReadSample(0)
	if err != nil {
		t.Fatal(err)
	}
	if s.Frames() != 118916/4 || c.Info.Pads[0].OriginalTempo != 890 {
		t.Errorf("frames=%d bpm=%d", s.Frames(), c.Info.Pads[0].OriginalTempo)
	}
}

// ImportFileProgress reports the bytes of the sample file as they are
// written, ending with done == total == the size of the file on the card.
func TestImportFileProgress(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	var last, calls int64
	var total int64 = -1
	err := c.ImportFileProgress(0, "testdata/import/s24.aif", func(done, tot int64) {
		if done < last || (total >= 0 && tot != total) {
			t.Fatalf("progress went from %d/%d to %d/%d", last, total, done, tot)
		}
		last, total = done, tot
		calls++
	})
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(c.SamplePath(0))
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 || last != fi.Size() || total != fi.Size() {
		t.Errorf("last report %d/%d after %d calls; file is %d bytes", last, total, calls, fi.Size())
	}
}

func TestImportFileErrorLeavesPadUntouched(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	j5 := PadIndex('J', 5)
	before := c.Info.Pads[j5]
	bad := filepath.Join(t.TempDir(), "x.wav")
	os.WriteFile(bad, []byte("not audio"), 0o644)
	if err := c.ImportFile(j5, bad); err == nil {
		t.Fatal("expected error")
	}
	if c.Info.Pads[j5] != before {
		t.Error("pad changed after a failed import")
	}
}

func TestExportFile(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	j5 := PadIndex('J', 5)
	dir := t.TempDir()

	full := filepath.Join(dir, "full.wav")
	if err := c.ExportFile(j5, full, false); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(full)
	a, err := DecodeAudio(raw)
	if err != nil || a.Frames() != 118916/4 || bytes.Contains(raw, []byte("RLND")) {
		t.Fatalf("full export: err=%v", err)
	}

	// trimmed export follows the pad's user range
	c.Info.Pads[j5].UserStart = HeaderSize + 4*100
	trimmed := filepath.Join(dir, "trim.wav")
	if err := c.ExportFile(j5, trimmed, true); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(trimmed)
	if a, _ := DecodeAudio(raw); a.Frames() != 118916/4-100 {
		t.Errorf("trimmed frames = %d", a.Frames())
	}
}

func TestExportEmptyPadFails(t *testing.T) {
	c, _ := OpenCard(fakeCard(t))
	if err := c.ExportFile(0, filepath.Join(t.TempDir(), "x.wav"), false); err == nil {
		t.Error("expected error exporting an empty pad")
	}
}
