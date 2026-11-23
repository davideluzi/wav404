package sp404sx

import "testing"

// Expected values come from PAD_INFO.BIN on a real card (pads imported
// unchanged) and from the logic disassembled from Roland's converter.
func TestComputeBPM(t *testing.T) {
	cases := []struct {
		name   string
		frames int
		want   int
	}{
		{"zero length is the converter default", 0, 1020},
		{"exactly one 4/4 bar at 120 BPM", 88200, 1200},
		{"J5 short loop", 118916 / 4, 890},
		{"A10 mono one-shot", 53360 / 2, 991},
		{"A9 very short", 80292 / 4, 1318},
		{"E1 long loop", 3603060 / 4, 1175},
		{"F5", 1336924 / 4, 1266},
		{"C1 exactly one bar", 470400 / 4, 900},
		{"A3 long sample", 2385124 / 4, 1065},
		{"G1 very long sample", 37808400 / 4, 1209},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ComputeBPM(c.frames); got != c.want {
				t.Errorf("ComputeBPM(%d) = %d, want %d", c.frames, got, c.want)
			}
		})
	}
}
