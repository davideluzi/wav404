package main

import (
	"bytes"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"

	"github.com/davideluzi/wav404/sp404sx"
)

// audioPlayer plays one preview at a time.
type audioPlayer interface {
	// play starts pcm (stereo, 16-bit LE, 44.1 kHz), replacing any current
	// playback. onEnd runs, on any goroutine, when it finishes by itself.
	play(pcm []byte, onEnd func()) error
	stop()
}

// otoPlayer plays through the system audio output. The output is opened on
// first use, so the app starts even on a machine without audio.
type otoPlayer struct {
	mu  sync.Mutex
	ctx *oto.Context
	cur *oto.Player
	gen int // bumped on every play/stop; stale watchers exit
}

func (o *otoPlayer) play(pcm []byte, onEnd func()) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ctx == nil {
		ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate: sp404sx.SampleRate, ChannelCount: 2, Format: oto.FormatSignedInt16LE,
		})
		if err != nil {
			return err
		}
		<-ready
		o.ctx = ctx
	}
	o.stopLocked()
	p := o.ctx.NewPlayer(bytes.NewReader(pcm))
	p.Play()
	o.cur = p
	gen := o.gen
	go o.watch(p, gen, onEnd)
	return nil
}

// watch polls until p finishes, then reports the end unless it was stopped.
func (o *otoPlayer) watch(p *oto.Player, gen int, onEnd func()) {
	for {
		time.Sleep(30 * time.Millisecond)
		o.mu.Lock()
		if o.gen != gen {
			o.mu.Unlock()
			return
		}
		if !p.IsPlaying() {
			p.Close()
			o.cur = nil
			o.mu.Unlock()
			onEnd()
			return
		}
		o.mu.Unlock()
	}
}

func (o *otoPlayer) stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stopLocked()
}

func (o *otoPlayer) stopLocked() {
	o.gen++
	if o.cur != nil {
		o.cur.Pause()
		o.cur.Close()
		o.cur = nil
	}
}
