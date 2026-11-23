package main

// progressFunc tells the progress dialog how far a task is (0–1) and what
// it is doing now.
type progressFunc func(fraction float64, detail string)

// batchProgress turns per-file write progress into a fraction of a whole
// batch. Each file weighs in proportion to its size on disk, a cheap proxy
// for the bytes written to the card; if no size is known, files weigh the
// same.
type batchProgress struct {
	start []float64 // share of the batch before file i
	share []float64 // share of file i
}

func newBatchProgress(sizes []int64) *batchProgress {
	var sum int64
	for _, s := range sizes {
		sum += max(s, 0)
	}
	b := &batchProgress{start: make([]float64, len(sizes)), share: make([]float64, len(sizes))}
	acc := 0.0
	for i, s := range sizes {
		if sum > 0 {
			b.share[i] = float64(max(s, 0)) / float64(sum)
		} else {
			b.share[i] = 1 / float64(len(sizes))
		}
		b.start[i] = acc
		acc += b.share[i]
	}
	return b
}

// fraction is the batch progress while file i has written done of total bytes.
func (b *batchProgress) fraction(i int, done, total int64) float64 {
	f := b.start[i]
	if total > 0 {
		f += b.share[i] * float64(done) / float64(total)
	}
	return min(f, 1)
}
