# Single-file import and drop highlight (macOS)

Date: 2026-11-23. Status: awaiting approval.

## Why

When preparing a card, the user decides where each sample goes. Filling
consecutive pads from a multi-file import gets in the way of that. Roland's
converter imported one file at a time: `CFileChooser::AskImportFile` calls
`setAllowsMultipleSelection: NO`. When dragging a file onto the window,
the user wants to see which pad it will land on before releasing.

## Goals

- Import accepts exactly one file, from the dialog or by drag and drop.
- On macOS, the pad under the cursor lights up while a file is dragged
  over the window, and the highlight goes away when the drag leaves or ends.

## Non-goals

- No change to Windows or Linux drag and drop: no hover highlight there,
  and they behave as they do today apart from the single-file rule.
- No change to the save model: each import still writes to the card at once,
  as in Roland's converter. The settings also still save at once.
- No moving or copying samples between pads (possible later).

## Anti-goals

- A wrong highlight is worse than none: the highlighted pad must always be
  the one that `drop` imports into.
- If the native hook stops working (for example GLFW renames its view class),
  the app must still run and drop must still work, just without a highlight.

## Approaches considered

- **A. Native drag-over hook on macOS (chosen by the user).** Through cgo,
  add `draggingUpdated:` and `draggingExited:` to GLFW's `GLFWContentView`
  class, which only implements `draggingEntered:` and `performDragOperation:`.
  The hook reports the position to Go, and the pad under it is highlighted.
  This is the real hover feedback the user asked for. It depends on a GLFW
  internal class name.
- B. Confirm after drop ("Import kick.wav into A5?"), on every platform.
  It is robust but costs a click per import, and gives no feedback during the drag.
- C. A on macOS plus B elsewhere. The user chose to leave Windows and Linux as they are.

Fyne 2.8 only exposes `Window.SetOnDropped`, and GLFW sends no cursor
events during an OS drag, so a hover highlight cannot be done in pure Go.

## Design

**Single file**
- `picker.openAudio` lets the user select one file only, in both the native (zenity)
  and the Fyne picker.
- `drop` with more than one file shows "Drop one file at a time" and imports nothing.
- `importInto` takes one path. Consecutive-pad logic (`dropTargets` filling
  n pads) is removed. The overwrite confirmation stays.
- The progress dialog stays and shows one file.
- The hint text changes from "drop WAV/AIFF files" to "drop a WAV/AIFF file".

**Drop highlight (macOS)**
- A darwin-only cgo file uses `class_addMethod` to add `draggingUpdated:`
  (it reports the position in view coordinates, flipped like GLFW does, and
  returns `NSDragOperationCopy`) and `draggingExited:` / `draggingEnded:`
  (they report "gone"). If the class is missing, it does nothing.
- On the Go side the position is converted to canvas coordinates the same way
  Fyne converts mouse positions, then handed to the UI thread with `fyne.Do`.
- The pad lookup is shared with `drop`, so the highlight and the import
  target are always the same pad.
- The pad shows a distinct drop highlight (amber outline plus a light amber
  fill), separate from the selection outline. It clears on exit, end of drag,
  drop, and bank change.
- Other platforms build a no-op stub.

## Left to the builder

- The exact look of the highlight. A screenshot is checked with the user.
- Tests: the drop with several files is rejected, and the hover-to-pad
  mapping matches the drop target. The Objective-C hook itself is verified
  by hand on macOS.
