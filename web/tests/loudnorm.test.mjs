import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseLoudnormJSON, measurementsUsable, parseEbur128Summary } from '../src/loudnorm.js';

// Real pass-1 output (ffmpeg 5.1, print_format=json), including the preceding noise.
const PASS1 = `
size=N/A time=00:01:02.35 bitrate=N/A speed= 9.3x
video:0kB audio:10748kB subtitle:0kB other streams:0kB global headers:0kB muxing overhead: unknown
[Parsed_loudnorm_1 @ 0x5f2a80]
{
\t"input_i" : "-19.83",
\t"input_tp" : "-0.42",
\t"input_lra" : "9.70",
\t"input_thresh" : "-30.31",
\t"output_i" : "-14.35",
\t"output_tp" : "-1.00",
\t"output_lra" : "8.10",
\t"output_thresh" : "-24.73",
\t"normalization_type" : "dynamic",
\t"target_offset" : "0.35"
}
`;

test('parseLoudnormJSON extracts the measurement block as numbers', () => {
  const m = parseLoudnormJSON(PASS1);
  assert.ok(m);
  assert.equal(m.input_i, -19.83);
  assert.equal(m.input_tp, -0.42);
  assert.equal(m.input_lra, 9.7);
  assert.equal(m.input_thresh, -30.31);
  assert.equal(m.target_offset, 0.35);
  assert.equal(m.output_i, -14.35);
  assert.equal(m.normalization_type, 'dynamic');
  assert.equal(measurementsUsable(m), true);
});

test('parseLoudnormJSON takes the last block when several are present', () => {
  const twice = PASS1 + PASS1.replace('"input_i" : "-19.83"', '"input_i" : "-12.00"');
  assert.equal(parseLoudnormJSON(twice).input_i, -12);
});

test('parseLoudnormJSON handles -inf for silence and flags it unusable', () => {
  const silent = PASS1.replace('"-19.83"', '"-inf"').replace('"-30.31"', '"-inf"');
  const m = parseLoudnormJSON(silent);
  assert.equal(m.input_i, -Infinity);
  assert.equal(measurementsUsable(m), false);
});

test('parseLoudnormJSON returns null when nothing matches', () => {
  assert.equal(parseLoudnormJSON('frame= 1 fps=0.0 q=0.0 size=N/A'), null);
  assert.equal(parseLoudnormJSON(''), null);
});

const EBUR = `
[Parsed_ebur128_0 @ 0x5a3b20] Summary:

  Integrated loudness:
    I:         -14.1 LUFS
    Threshold: -24.4 LUFS

  Loudness range:
    LRA:         7.9 LU
    Threshold: -34.5 LUFS
    LRA low:    -19.6 LUFS
    LRA high:   -11.7 LUFS

  True peak:
    Peak:       -1.0 dBFS
`;

test('parseEbur128Summary reads I, LRA and true peak', () => {
  const s = parseEbur128Summary(EBUR);
  assert.deepEqual(s, { I: -14.1, threshold: -24.4, LRA: 7.9, peak: -1.0 });
});

test('parseEbur128Summary returns null without a summary block', () => {
  assert.equal(parseEbur128Summary('no summary here'), null);
});
