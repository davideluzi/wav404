#!/bin/sh
# Regenerates sp404sx/testdata/import/* from the synthetic J0000005.WAV
# with macOS afconvert. Run from the repository root after genfixtures.
set -eu
src=sp404sx/testdata/J0000005.WAV
out=sp404sx/testdata/import
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

afconvert -f WAVE -d LEI16 "$src" "$tmp/src16.wav"
mkdir -p "$out"
afconvert -f WAVE -d LEI16          "$tmp/src16.wav" "$out/s16.wav"
afconvert -f WAVE -d LEI24          "$tmp/src16.wav" "$out/s24.wav"
afconvert -f WAVE -d LEI32          "$tmp/src16.wav" "$out/s32.wav"
afconvert -f WAVE -d LEF32          "$tmp/src16.wav" "$out/f32.wav"
afconvert -f WAVE -d UI8            "$tmp/src16.wav" "$out/u8.wav"
afconvert -f AIFF -d BEI16          "$tmp/src16.wav" "$out/s16.aif"
afconvert -f AIFF -d BEI24          "$tmp/src16.wav" "$out/s24.aif"
afconvert -f AIFC -d BEF32          "$tmp/src16.wav" "$out/fl32.aifc"
afconvert -f WAVE -d LEI16@48000    "$tmp/src16.wav" "$out/s16_48k.wav"
afconvert -f WAVE -d LEI16@22050    "$tmp/src16.wav" "$out/s16_22k.wav"
afconvert -f WAVE -d LEI24@96000    "$tmp/src16.wav" "$out/s24_96k.wav"
afconvert -f WAVE -d LEI16 -c 1     "$tmp/src16.wav" "$out/mono.wav"
