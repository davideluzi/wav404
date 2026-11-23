//go:build !darwin

package main

// installDragHook does nothing outside macOS: there is no drag-over
// highlight, and drop works as before.
func installDragHook(moved func(x, y float64), gone func()) {}
