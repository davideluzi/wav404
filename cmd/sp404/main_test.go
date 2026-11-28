package main

import (
	"bytes"
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

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(args, &out)
	return out.String(), err
}

func TestListShowsAssignedPads(t *testing.T) {
	out, err := runCLI(t, "list", fakeCard(t))
	if err != nil {
		t.Fatal(err)
	}
	j5 := lineStarting(out, "J5 ")
	if j5 == "" || !strings.Contains(j5, "89.0") || !strings.Contains(j5, "stereo") {
		t.Errorf("J5 line missing or wrong: %q\n%s", j5, out)
	}
	if lineStarting(out, "A1 ") != "" {
		t.Errorf("empty pad A1 listed:\n%s", out)
	}
}

func lineStarting(s, prefix string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

func TestImportCommand(t *testing.T) {
	root := fakeCard(t)
	src := filepath.Join(testdata, "import", "s16_48k.wav")
	if _, err := runCLI(t, "import", root, "A1", src); err != nil {
		t.Fatal(err)
	}
	c, err := sp404sx.OpenCard(root)
	if err != nil {
		t.Fatal(err)
	}
	p := c.Info.Pads[0]
	if p.Empty() || p.Channels != 2 || p.Tempo != 890 {
		t.Errorf("pad A1 after import = %+v", p)
	}
	if _, err := c.ReadSample(0); err != nil {
		t.Error(err)
	}
}

func TestImportCommandRejectsBadFile(t *testing.T) {
	root := fakeCard(t)
	bad := filepath.Join(t.TempDir(), "x.wav")
	os.WriteFile(bad, []byte("not audio"), 0o644)
	if _, err := runCLI(t, "import", root, "A1", bad); err == nil {
		t.Fatal("expected error")
	}
	c, _ := sp404sx.OpenCard(root)
	if !c.Info.Pads[0].Empty() {
		t.Error("failed import modified the card")
	}
}

func TestExportCommand(t *testing.T) {
	root := fakeCard(t)
	dst := filepath.Join(t.TempDir(), "j5.wav")
	if _, err := runCLI(t, "export", root, "J5", dst); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	a, err := sp404sx.DecodeAudio(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Frames() != 118916/4 || bytes.Contains(raw, []byte("RLND")) {
		t.Errorf("frames=%d, RLND present=%v", a.Frames(), bytes.Contains(raw, []byte("RLND")))
	}
}

func TestExportEmptyPadFails(t *testing.T) {
	if _, err := runCLI(t, "export", fakeCard(t), "A1", filepath.Join(t.TempDir(), "x.wav")); err == nil {
		t.Error("expected error exporting an empty pad")
	}
}

func TestClearCommand(t *testing.T) {
	root := fakeCard(t)
	if _, err := runCLI(t, "clear", root, "J5"); err != nil {
		t.Fatal(err)
	}
	c, _ := sp404sx.OpenCard(root)
	if !c.Info.Pads[sp404sx.PadIndex('J', 5)].Empty() {
		t.Error("J5 not cleared")
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"list"}, {"import", "x", "A1"}} {
		if _, err := runCLI(t, args...); err == nil {
			t.Errorf("run(%q) succeeded", args)
		}
	}
}
