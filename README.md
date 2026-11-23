# wav404

Manage the samples on a **Roland SP-404SX** SD card from your computer: import
WAV/AIFF files into pads, listen to them, change pad settings, export them back
as plain WAV files.

wav404 is an independent, open replacement for Roland's discontinued
*SP-404SX Wave Converter*, which only runs on Intel Macs (through Rosetta) and
old Windows versions. It runs natively on macOS, Windows and Linux, as a desktop
app and as a command-line tool.

> **Not affiliated with Roland.** Roland and SP-404SX are trademarks of Roland
> Corporation. The card format was reverse engineered for interoperability; see
> [docs/SP404SX_FORMAT.md](docs/SP404SX_FORMAT.md).

## Features

- **Import** any uncompressed WAV or AIFF/AIFC file: 8/16/24/32-bit integer or
  32/64-bit float, mono or stereo, any sample rate. Files are converted to the
  SP-404SX format (44.1 kHz, 16-bit) with a high-quality resampler. Files that are
  already 16-bit / 44.1 kHz are copied bit for bit.
- **Original tempo** computed exactly like Roland's converter.
- **Listen** to pads as they would sound (trim range, reverse, volume).
- **Pad settings**: volume, loop, gate, reverse, lo-fi, tempo.
- **Trim** on a waveform: drag the start and end markers.
- **Export** a pad as a standard WAV, whole or limited to its trim range.
- **Drag & drop** a file onto a pad; on macOS the target pad lights up.
- **Safe writes**: every change goes to a temporary file that then replaces the
  original, so an interrupted write never leaves a corrupt file on the card.

## Download

Website: **https://davideluzi.github.io/wav404/**

Ready-made builds are attached to each
[release](https://github.com/davideluzi/wav404/releases):

- **macOS 13 Ventura or later**, Apple Silicon and Intel (universal app).
  Open the `.dmg` and drag *wav404* onto *Applications*. The app is not
  notarized: the first time macOS blocks it. Go to *System Settings → Privacy &
  Security* and click *Open Anyway* (on macOS 13–14 right-click the app and
  choose *Open* also works).
- **Windows 10 or later**, 64-bit.

Every push to `main` also builds them; download them from the run's
*Artifacts* on the [Actions](https://github.com/davideluzi/wav404/actions) page.

## Build from source

You need [Go](https://go.dev/dl/) 1.27 or later and a C compiler (the GUI uses
[Fyne](https://fyne.io), which needs cgo).

- **macOS**: Xcode Command Line Tools (`xcode-select --install`).
- **Windows**: a MinGW-w64 GCC, e.g. from [MSYS2](https://www.msys2.org/).
- **Linux** (Debian/Ubuntu):
  `sudo apt install gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev libasound2-dev`.
  For the system file dialogs, also install `zenity` (GNOME) or `kdialog` (KDE).
  Without them the app uses its own built-in dialogs.

```sh
go install github.com/davideluzi/wav404/cmd/wav404@latest   # desktop app
go install github.com/davideluzi/wav404/cmd/sp404@latest      # command line
```

Or from a clone: `go run ./cmd/wav404`. To make the macOS app bundle, install
the packaging tool (`go install fyne.io/tools/cmd/fyne@latest`) and run
`fyne package --os darwin` in `cmd/wav404`. The CI workflow
(`.github/workflows/build.yml`) shows the full universal and Windows builds.

## Usage

**Back up your card first.** Copy the whole card to your computer before you make
changes. The app writes every change to the card immediately. You can also work on
the copy: open its folder with *Open folder…* and copy it back to the card when
you are done.

### Desktop app

Insert the card (it must have been formatted by the SP-404SX) and start
`wav404`. The card is found automatically. If it is not, use *Open folder…*
and pick the card's root folder (the one containing `ROLAND`).

- Pick a bank (A–J), click a pad to select it, **double-click** to listen.
  Space plays/stops the selected pad.
- *Import…* loads a file into the selected pad, or drop a file onto a pad
  (on macOS the pad under the cursor lights up). One file at a time, like
  Roland's converter. The app remembers the last folder you used.
- Change the settings in the panel under the pads. Drag the white markers on
  the waveform to set where the pad starts and stops playing.
- *Export…* saves the pad as a WAV file. Tick *Only the trimmed part* to export
  just the trim range.

### Command line

```sh
sp404 list   /Volumes/SP-404SX                 # assigned pads
sp404 import /Volumes/SP-404SX A1 kick.aif     # convert and load into pad A1
sp404 export /Volumes/SP-404SX J5 out.wav      # add -trim for the trim range only
sp404 clear  /Volumes/SP-404SX J5              # delete the sample on J5
```

## Limitations

- Lo-fi and looping are not rendered when you listen in the app.
- Patterns (`PTN/`) are not handled.
- Samples stored on the card as `.AIF` (byte 21 = 0) are not read. This
  converter writes only `.WAV`.
- The meaning of the *tempo mode* byte is unknown. Its value is kept as it is.

## Development

```sh
go test ./...
```

- `sp404sx/` is the core library: the card format, decoding, resampling,
  import/export and card operations. `cmd/sp404` and `cmd/wav404` are thin
  front ends.
- The test fixtures in `sp404sx/testdata` are synthetic. Regenerate them with
  `go run ./tools/genfixtures`, then `tools/genfixtures/import-fixtures.sh`
  (macOS, uses `afconvert`).
- Optional checks against real hardware:
  - `SP404_CARD=/Volumes/SP-404SX go test ./sp404sx -run RealCard`
    reads (never writes) every sample on a real card and checks it decodes and
    re-encodes byte for byte.
  - `WAV404_AUDIO_TEST=1 go test ./cmd/wav404 -run Oto` plays 0.2 s of
    silence on the real audio output.

## License

[MIT](LICENSE)
