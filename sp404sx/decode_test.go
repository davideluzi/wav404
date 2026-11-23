package sp404sx

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// reference: the 16-bit PCM of J5, from which all import fixtures were made
// with macOS afconvert.
func refAudio(t *testing.T) []float64 {
	t.Helper()
	raw, err := os.ReadFile("testdata/J0000005.WAV")
	if err != nil {
		t.Fatal(err)
	}
	s, err := DecodeSample(raw)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float64, len(s.PCM))
	for i, v := range s.PCM {
		out[i] = float64(v) / 32768
	}
	return out
}

func TestDecodeAudioFormats(t *testing.T) {
	ref := refAudio(t)
	cases := []struct {
		file string
		tol  float64
	}{
		{"s16.wav", 0},
		{"s24.wav", 0},
		{"s32.wav", 0},
		{"f32.wav", 0},
		{"u8.wav", 1.0 / 128},
		{"s16.aif", 0},
		{"s24.aif", 0},
		{"fl32.aifc", 0},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/import/" + c.file)
			if err != nil {
				t.Fatal(err)
			}
			a, err := DecodeAudio(raw)
			if err != nil {
				t.Fatalf("DecodeAudio: %v", err)
			}
			if a.Rate != 44100 || a.Channels != 2 {
				t.Fatalf("rate=%v ch=%d", a.Rate, a.Channels)
			}
			if len(a.Samples) != len(ref) {
				t.Fatalf("got %d samples, want %d", len(a.Samples), len(ref))
			}
			for i := range ref {
				if d := math.Abs(a.Samples[i] - ref[i]); d > c.tol {
					t.Fatalf("sample %d: got %v want %v (diff %v > %v)", i, a.Samples[i], ref[i], d, c.tol)
				}
			}
		})
	}
}

func TestDecodeAudioRateAndChannels(t *testing.T) {
	cases := []struct {
		file string
		rate float64
		ch   int
	}{
		{"s16_48k.wav", 48000, 2},
		{"s16_22k.wav", 22050, 2},
		{"s24_96k.wav", 96000, 2},
		{"mono.wav", 44100, 1},
	}
	for _, c := range cases {
		raw, err := os.ReadFile("testdata/import/" + c.file)
		if err != nil {
			t.Fatal(err)
		}
		a, err := DecodeAudio(raw)
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if a.Rate != c.rate || a.Channels != c.ch {
			t.Errorf("%s: rate=%v ch=%d, want %v/%d", c.file, a.Rate, a.Channels, c.rate, c.ch)
		}
	}
}

// --- hand-built files for variants afconvert cannot produce ---

func chunk(id string, body []byte, order binary.ByteOrder) []byte {
	b := make([]byte, 8, 8+len(body)+1)
	copy(b, id)
	order.PutUint32(b[4:], uint32(len(body)))
	b = append(b, body...)
	if len(body)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

func riff(chunks ...[]byte) []byte {
	body := []byte("WAVE")
	for _, c := range chunks {
		body = append(body, c...)
	}
	return chunk("RIFF", body, binary.LittleEndian)
}

func TestDecodeWAVExtensible(t *testing.T) {
	le := binary.LittleEndian
	fmtc := make([]byte, 40)
	le.PutUint16(fmtc[0:], 0xFFFE)
	le.PutUint16(fmtc[2:], 1)     // mono
	le.PutUint32(fmtc[4:], 44100) // rate
	le.PutUint32(fmtc[8:], 44100*3)
	le.PutUint16(fmtc[12:], 3)
	le.PutUint16(fmtc[14:], 24) // container bits
	le.PutUint16(fmtc[16:], 22) // cbSize
	le.PutUint16(fmtc[18:], 24) // valid bits
	le.PutUint16(fmtc[24:], 1)  // subformat GUID starts with 0x0001 = PCM
	data := []byte{0x00, 0x00, 0x40 /* +0.5 */, 0x00, 0x00, 0xC0 /* -0.5 */}
	a, err := DecodeAudio(riff(chunk("fmt ", fmtc, le), chunk("data", data, le)))
	if err != nil {
		t.Fatal(err)
	}
	if a.Channels != 1 || len(a.Samples) != 2 || a.Samples[0] != 0.5 || a.Samples[1] != -0.5 {
		t.Errorf("got ch=%d %v", a.Channels, a.Samples)
	}
}

func TestDecodeAIFCSowt(t *testing.T) {
	be := binary.BigEndian
	comm := make([]byte, 22)
	be.PutUint16(comm[0:], 1) // channels
	be.PutUint32(comm[2:], 2) // frames
	be.PutUint16(comm[6:], 16)
	copy(comm[8:], []byte{0x40, 0x0E, 0xAC, 0x44, 0, 0, 0, 0, 0, 0}) // 44100 as 80-bit extended
	copy(comm[18:], "sowt")
	ssnd := append(make([]byte, 8), 0x00, 0x40 /* LE 0x4000 = +0.5 */, 0x00, 0xC0 /* -0.5 */)
	body := append([]byte("AIFC"), chunk("COMM", comm, be)...)
	body = append(body, chunk("SSND", ssnd, be)...)
	a, err := DecodeAudio(chunk("FORM", body, be))
	if err != nil {
		t.Fatal(err)
	}
	if a.Rate != 44100 || len(a.Samples) != 2 || a.Samples[0] != 0.5 || a.Samples[1] != -0.5 {
		t.Errorf("got rate=%v %v", a.Rate, a.Samples)
	}
}

func TestDecodeAudioRejectsCompressed(t *testing.T) {
	le := binary.LittleEndian
	fmtc := make([]byte, 16)
	le.PutUint16(fmtc[0:], 2) // MS ADPCM
	le.PutUint16(fmtc[2:], 1)
	le.PutUint32(fmtc[4:], 44100)
	le.PutUint16(fmtc[14:], 4)
	if _, err := DecodeAudio(riff(chunk("fmt ", fmtc, le), chunk("data", []byte{1, 2}, le))); err == nil {
		t.Error("expected error for compressed WAV")
	}
	if _, err := DecodeAudio([]byte("ID3\x03 not audio at all")); err == nil {
		t.Error("expected error for non-WAV/AIFF data")
	}
}
