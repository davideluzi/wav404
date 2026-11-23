package main

// The drag-over hook for macOS. GLFW's content view only implements
// draggingEntered: and performDragOperation:, so the app learns where a
// file is only when it is dropped. drag_darwin.m adds draggingUpdated:,
// draggingExited: and draggingEnded: to that view class, which report here.

/*
#cgo LDFLAGS: -framework Cocoa
void wav404InstallDragHook(void);
*/
import "C"

var dragMovedFn func(x, y float64)
var dragGoneFn func()

// installDragHook makes moved run while a file is dragged over the window
// (x, y in GLFW screen coordinates) and gone run when the drag leaves the
// window or ends. Both run on the AppKit main thread. If GLFW's view class
// cannot be found nothing is installed, and drop still works.
func installDragHook(moved func(x, y float64), gone func()) {
	dragMovedFn, dragGoneFn = moved, gone
	C.wav404InstallDragHook()
}

//export wav404DragMoved
func wav404DragMoved(x, y C.double) {
	if dragMovedFn != nil {
		dragMovedFn(float64(x), float64(y))
	}
}

//export wav404DragGone
func wav404DragGone() {
	if dragGoneFn != nil {
		dragGoneFn()
	}
}
