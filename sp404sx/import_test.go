package sp404sx

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"testing"
)

func refSample(t *testing.T) *Sample {
	t.Helper()
	raw, err := os.ReadFile("testdata/J0000005.WAV")
	if err != nil {
		t.Fatal(err)
	}
	s, err := DecodeSample(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func importFixture(t *testing.T, name string) *Sample {
	t.Helper()
	raw, err := os.ReadFile("testdata/import/" + name)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ImportAudio(raw)
	if err != nil {
		t.Fatalf("ImportAudio(%s): %v", name, err)
	}
	return s
}

func TestImportLosslessFormatsMatchOriginal(t *testing.T) {
	ref := refSample(t)
	for _, f := range []string{"s16.wav", "s24.wav", "s32.wav", "f32.wav", "s24.aif", "fl32.aifc"} {
		s := importFixture(t, f)
		if s.Channels != 2 || len(s.PCM) != len(ref.PCM) {
			t.Errorf("%s: ch=%d len=%d", f, s.Channels, len(s.PCM))
			continue
		}
		for i := range ref.PCM {
			if s.PCM[i] != ref.PCM[i] {
				t.Errorf("%s: sample %d = %d, want %d", f, i, s.PCM[i], ref.PCM[i])
				break
			}
		}
	}
}

func TestImportResamples48k(t *testing.T) {
	ref := refSample(t)
	s := importFixture(t, "s16_48k.wav")
	if s.Channels != 2 {
		t.Fatalf("channels = %d", s.Channels)
	}
	// afconvert went 44.1→48 kHz, we go back: same length within a frame,
	// and the audio must closely match the original.
	if d := s.Frames() - ref.Frames(); d < -1 || d > 1 {
		t.Fatalf("frames = %d, original %d", s.Frames(), ref.Frames())
	}
	var diff, sig float64
	n := min(len(s.PCM), len(ref.PCM))
	for i := 400; i < n-400; i++ { // skip filter edges
		d := float64(s.PCM[i]) - float64(ref.PCM[i])
		diff += d * d
		sig += float64(ref.PCM[i]) * float64(ref.PCM[i])
	}
	if snr := 10 * math.Log10(sig/diff); snr < 40 {
		t.Errorf("round-trip through 48 kHz: SNR %.1f dB < 40 dB", snr)
	}
}

func TestImportKeepsMono(t *testing.T) {
	if s := importFixture(t, "mono.wav"); s.Channels != 1 {
		t.Errorf("channels = %d", s.Channels)
	}
}

func TestQuantizeClipsAndRounds(t *testing.T) {
	a := &Audio{Rate: 44100, Channels: 1, Samples: make([]float64, 4000)}
	copy(a.Samples, []float64{1.5, -1.5, 0.5, -0.5, 1.0 / 65536 * 1.01})
	s := audioToSample(a)
	want := []int16{32767, -32768, 16384, -16384, 1}
	for i, w := range want {
		if s.PCM[i] != w {
			t.Errorf("PCM[%d] = %d, want %d", i, s.PCM[i], w)
		}
	}
}

func TestImportMinimumLength(t *testing.T) {
	if err := checkLength(&Sample{Channels: 1, PCM: make([]int16, 3043)}); err != nil { // 0.06900 s
		t.Errorf("3043 frames rejected: %v", err)
	}
	if err := checkLength(&Sample{Channels: 2, PCM: make([]int16, 2*3042)}); !errors.Is(err, ErrTooShort) {
		t.Errorf("3042 stereo frames: err = %v, want ErrTooShort", err)
	}
}

// A file that only reaches 0.069 s after resampling is judged on the
// converted length, like the Roland converter does.
func TestImportMinimumLengthAfterResample(t *testing.T) {
	le := binary.LittleEndian
	fmtc := make([]byte, 16)
	le.PutUint16(fmtc[0:], 1)
	le.PutUint16(fmtc[2:], 1)
	le.PutUint32(fmtc[4:], 48000)
	le.PutUint16(fmtc[14:], 16)
	wav := func(frames int) []byte {
		return riff(chunk("fmt ", fmtc, le), chunk("data", make([]byte, 2*frames), le))
	}
	if _, err := ImportAudio(wav(3312)); err != nil { // → 3043 frames at 44.1 kHz
		t.Errorf("3312 frames @48k rejected: %v", err)
	}
	if _, err := ImportAudio(wav(3310)); !errors.Is(err, ErrTooShort) { // → 3041
		t.Errorf("3310 frames @48k: err = %v, want ErrTooShort", err)
	}
}

func TestImportRejectsMultichannel(t *testing.T) {
	le := binary.LittleEndian
	for _, bits := range []int{16, 24} { // direct and converted paths
		fmtc := make([]byte, 16)
		le.PutUint16(fmtc[0:], 1)
		le.PutUint16(fmtc[2:], 3)
		le.PutUint32(fmtc[4:], 44100)
		le.PutUint16(fmtc[14:], uint16(bits))
		wav := riff(chunk("fmt ", fmtc, le), chunk("data", make([]byte, 3*bits/8*5000), le))
		if s, err := ImportAudio(wav); !errors.Is(err, ErrChannels) || s != nil {
			t.Errorf("%d bit: sample returned: %v, err = %v; want nil, ErrChannels", bits, s != nil, err)
		}
	}
}
