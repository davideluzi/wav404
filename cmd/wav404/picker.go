package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"github.com/ncruces/zenity"
)

// Preference keys for the folders the dialogs start in.
const (
	prefImportDir = "importDir"
	prefExportDir = "exportDir"
	prefCardDir   = "cardDir"
)

// picker shows file dialogs. done runs on the UI thread; a cancelled
// dialog gives an empty path and no error.
type picker interface {
	openAudio(dir string, done func(path string, err error))
	saveWAV(dir, name string, done func(path string, err error))
	openFolder(dir string, done func(path string, err error))
}

// choosePicker prefers the operating system's dialogs and falls back to
// Fyne's (Linux without zenity/kdialog installed).
func choosePicker(w fyne.Window, native bool) picker {
	if native {
		return nativePicker{}
	}
	return fynePicker{w}
}

// nativePicker uses the system dialogs (Finder, Explorer, GTK/KDE).
// They block, so they run off the UI thread.
type nativePicker struct{}

var audioFilter = zenity.FileFilter{
	Name: "Audio files", Patterns: []string{"*.wav", "*.aif", "*.aiff", "*.aifc"}, CaseFold: true,
}

// inDir makes zenity start in dir (a trailing separator means "folder").
func inDir(dir string) zenity.Option {
	if dir == "" {
		return zenity.Filename("")
	}
	return zenity.Filename(dir + string(os.PathSeparator))
}

// notCancelled turns a cancelled dialog into "no error".
func notCancelled(err error) error {
	if errors.Is(err, zenity.ErrCanceled) {
		return nil
	}
	return err
}

func (nativePicker) openAudio(dir string, done func(string, error)) {
	go func() {
		path, err := zenity.SelectFile(zenity.Title("Import sample"), inDir(dir), audioFilter)
		fyne.Do(func() { done(path, notCancelled(err)) })
	}()
}

func (nativePicker) saveWAV(dir, name string, done func(string, error)) {
	go func() {
		path, err := zenity.SelectFileSave(zenity.Title("Export pad"),
			zenity.Filename(filepath.Join(dir, name)), zenity.ConfirmOverwrite(),
			zenity.FileFilter{Name: "WAV files", Patterns: []string{"*.wav"}, CaseFold: true})
		if err == nil && filepath.Ext(path) == "" {
			path += ".wav"
		}
		fyne.Do(func() { done(path, notCancelled(err)) })
	}()
}

func (nativePicker) openFolder(dir string, done func(string, error)) {
	go func() {
		path, err := zenity.SelectFile(zenity.Title("Open SP-404SX card folder"), zenity.Directory(), inDir(dir))
		fyne.Do(func() { done(path, notCancelled(err)) })
	}()
}

// fynePicker uses Fyne's own dialogs.
type fynePicker struct{ win fyne.Window }

// startIn points a Fyne dialog at dir when it exists.
func startIn(d *dialog.FileDialog, dir string) {
	if dir == "" {
		return
	}
	if l, err := storage.ListerForURI(storage.NewFileURI(dir)); err == nil {
		d.SetLocation(l)
	}
}

func (f fynePicker) openAudio(dir string, done func(string, error)) {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			done("", err)
			return
		}
		r.Close()
		done(r.URI().Path(), nil)
	}, f.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".wav", ".WAV", ".aif", ".AIF", ".aiff", ".AIFF", ".aifc", ".AIFC"}))
	startIn(d, dir)
	d.Show()
}

func (f fynePicker) saveWAV(dir, name string, done func(string, error)) {
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			done("", err)
			return
		}
		w.Close()
		path := w.URI().Path()
		if !strings.EqualFold(filepath.Ext(path), ".wav") {
			path += ".wav"
		}
		done(path, nil)
	}, f.win)
	d.SetFileName(name)
	startIn(d, dir)
	d.Show()
}

func (f fynePicker) openFolder(dir string, done func(string, error)) {
	d := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) {
		if err != nil || u == nil {
			done("", err)
			return
		}
		done(u.Path(), nil)
	}, f.win)
	startIn(d, dir)
	d.Show()
}
