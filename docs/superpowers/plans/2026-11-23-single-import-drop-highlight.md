# Single-file import and macOS drop highlight: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import exactly one file at a time. On macOS, light up the pad under the cursor while a file is dragged over the window.

**Architecture:** The picker and drop paths move from `[]string` to a single path, and the consecutive-pad logic goes away. A darwin-only cgo file adds `draggingUpdated:`, `draggingExited:` and `draggingEnded:` to GLFW's `GLFWContentView`. It calls back into Go, which maps the point to a pad with the same `padAt` function that `drop` uses, and sets a `dropTarget` flag on that tile.

**Tech Stack:** Go, Fyne v2.8.2, GLFW 3.4 (via Fyne), Objective-C/cgo on darwin.

**Spec:** `docs/superpowers/specs/2026-11-23-single-import-drop-highlight-design.md`

## Global Constraints

- Windows and Linux: no hover highlight; the drop works as before, for one file.
- Every import still writes to the card at once; the overwrite confirmation stays.
- Error text for several dropped files: `drop one file at a time`.
- The hint text reads `Select a pad, or drop a WAV/AIFF file onto a pad.`
- If `GLFWContentView` is missing, the hook does nothing (no crash, drop still works).

## Review Focus

- The drag leaves the window, or the user presses Esc: the highlight must clear (`draggingExited:` / `draggingEnded:` → `dragGone`).
- The drag is over the toolbar or the panel, not over a pad: no pad is highlighted (`dragOver` outside the tiles clears it).
- The bank changes while a highlight is shown (it cannot during an OS drag, but `refresh` must not leave a stale flag): `TestBankChangeClearsDropTarget`.
- The highlighted pad differs from the import target: `TestDragOverMatchesDropTarget` pins both to `padAt`.
- A drag with no card open: `dragOver` highlights nothing.

---

### Task 1: One file per import

**Files:**
- Modify: `cmd/sp404gui/picker.go` (`picker.openAudio`, `nativePicker.openAudio`, `fynePicker.openAudio`)
- Modify: `cmd/sp404gui/ui.go` (`drop`, `importInto`, `importDialog`, hint text)
- Modify: `cmd/sp404gui/model.go` (remove `dropTargets`)
- Modify tests: `picker_test.go`, `model_test.go` (remove `TestDropTargets`), `progress_test.go`, `panel_test.go:206`, `ui_test.go`

**Interfaces:**
- Produces: `openAudio(dir string, done func(path string, err error))`, where a cancelled dialog gives `""`, nil; `func (g *gui) importInto(pad int, path string)`; `func (g *gui) padAt(pos fyne.Position) (int, bool)`, the pad index of the tile under an absolute canvas position (false if none or no card).

- [ ] Step 1: Update the tests to the single-path API and add the new ones.
  - `TestImportSeveralFilesFromPicker` → `TestImportOneFileFromPicker`: `fp.openDone(fixture("s24.aif"), nil)` fills only A1. A2 is unchanged and the folder is remembered.
  - `TestDropSeveralFilesIsRejected`: drop two URIs on A1. The card's PAD_INFO is unchanged (`os.SameFile` on the file before and after, as in the existing tests), and `showError` receives `drop one file at a time`. Capture it by making `showError` replaceable only if needed; otherwise check the dialog via the window overlays.
  - `TestImportReportsProgress`: one file; the last fraction is 1 and the detail contains `1 of 1`.
- [ ] Step 2: `go test ./cmd/sp404gui`. Expect a build failure (signature mismatch).
- [ ] Step 3: Implement.
  - native: `zenity.SelectFile` (title `Import sample`);
  - `drop` rejects `len(paths) != 1`;
  - `importInto` with one pad and the overwrite check `!g.card.Info.Pads[pad].Empty()`;
  - extract `padAt` from `drop`;
  - delete `dropTargets`;
  - the progress detail stays `File 1 of 1 → A5: name`, so `batchProgress` is kept with one size.
