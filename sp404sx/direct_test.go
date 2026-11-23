package sp404sx

import (
	"encoding/binary"
	"os"
	"runtime"
	"testing"
)

func parseFixture(t *testing.T, name string) *pcmData {
	t.Helper()
	raw, err := os.ReadFile("testdata/import/" + name)
	if err != nil {
		t.Fatal(err)
	}
	p, err := parseAudio(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDirectCopyUsedOnlyFor16Bit44k(t *testing.T) {
	cases := map[string]bool{
		"s16.wav":     true,
		"s16.aif":     true,
		"mono.wav":    true,
		"s24.wav":     false,
		"f32.wav":     false,
		"u8.wav":      false,
		"s16_48k.wav": false,
	}
	for f, want := range cases {
		if _, ok := directSample(parseFixture(t, f)); ok != want {
			t.Errorf("%s: direct copy = %v, want %v", f, ok, want)
		}
	}
}

func TestDirectCopyBitIdentical(t *testing.T) {
	ref := refSample(t)
	for _, f := range []string{"s16.wav", "s16.aif"} { // little- and big-endian
		s, ok := directSample(parseFixture(t, f))
		if !ok {
			t.Fatalf("%s: direct copy not used", f)
		}
		if s.Channels != 2 || len(s.PCM) != len(ref.PCM) {
			t.Fatalf("%s: ch=%d len=%d", f, s.Channels, len(s.PCM))
		}
		for i := range ref.PCM {
			if s.PCM[i] != ref.PCM[i] {
				t.Fatalf("%s: sample %d = %d, want %d", f, i, s.PCM[i], ref.PCM[i])
			}
		}
	}
}

func TestDirectCopySowt(t *testing.T) {
	be := binary.BigEndian
	comm := make([]byte, 22)
	be.PutUint16(comm[0:], 1)
	be.PutUint32(comm[2:], 2)
	be.PutUint16(comm[6:], 16)
	copy(comm[8:], []byte{0x40, 0x0E, 0xAC, 0x44, 0, 0, 0, 0, 0, 0})
	copy(comm[18:], "sowt")
	ssnd := append(make([]byte, 8), 0x34, 0x12, 0xFE, 0xFF) // LE 0x1234, -2
	body := append([]byte("AIFC"), chunk("COMM", comm, be)...)
	body = append(body, chunk("SSND", ssnd, be)...)
	p, err := parseAudio(chunk("FORM", body, be))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := directSample(p)
	if !ok || len(s.PCM) != 2 || s.PCM[0] != 0x1234 || s.PCM[1] != -2 {
		t.Errorf("ok=%v pcm=%v", ok, s)
	}
}

func TestDirectImportStillChecksLength(t *testing.T) {
	le := binary.LittleEndian
	fmtc := make([]byte, 16)
	le.PutUint16(fmtc[0:], 1)
	le.PutUint16(fmtc[2:], 1)
	le.PutUint32(fmtc[4:], 44100)
	le.PutUint16(fmtc[14:], 16)
	short := riff(chunk("fmt ", fmtc, le), chunk("data", make([]byte, 2*3042), le))
	if s, err := ImportAudio(short); err != ErrTooShort || s != nil {
		t.Errorf("sample returned: %v, err = %v; want nil, ErrTooShort", s != nil, err)
	}
}

func TestDirectImportAllocatesLittle(t *testing.T) {
	raw, err := os.ReadFile("testdata/import/s16.wav")
	if err != nil {
		t.Fatal(err)
	}
	pcmBytes := uint64(118916)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	if _, err := ImportAudio(raw); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	// The float64 path needs ≥ 5× the PCM size (4× floats + 1× int16).
	if got := after.TotalAlloc - before.TotalAlloc; got > 2*pcmBytes {
		t.Errorf("ImportAudio allocated %d bytes for %d bytes of PCM", got, pcmBytes)
	}
}
