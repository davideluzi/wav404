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
- **Export** a pad as a standard WAV, whole or limited to its trim range.
- **Drag & drop** files onto pads; dropping several files fills consecutive pads.
- **Safe writes**: every change goes to a temporary file that then replaces the
  original, so an interrupted write never leaves a corrupt file on the card.

## Install

You need [Go](https://go.dev/dl/) 1.27 or later and a C compiler (the GUI uses
[Fyne](https://fyne.io), which needs cgo).

- **macOS**: Xcode Command Line Tools (`xcode-select --install`).
- **Windows**: a MinGW-w64 GCC, e.g. from [MSYS2](https://www.msys2.org/).
- **Linux** (Debian/Ubuntu):
  `sudo apt install gcc libgl1-mesa-dev xorg-dev libasound2-dev`

```sh
go install github.com/davideluzi/wav404/cmd/sp404gui@latest   # desktop app
go install github.com/davideluzi/wav404/cmd/sp404@latest      # command line
```

Or from a clone: `go run ./cmd/sp404gui`.

## Usage

**Back up your card first.** Copy the whole card to your computer before you make
changes. The app writes every change to the card immediately. You can also work on
the copy: open its folder with *Open folder…* and copy it back to the card when
you are done.

### Desktop app

Insert the card (it must have been formatted by the SP-404SX) and start
`sp404gui`. The card is found automatically. If it is not, use *Open folder…*
and pick the card's root folder (the one containing `ROLAND`).

- Pick a bank (A–J), click a pad to select it, **double-click** to listen.
  Space plays/stops the selected pad.
- *Import…* or drop files onto a pad to load samples.
- Change the settings in the panel under the pads.
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

- Trim points can't be edited yet (they are kept, exported and previewed).
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
  import/export and card operations. `cmd/sp404` and `cmd/sp404gui` are thin
  front ends.
- The test fixtures in `sp404sx/testdata` are synthetic. Regenerate them with
  `go run ./tools/genfixtures`, then `tools/genfixtures/import-fixtures.sh`
  (macOS, uses `afconvert`).
- Optional checks against real hardware:
  - `SP404_CARD=/Volumes/SP-404SX go test ./sp404sx -run RealCard`
    reads (never writes) every sample on a real card and checks it decodes and
    re-encodes byte for byte.
  - `WAV404_AUDIO_TEST=1 go test ./cmd/sp404gui -run Oto` plays 0.2 s of
    silence on the real audio output.