- [ ] Step 4: `go vet ./... && go test -race ./...`. Expect PASS.
- [ ] Step 5: Commit `Import one file at a time, like Roland's converter`.

### Task 2: Drop highlight in the UI (platform-independent)

**Files:**
- Modify: `cmd/sp404gui/padtile.go`: a `dropTarget bool` field, and `set(t tile, selected, playing, dropTarget bool)`. A second vector `canvas.Rectangle` named `hover`: amber 3-point stroke at scale and fill `hwAmber` at alpha 0x33, hidden unless `dropTarget`.
- Modify: `cmd/sp404gui/ui.go`: `g.dropPad int` (-1 = none), `func (g *gui) dragOver(pos fyne.Position)`, `func (g *gui) dragGone()`. `refresh` passes `i+bank*12 == g.dropPad`. `drop` and the bank change call `dragGone`.
- Test: `cmd/sp404gui/drag_test.go`

**Interfaces:**
- Consumes: `padAt` (Task 1).
- Produces: `dragOver(pos fyne.Position)` and `dragGone()`, called on the UI thread.

- [ ] Step 1: Write the tests.
  - `TestDragOverMatchesDropTarget`: `dragOver(center of tile 4)` → only `g.tiles[4].dropTarget` is true; a `drop` at the same position imports into the pad `padAt` returned.
  - `TestDragOutsidePadsClearsHighlight`: `dragOver(fyne.NewPos(1, 1))` → no tile is flagged.
  - `TestDragGoneClearsHighlight`.
  - `TestBankChangeClearsDropTarget`.
  - `TestDragWithoutCardHighlightsNothing`.
- [ ] Step 2: Run them. Expect FAIL (undefined).
- [ ] Step 3: Implement as listed under Files.
- [ ] Step 4: `go test -race ./...` passes. Render a screenshot with a temporary test (`dragOver` over a full pad and over an empty pad) and check it with the user.
- [ ] Step 5: Commit `Highlight the pad under a dragged file`.

### Task 3: macOS drag-over hook

**Files:**
- Create: `cmd/sp404gui/drag_darwin.go` (cgo + Objective-C in the preamble, `-x objective-c`, `-framework Cocoa`)
- Create: `cmd/sp404gui/drag_other.go` (`//go:build !darwin`, no-op)
- Modify: `cmd/sp404gui/ui.go` (install the hook in `newGUI`)

**Interfaces:**
- Produces: `func installDragHook(moved func(x, y float64), gone func())`. x and y are in GLFW screen coordinates, the same space as GLFW cursor positions. The callbacks run on the AppKit main thread.
- The GUI converts with `fyne.NewPos(float32(x)/c.Scale(), float32(y)/c.Scale())`, where `c := g.win.Canvas()`, the same formula as Fyne's `scale.ToFyneCoordinate`. Both callbacks are wrapped in `fyne.Do`.

- [ ] Step 1: In C, `objc_getClass("GLFWContentView")`, returning if NULL. Then `class_addMethod` for:
  - `draggingUpdated:` (type `"Q@:@"`): it computes `pos = [sender draggingLocation]` and `y = [self frame].size.height - pos.y`, exactly as GLFW's `performDragOperation:` does, calls the exported Go `wav404DragMoved(x, y)`, and returns `NSDragOperationCopy`;
  - `draggingExited:` and `draggingEnded:` (`"v@:@"`), which call `wav404DragGone()`.
  Go keeps the two callbacks in package variables.
- [ ] Step 2: `go build ./cmd/sp404gui` on darwin, plus `GOOS=windows go vet ./cmd/sp404gui` and `GOOS=linux go vet ./cmd/sp404gui` to check the stub builds.
- [ ] Step 3: `go test -race ./...` passes (tests do not install the hook: the test driver has no GLFW).
- [ ] Step 4: Manual check by the user: `go run ./cmd/sp404gui`, drag a file from the Finder over the pads, outside the window, and back, then press Esc mid-drag.
- [ ] Step 5: Commit `macOS: report drag position to light up the target pad`.
