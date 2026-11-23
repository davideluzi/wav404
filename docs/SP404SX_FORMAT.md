# SP-404SX SD card format (reverse engineered)

Source: what we observed on a real card (102 samples) and in the strings and symbols of Roland's
"SP-404SX Wave Converter" 1.01 (x86_64, symbols not stripped).

## Layout

```
/ROLAND/SP-404SX/SMPL/A0000001.WAV … J0000012.WAV   samples (bank A–J × pad 1–12 = 120)
/ROLAND/SP-404SX/SMPL/PAD_INFO.BIN                   pad parameters (120 × 32 byte)
/ROLAND/SP-404SX/SMPL/STPINFO.BIN                    124 byte, almost all zero; the converter does not use it
/ROLAND/SP-404SX/PTN/PTN?????.BIN                    patterns (not handled)
/ROLAND/IMPORT/                                      device import folder
/BKUP/
```

Pad index (0–119) = bank*12 + (pad-1). File name: `%c%07d.WAV` (bank letter, pad 1–12).

## Sample WAV

Little-endian RIFF. The header is always 512 bytes:

| offset | content |
|---|---|
| 0   | `RIFF` u32 size `WAVE` |
| 12  | `fmt ` size=18: PCM(1), ch 1/2, 44100, byterate, blockalign, 16 bit, cbSize=0 |
| 38  | `RLND` size=458: `"roifspsx"`, u32 LE = 4, u32 LE = pad index (0–119), then zeros |
| 504 | `data` u32 size, PCM starts at **512** |

Only 44.1 kHz / 16 bit. Mono or stereo (on the card: 99 stereo, 3 mono).

## PAD_INFO.BIN

120 records × 32 byte, **big-endian**:

| byte | field | notes |
|---|---|---|
| 0–3   | sample start | byte offset in the file, normally 512 |
| 4–7   | sample end | = file size |
| 8–11  | user start | trim; 512 if not set |
| 12–15 | user end | trim |
| 16 | volume | 0–127 |
| 17 | lofi | 0/1 |
| 18 | loop | 0/1 |
| 19 | gate | 0/1 |
| 20 | reverse | 0/1 |
| 21 | file format | **1 = `.WAV`, 0 = `.AIF`** (see below) |
| 22 | channels | 1/2 |
| 23 | tempo mode | 0–2; the converter writes 0 and only passes it through |
| 24–27 | original tempo | BPM × 10, 0–2000 |
| 28–31 | user tempo | BPM × 10, 400–2000 (40.0–200.0 BPM) |

Empty pad: 512/512/512/512, vol 127, lofi 0, loop 0, gate 1, rev 0, b21 1, ch 2, tm 0, 1200/1200.

The ranges come from `SP::CSampleParameter::ClassInvariant`. The converter calls the field mapping
"marshalling" (`CModelMarshaller::MarshallSp404sx` / `Unmarshall`).

### Byte offsets ↔ frames

Inside the converter, positions are kept in **frames** relative to the start of the PCM data.

- read: `frames = (bytes - 512) / 2 / channels`
- write: `bytes = ((frames * 2 * channels) &^ 3) + 512` (rounded down to a multiple of 4: watch out for mono with an odd frame count)

### Byte 21 = file format on the card

`CTargetDescriptorInstance<2>::GetSampleFilePathName` picks the extension from the internal field
(1 → `.AIF`, 2 → `.WAV`), which is written to the file as `b21 = (field != 1)`. So the SP-404SX also
accepts **AIFF** samples on the card (`A0000001.AIF`). `ConformSp404sxWaveFileConstraint` has an AIFF
branch that writes an `APPL` chunk instead of `RLND`. `SL::EWaveFileFormat`: 0 = AIFF, 1 = WAV.

In the Mac build, the format local in `CControllerSp404sx::Import` is **never initialized**. In
practice the result is always WAV: all 102 samples on the card are `.WAV` with b21=1. wav404 always
writes WAV + b21=1.

## Original tempo (`SP::CSampleParameter::ComputeBpm`)

On import, original tempo = user tempo = `ComputeBpm(lengthInFrames)`. The algorithm works in
**4/4 bars** (the constant 105 840 000 = 600 × 44100 × 4) and picks the number of bars whose BPM
is closest to **120.0**:

```
calcBPM(len, b):            // b = bars × 256; result = BPM × 10
    if len == 0 || b == 0: return 1020
    return (b * 826875 / 2) / len     // integer math; 826875 = 0xC9DFB = 105840000/128
                                       // (when b > 0x144A3747198D there's an overflow-guard branch; never reached)
computeBpm(len):
    if len == 0: return 1020
    x = 105840000 / len                // BPM×10 for 1 bar
    if x == 1200: return 1200
    if x < 1200:                       // long sample: increase the bar count
        b = 256; cur = x
        loop: prev = cur; b = (b == 256) ? 512 : b + 512   // 1, 2, 4, 6, 8, … bars
              cur = calcBPM(len, b)
              until cur > 1199 || b >= 0x96C801
        return |cur-1200| >= |prev-1200| ? prev : cur       // tie → prev
    else:                              // short sample: start from 1/128 bar
        cur = 826875 / len; b = 2
        if cur <= 1199:
            loop: prev = cur; b = (b >= 256) ? b + 256 : b * 2   // 1/128, 1/64, … 1, 2, 3, … bars
                  cur = calcBPM(len, b)
                  until cur > 1199 || b >= 0x96C801
        else prev = cur
        if cur > 2000: return prev
        return |cur-1200| < |prev-1200| ? cur : prev        // tie → prev
```

Checked against the card: 90/102 pads match exactly. The other 12 have tempo or trim changed on the
device (e.g. 4, 8, 24 exact beats, or user tempo ≠ original tempo).

## Import (`CControllerSp404sx::Import`)

1. `CanImport`: mono or stereo only (more channels → rejected). WAV (`.wav`) or AIFF (`.aif`/`.aiff`), uncompressed.
2. If the rate ≠ 44100, the expected length is `round(frames × 44100 / rate)`.
3. Minimum length: **0.069 s** (shorter → error 0xC). Free space on the card is checked (error 0x11).
4. `CSampleRateConverter::Convert` → `CFileSampleAttributeConverter(format, 16 bit, 44100.0,
   antialiasing algorithm 0, 0xE000, 0x1000)`: FIR resampling (`CFirFilterProcessorPrimitive`),
   then reduction to 16 bit. **No dither** (no rand/dither symbols in the binary).
5. `ConformSp404sxWaveFileConstraint` rewrites the file: `fmt ` + `RLND`(`spsx` sub-chunk) + `data`, header 512.
6. Pad parameters: start 0, end = length, user start 0, user end = length, vol 127, lofi 0, loop 0,
   gate 1, rev 0, b21 per format, ch from the file, tempo mode 0, original = user = `ComputeBpm(length)`.

**Loop points (`smpl`, AIFF `MARK`/`INST`) are ignored**: the converter parses them (`SL::CLoop`) but
does not carry them into the pad (loop flag 0, user start/end = whole sample).

## Export

There is none for the SP-404SX: `CControllerSp404sx::Export` and `ComputeExportedWaveByteSize` are
stubs that `return 0`. (Export exists only for the SP-555.) In wav404, export is our own design:
strip `RLND`, write a clean WAV, optionally applying the user start/end trim.

## Open questions

- Meaning of the **tempo mode** (byte 23, values 0–2): the converter never interprets it, it only
  passes it through. It is a device-side setting. Always 0 on the card.
- `STPINFO.BIN`: not used by the converter.
