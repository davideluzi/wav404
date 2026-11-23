package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davideluzi/wav404/sp404sx"
)

const testdata = "../../sp404sx/testdata"

func fakeCard(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	smpl := filepath.Join(root, "ROLAND", "SP-404SX", "SMPL")
	if err := os.MkdirAll(smpl, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"PAD_INFO.BIN", "J0000005.WAV", "A0000010.WAV"} {
		b, err := os.ReadFile(filepath.Join(testdata, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(smpl, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestTileText(t *testing.T) {
	c, err := sp404sx.OpenCard(fakeCard(t))
	if err != nil {
		t.Fatal(err)
	}
	j5 := tileFor(c, sp404sx.PadIndex('J', 5))
	if j5.title != "J5" || j5.empty || !strings.Contains(j5.detail, "0.67 s") || !strings.Contains(j5.detail, "89.0") {
		t.Errorf("J5 tile = %+v", j5)
	}
	if !strings.Contains(j5.flags, "gate") {
		t.Errorf("J5 flags = %q", j5.flags)
	}
	a1 := tileFor(c, 0)
	if a1.title != "A1" || !a1.empty || a1.detail != "empty" {
		t.Errorf("A1 tile = %+v", a1)
	}
	if none := tileFor(nil, 0); !none.empty || none.title != "A1" {
		t.Errorf("tile without card = %+v", none)
	}
}

func TestDropTargets(t *testing.T) {
	c, _ := sp404sx.OpenCard(fakeCard(t))
	// two files dropped on A1 (empty) fill A1 and A2 (assigned → overwrite)
	pads, overwrite := dropTargets(c, 0, 2)
	if len(pads) != 2 || pads[0] != 0 || pads[1] != 1 {
		t.Errorf("pads = %v, want [0 1]", pads)
	}
	if len(overwrite) != 1 || overwrite[0] != "A2" {
		t.Errorf("overwrite = %v, want [A2]", overwrite)
	}
	// never past the last pad
	pads, _ = dropTargets(c, sp404sx.NumPads-1, 5)
	if len(pads) != 1 {
		t.Errorf("drop on J12 with 5 files → %v", pads)
	}
}

func TestFindCards(t *testing.T) {
	card := fakeCard(t)
	other := t.TempDir()
	got := findCards([]string{other, card, filepath.Join(other, "missing")})
	if len(got) != 1 || got[0] != card {
		t.Errorf("findCards = %v, want [%s]", got, card)
	}
}
