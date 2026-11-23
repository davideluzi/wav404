package sp404sx

import (
	"bytes"
	"testing"
)

// recordWriter remembers the size of every Write call.
type recordWriter struct {
	buf   bytes.Buffer
	sizes []int
}

func (w *recordWriter) Write(p []byte) (int, error) {
	w.sizes = append(w.sizes, len(p))
	return w.buf.Write(p)
}

// A large sample must reach the card in small writes: macOS mounts FAT
// cards through FSKit, and one long write to a slow SD card fails with
// "operation timed out".
func TestWriteChunkedLimitsEachWrite(t *testing.T) {
	data := make([]byte, 3*writeChunk+123)
	for i := range data {
		data[i] = byte(i * 7)
	}
	var w recordWriter
	var reports [][2]int64
	if err := writeChunked(&w, data, func(done, total int64) {
		reports = append(reports, [2]int64{done, total})
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.buf.Bytes(), data) {
		t.Fatal("data written differs from input")
	}
	if len(w.sizes) != 4 {
		t.Fatalf("got %d writes, want 4: %v", len(w.sizes), w.sizes)
	}
	for _, n := range w.sizes {
		if n > writeChunk {
			t.Fatalf("write of %d bytes exceeds chunk %d", n, writeChunk)
		}
	}
	total := int64(len(data))
	want := [][2]int64{{writeChunk, total}, {2 * writeChunk, total}, {3 * writeChunk, total}, {total, total}}
	if len(reports) != len(want) {
		t.Fatalf("progress reports %v, want %v", reports, want)
	}
	for i := range want {
		if reports[i] != want[i] {
			t.Fatalf("progress reports %v, want %v", reports, want)
		}
	}
}

func TestWriteChunkedNilProgress(t *testing.T) {
	var w recordWriter
	if err := writeChunked(&w, []byte("abc"), nil); err != nil || w.buf.String() != "abc" {
		t.Fatalf("err=%v got %q", err, w.buf.String())
	}
}
