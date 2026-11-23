package main

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/davideluzi/wav404/sp404sx"
)

// slowCard makes sample reads wait until the test drains them, like a
// slow SD card, and counts the reads.
type slowCard struct {
	queue []func()
	reads map[int]int
}

func withSlowCard(t *testing.T) (*gui, *slowCard) {
	t.Helper()
	g, _ := newTestGUI(t)
	sc := &slowCard{reads: map[int]int{}}
	real := g.readSample
	g.readSample = func(c *sp404sx.Card, i int) (*sp404sx.Sample, error) { sc.reads[i]++; return real(c, i) }
	g.async = func(work, done func()) { sc.queue = append(sc.queue, func() { work(); done() }) }
	return g, sc
}

// drain completes the pending reads (and any they start).
func (sc *slowCard) drain() {
	for len(sc.queue) > 0 {
		next := sc.queue[0]
		sc.queue = sc.queue[1:]
		next()
	}
}

func TestSelectionDoesNotWaitForTheCard(t *testing.T) {
	g, sc := withSlowCard(t)
	test.Tap(g.tiles[9]) // A10, file not read yet
	if g.selected != sp404sx.PadIndex('A', 10) || !g.tiles[9].selected {
		t.Fatal("selection waited for the sample file")
	}
	if g.wave.sample != nil || !strings.Contains(g.trimLabel.Text, "LOADING") {
		t.Errorf("while loading: wave %v, label %q", g.wave.sample != nil, g.trimLabel.Text)
	}
	if g.volume.Value != 127 || !g.gate.Checked {
		t.Error("pad settings should show at once, from PAD_INFO")
	}
	sc.drain()
	if g.wave.sample == nil || g.wave.frames() != 26680 {
		t.Error("waveform not shown after the read finished")
	}
}

func TestLatestSelectionWins(t *testing.T) {
	g, sc := withSlowCard(t)
	test.Tap(g.tiles[9]) // A10: read starts
	selectJ5(g)          // J5 before A10 has been read
	sc.drain()
	if g.wave.frames() != 29729 {
		t.Errorf("wave shows %d frames, want J5's 29729", g.wave.frames())
	}
}

func TestOnlyTheLatestQueuedReadRuns(t *testing.T) {
	g, sc := withSlowCard(t)
	g.banks.SetSelected("J")
	test.Tap(g.tiles[4]) // J5: in flight
	test.Tap(g.tiles[0]) // J1: queued (no file on the fake card)
	test.Tap(g.tiles[1]) // J2: replaces J1 in the queue
	test.Tap(g.tiles[4]) // back to J5
	sc.drain()
	if sc.reads[sp404sx.PadIndex('J', 1)] != 0 {
		t.Error("a read that was overtaken still ran")
	}
}

func TestPlayReusesLoadedSample(t *testing.T) {
	g, sc := withSlowCard(t)
	fp := g.player.(*fakePlayer)
	test.Tap(g.tiles[9])
	sc.drain()
	doubleTap(g.tiles[9])
	sc.drain()
	if sc.reads[sp404sx.PadIndex('A', 10)] != 1 {
		t.Errorf("sample read %d times", sc.reads[sp404sx.PadIndex('A', 10)])
	}
	if len(fp.pcm) != 26680*4 || g.playing != sp404sx.PadIndex('A', 10) {
		t.Errorf("not playing: %d bytes, playing=%d", len(fp.pcm), g.playing)
	}
}

func TestPlaybackStartsWhenSampleArrives(t *testing.T) {
	g, sc := withSlowCard(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9]) // A10 not loaded yet
	if !g.tiles[9].playing {
		t.Error("pad should light up at once")
	}
	sc.drain()
	if len(fp.pcm) != 26680*4 {
		t.Error("playback did not start when the sample arrived")
	}
}

func TestCancelledPlaybackDoesNotStartLate(t *testing.T) {
	g, sc := withSlowCard(t)
	fp := g.player.(*fakePlayer)
	doubleTap(g.tiles[9]) // A10, still loading
	test.Tap(g.tiles[0])  // user moves on: playback cancelled
	sc.drain()
	if fp.pcm != nil || g.playing >= 0 {
		t.Error("cancelled playback started when the sample arrived")
	}
}

// The read runs on its own goroutine and must not touch UI state: run with
// -race. The test goroutine plays the UI thread, running the completions
// one at a time like the real driver does (Fyne's test driver would run
// them on the worker goroutine instead).
func TestBackgroundReadTouchesNoUIState(t *testing.T) {
	g, _ := newTestGUI(t)
	ui := make(chan func(), 1)
	g.async = func(work, done func()) {
		go func() {
			work()
			ui <- done
		}()
	}
	test.Tap(g.tiles[9]) // A10: read starts in the background
	test.Tap(g.tiles[0]) // meanwhile the UI keeps working
	selectJ5(g)
	for g.inflight >= 0 {
		select {
		case done := <-ui:
			done()
		case <-time.After(5 * time.Second):
			t.Fatal("background read never completed")
		}
	}
	if g.wave.frames() != 29729 {
		t.Errorf("wave frames = %d, want J5", g.wave.frames())
	}
}
