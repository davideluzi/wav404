package main

import (
	"fyne.io/fyne/v2"

	"github.com/davideluzi/wav404/sp404sx"
)

// sampleLoader reads sample files off the UI thread. An SD card reads at a
// few MB/s, so a large sample takes seconds: reads go through one worker,
// the latest request first (requests overtaken while waiting are dropped),
// and finished samples are cached for the waveform and playback.
type sampleLoader struct {
	// replaceable in tests
	readSample func(c *sp404sx.Card, index int) (*sp404sx.Sample, error)
	async      func(work, done func()) // work off the UI thread, then done on it

	samples  map[int]*sp404sx.Sample
	order    []int // cached pads, oldest first
	bytes    int
	waiters  map[int][]func(*sp404sx.Sample, error)
	inflight int // pad being read, -1 = none
	queued   int // next pad to read, -1 = none
	gen      int // bumped when the cache is invalidated
}

const maxSampleBytes = 256 << 20

func (l *sampleLoader) initLoader() {
	l.readSample = func(c *sp404sx.Card, i int) (*sp404sx.Sample, error) { return c.ReadSample(i) }
	l.async = func(work, done func()) {
		go func() {
			work()
			fyne.Do(done)
		}()
	}
	l.samples, l.waiters = map[int]*sp404sx.Sample{}, map[int][]func(*sp404sx.Sample, error){}
	l.inflight, l.queued = -1, -1
}

func (l *sampleLoader) cachedSample(index int) (*sp404sx.Sample, bool) {
	s, ok := l.samples[index]
	return s, ok
}

// fetchSample calls cb on the UI thread with the sample of pad index, read
// from card c, unless the request is overtaken or the cache invalidated.
func (l *sampleLoader) fetchSample(c *sp404sx.Card, index int, cb func(*sp404sx.Sample, error)) {
	if s, ok := l.samples[index]; ok {
		cb(s, nil)
		return
	}
	l.waiters[index] = append(l.waiters[index], cb)
	switch {
	case l.inflight == index || l.queued == index:
	case l.inflight < 0:
		l.startRead(c, index)
	default:
		if l.queued >= 0 {
			delete(l.waiters, l.queued) // overtaken by a newer request
		}
		l.queued = index
	}
}

func (l *sampleLoader) startRead(c *sp404sx.Card, index int) {
	l.inflight = index
	gen, read := l.gen, l.readSample
	var s *sp404sx.Sample
	var err error
	l.async(func() { s, err = read(c, index) }, func() {
		l.inflight = -1
		if gen == l.gen {
			if err == nil {
				l.store(index, s)
			}
			cbs := l.waiters[index]
			delete(l.waiters, index)
			for _, cb := range cbs {
				cb(s, err)
			}
		}
		if q := l.queued; q >= 0 {
			l.queued = -1
			l.startRead(c, q)
		}
	})
}

func (l *sampleLoader) store(index int, s *sp404sx.Sample) {
	l.samples[index] = s
	l.order = append(l.order, index)
	l.bytes += 2 * len(s.PCM)
	for l.bytes > maxSampleBytes && len(l.order) > 1 {
		old := l.order[0]
		l.order = l.order[1:]
		l.bytes -= 2 * len(l.samples[old].PCM)
		delete(l.samples, old)
	}
}

// invalidateSamples forgets cached and pending samples: the files changed
// (import, clear) or another card was opened.
func (l *sampleLoader) invalidateSamples() {
	l.gen++
	clear(l.samples)
	clear(l.waiters)
	l.order, l.bytes, l.queued = nil, 0, -1
}
