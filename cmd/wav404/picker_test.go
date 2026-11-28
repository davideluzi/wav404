package main

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/davideluzi/wav404/sp404sx"
)

// fakePicker records the folder each dialog starts in and lets the test
// decide when and how the dialog closes.
type fakePicker struct {
	dir, name string
	openDone  func(string, error)
	saveDone  func(string, error)
	dirDone   func(string, error)
}

func (f *fakePicker) openAudio(dir string, done func(string, error)) {
	f.dir, f.openDone = dir, done
}

func (f *fakePicker) saveWAV(dir, name string, done func(string, error)) {
	f.dir, f.name, f.saveDone = dir, name, done
}

func (f *fakePicker) openFolder(dir string, done func(string, error)) {
	f.dir, f.dirDone = dir, done
}

func pickerGUI(t *testing.T) (*gui, *fakePicker, string) {
	t.Helper()
	g, root := newTestGUI(t)
	fp := &fakePicker{}
	g.picker = fp
	return g, fp, root
}

func fixture(name string) string {
	p, _ := filepath.Abs(filepath.Join(testdata, "import", name))
	return p
}

// The picker imports exactly one file, into the selected pad only.
func TestImportOneFileFromPicker(t *testing.T) {
	g, fp, root := pickerGUI(t)
	a2Before := g.card.Info.Pads[1]
	test.Tap(g.tiles[0]) // A1
	test.Tap(g.importBtn)
	fp.openDone(fixture("s24.aif"), nil)

	c, _ := sp404sx.OpenCard(root)
	if p := c.Info.Pads[0]; p.Empty() || p.Tempo != 890 {
		t.Errorf("A1 after import = %+v", p)
	}
	if c.Info.Pads[1] != a2Before {
		t.Error("A2 changed: the import spilled into the next pad")
	}
	if got := g.prefs().String(prefImportDir); got != filepath.Dir(fixture("s24.aif")) {
		t.Errorf("remembered import folder = %q", got)
	}
}

func TestImportDialogStartsInLastFolder(t *testing.T) {
	g, fp, _ := pickerGUI(t)
	g.prefs().SetString(prefImportDir, "/some/samples")
	test.Tap(g.tiles[0])
	test.Tap(g.importBtn)
	if fp.dir != "/some/samples" {
		t.Errorf("dialog started in %q", fp.dir)
	}
}

// The native dialog does not block the window: the target pad is the one
// selected when the dialog opened.
func TestImportTargetFixedWhenDialogOpens(t *testing.T) {
	g, fp, root := pickerGUI(t)
	test.Tap(g.tiles[0]) // A1
	test.Tap(g.importBtn)
	test.Tap(g.tiles[9]) // user clicks A10 while the dialog is open
	fp.openDone(fixture("s16.wav"), nil)
	c, _ := sp404sx.OpenCard(root)
	if c.Info.Pads[0].Empty() {
		t.Error("file not imported into A1")
	}
	if c.Info.Pads[9].Frames() != 26680 {
		t.Error("A10 was overwritten")
	}
}

func TestCancelledImportChangesNothing(t *testing.T) {
	g, fp, root := pickerGUI(t)
	before, _ := os.Stat(filepath.Join(root, "ROLAND", "SP-404SX", "SMPL", "PAD_INFO.BIN"))
	test.Tap(g.tiles[0])
	test.Tap(g.importBtn)
	fp.openDone("", nil)
	after, _ := os.Stat(filepath.Join(root, "ROLAND", "SP-404SX", "SMPL", "PAD_INFO.BIN"))
	if !os.SameFile(before, after) {
		t.Error("cancelled import wrote to the card")
	}
}

func TestExportThroughPicker(t *testing.T) {
	g, fp, _ := pickerGUI(t)
	g.prefs().SetString(prefExportDir, "/exports")
	test.Tap(g.tiles[9]) // A10
	test.Tap(g.exportBtn)
	if fp.dir != "/exports" || fp.name != "A10.wav" {
		t.Errorf("save dialog in %q as %q", fp.dir, fp.name)
	}
	dst := filepath.Join(t.TempDir(), "out.wav")
	fp.saveDone(dst, nil)
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if a, err := sp404sx.DecodeAudio(raw); err != nil || a.Frames() != 26680 {
		t.Errorf("exported file: %v", err)
	}
	if got := g.prefs().String(prefExportDir); got != filepath.Dir(dst) {
		t.Errorf("remembered export folder = %q", got)
	}
}

func TestOpenFolderThroughPicker(t *testing.T) {
	g, fp, _ := pickerGUI(t)
	other := fakeCard(t)
	g.openFolderDialog()
	fp.dirDone(other, nil)
	if g.card == nil || g.card.Root != other {
		t.Errorf("card root = %v", g.card)
	}
}

func TestChoosePicker(t *testing.T) {
	w := test.NewTempApp(t).NewWindow("x")
	if _, ok := choosePicker(w, true).(nativePicker); !ok {
		t.Error("native dialogs available but not used")
	}
	if _, ok := choosePicker(w, false).(fynePicker); !ok {
		t.Error("no fallback to the Fyne dialogs")
	}
}
