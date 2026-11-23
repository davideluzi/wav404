package main

import (
	"math"
	"testing"
)

type report struct {
	frac   float64
	detail string
}

// An import reports a growing fraction that ends at 1, and names the file
// and its pad.
func TestImportReportsProgress(t *testing.T) {
	g, _ := newTestGUI(t)
	var reports []report
	g.runTask = func(_ string, task func(progressFunc) error, done func(error)) {
		done(task(func(f float64, d string) { reports = append(reports, report{f, d}) }))
	}
	g.importInto(0, fixture("s24.wav"))

	if len(reports) < 2 {
		t.Fatalf("got %d reports", len(reports))
	}
	for i := 1; i < len(reports); i++ {
		if reports[i].frac < reports[i-1].frac {
			t.Fatalf("progress went backwards: %v", reports)
		}
	}
	last := reports[len(reports)-1]
	if math.Abs(last.frac-1) > 1e-9 {
		t.Errorf("last fraction = %v, want 1", last.frac)
	}
	if last.detail != "A1 ← s24.wav" {
		t.Errorf("detail = %q, want %q", last.detail, "A1 ← s24.wav")
	}
}
