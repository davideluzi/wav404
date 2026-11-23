package main

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// Each file weighs in proportion to its size; the batch fraction is the
// share of all bytes already done.
func TestBatchProgressWeighsFilesBySize(t *testing.T) {
	b := newBatchProgress([]int64{100, 300})
	cases := []struct {
		file        int
		done, total int64
		want        float64
	}{
		{0, 0, 10, 0},
		{0, 5, 10, 0.125},
		{0, 10, 10, 0.25},
		{1, 0, 10, 0.25},
		{1, 5, 10, 0.625},
		{1, 10, 10, 1},
	}
	for _, c := range cases {
		if got := b.fraction(c.file, c.done, c.total); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("fraction(%d, %d/%d) = %v, want %v", c.file, c.done, c.total, got, c.want)
		}
	}
}

// Files whose size is unknown (0) still get an equal share.
func TestBatchProgressUnknownSizes(t *testing.T) {
	b := newBatchProgress([]int64{0, 0})
	if got := b.fraction(1, 1, 2); math.Abs(got-0.75) > 1e-9 {
		t.Errorf("fraction = %v, want 0.75", got)
	}
}

type report struct {
	frac   float64
	detail string
}

// Importing several files reports a growing fraction that ends at 1, and
// names the file being written.
func TestImportReportsProgress(t *testing.T) {
	g, _ := newTestGUI(t)
	var reports []report
	g.runTask = func(_ string, task func(progressFunc) error, done func(error)) {
		done(task(func(f float64, d string) { reports = append(reports, report{f, d}) }))
	}
	a, _ := filepath.Abs(filepath.Join(testdata, "import", "s24.wav"))
	b, _ := filepath.Abs(filepath.Join(testdata, "import", "s16.wav"))
	g.importInto(0, []string{a, b})

	if len(reports) < 2 {
		t.Fatalf("got %d reports", len(reports))
	}
	for i := 1; i < len(reports); i++ {
		if reports[i].frac < reports[i-1].frac {
			t.Fatalf("progress went backwards: %v", reports)
		}
	}
	if last := reports[len(reports)-1]; math.Abs(last.frac-1) > 1e-9 {
		t.Errorf("last fraction = %v, want 1", last.frac)
	}
	if !strings.Contains(reports[0].detail, "1 of 2") || !strings.Contains(reports[0].detail, "s24.wav") {
		t.Errorf("first detail = %q", reports[0].detail)
	}
	if last := reports[len(reports)-1].detail; !strings.Contains(last, "2 of 2") || !strings.Contains(last, "s16.wav") {
		t.Errorf("last detail = %q", last)
	}
}
