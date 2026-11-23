package main

import (
	"os"
	"testing"
	"time"
)

// TestOtoPlayerSmoke plays 0.2 s of silence on the real audio output.
// Needs an audio device: WAV404_AUDIO_TEST=1 go test ./cmd/wav404 -run Oto
func TestOtoPlayerSmoke(t *testing.T) {
	if os.Getenv("WAV404_AUDIO_TEST") == "" {
		t.Skip("WAV404_AUDIO_TEST not set")
	}
	p := &otoPlayer{}
	ended := make(chan struct{}, 1)
	silence := make([]byte, 44100*4/5) // 0.2 s stereo 16-bit
	start := time.Now()
	if err := p.play(silence, func() { ended <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ended:
		if d := time.Since(start); d < 150*time.Millisecond {
			t.Errorf("ended after %v, too early for 0.2 s", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("playback did not end")
	}

	// stop() suppresses the end callback
	if err := p.play(silence, func() { t.Error("onEnd after stop") }); err != nil {
		t.Fatal(err)
	}
	p.stop()
	time.Sleep(400 * time.Millisecond)
}
