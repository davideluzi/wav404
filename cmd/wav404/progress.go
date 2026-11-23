package main

// progressFunc tells the progress dialog how far a task is (0–1) and what
// it is doing now.
type progressFunc func(fraction float64, detail string)
