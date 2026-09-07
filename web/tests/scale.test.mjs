import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fitDimensions, coverBox, fitInsideBox } from '../src/scale.js';

test('landscape input fits the landscape box', () => {
  assert.deepEqual(fitDimensions(1920, 1080, 0, 1280, 720), { w: 1280, h: 720, scaled: true });
  assert.deepEqual(fitDimensions(3840, 2160, 0, 1920, 1080), { w: 1920, h: 1080, scaled: true });
});

test('portrait input swaps the box and fits', () => {
  assert.deepEqual(fitDimensions(1080, 1920, 0, 1280, 720), { w: 720, h: 1280, scaled: true });
  // Landscape input into the TikTok (portrait) box: the box is swapped to landscape.
  assert.deepEqual(fitDimensions(1920, 1080, 0, 1080, 1920), { w: 1920, h: 1080, scaled: false });
  assert.deepEqual(fitDimensions(3840, 2160, 0, 1080, 1920), { w: 1920, h: 1080, scaled: true });
});

test('rotation metadata swaps the input dimensions first', () => {
  assert.deepEqual(fitDimensions(1920, 1080, 90, 1280, 720), { w: 720, h: 1280, scaled: true });
  assert.deepEqual(fitDimensions(1920, 1080, -90, 1280, 720), { w: 720, h: 1280, scaled: true });
  assert.deepEqual(fitDimensions(1920, 1080, 270, 1280, 720), { w: 720, h: 1280, scaled: true });
  // 180 does not change orientation.
  assert.deepEqual(fitDimensions(1920, 1080, 180, 1280, 720), { w: 1280, h: 720, scaled: true });
  // Rotated portrait phone video that already fits: no scaling.
  assert.deepEqual(fitDimensions(1280, 720, 90, 1280, 720), { w: 720, h: 1280, scaled: false });
});

test('never upscales', () => {
  assert.deepEqual(fitDimensions(640, 360, 0, 1280, 720), { w: 640, h: 360, scaled: false });
  assert.deepEqual(fitDimensions(1280, 720, 0, 1280, 720), { w: 1280, h: 720, scaled: false });
  assert.deepEqual(fitDimensions(720, 1280, 0, 1080, 1920), { w: 720, h: 1280, scaled: false });
});

test('preserves aspect ratio for non-16:9 input', () => {
  assert.deepEqual(fitDimensions(1440, 1080, 0, 1280, 720), { w: 960, h: 720, scaled: true }); // 4:3
  assert.deepEqual(fitDimensions(1080, 1080, 0, 1280, 720), { w: 720, h: 720, scaled: true }); // square
  const ultra = fitDimensions(2560, 1080, 0, 1280, 720); // 21:9
  assert.equal(ultra.w, 1280);
  assert.equal(ultra.h, 540);
});

test('rounds odd dimensions down to even, minimum 2', () => {
  assert.deepEqual(fitDimensions(641, 361, 0, 1280, 720), { w: 640, h: 360, scaled: true });
  assert.deepEqual(fitDimensions(1281, 721, 0, 1280, 720), { w: 1278, h: 720, scaled: true });
  const tiny = fitDimensions(3, 1, 0, 1280, 720);
  assert.equal(tiny.w, 2);
  assert.equal(tiny.h, 2);
  const scaledOdd = fitDimensions(1919, 1079, 0, 1280, 720);
  assert.equal(scaledOdd.w % 2, 0);
  assert.equal(scaledOdd.h % 2, 0);
  assert.ok(scaledOdd.w <= 1280 && scaledOdd.h <= 720);
});

/* ---------- Cover tile maths ---------- */

test('coverBox is the preset box, always even, null when there is no box', () => {
  assert.deepEqual(coverBox(1280, 720), { w: 1280, h: 720 });
  assert.deepEqual(coverBox(1080, 1920), { w: 1080, h: 1920 }, 'portrait presets stay portrait');
  assert.deepEqual(coverBox(1921, 1081), { w: 1920, h: 1080 }, 'odd sizes round down to even');
  // audio-only has no video box, and neither does a knob typed out of range.
  assert.equal(coverBox(null, null), null);
  assert.equal(coverBox(undefined, undefined), null);
  assert.equal(coverBox(1, 720), null);
  assert.equal(coverBox(1280, 0), null);
});

test('the cover tile has the aspect ratio of the finished video', () => {
  const ar = (box) => box.w / box.h;
  assert.ok(Math.abs(ar(coverBox(1280, 720)) - 16 / 9) < 1e-9, 'x-audio is 16:9');
  assert.ok(Math.abs(ar(coverBox(1920, 1080)) - 16 / 9) < 1e-9, 'x-balanced is 16:9');
  assert.ok(Math.abs(ar(coverBox(1080, 1920)) - 9 / 16) < 1e-9, 'tiktok is 9:16');
  assert.ok(ar(coverBox(1080, 1920)) < 1, 'a portrait preset gives a portrait tile');
  assert.ok(ar(coverBox(1280, 720)) > 1, 'a landscape preset gives a landscape tile');
});

test('fitInsideBox mirrors force_original_aspect_ratio=decrease', () => {
  // A square picture in a landscape box is limited by the height.
  assert.deepEqual(fitInsideBox(3000, 3000, 1280, 720), { w: 720, h: 720 });
  // A wide picture in a landscape box is limited by the width.
  assert.deepEqual(fitInsideBox(4000, 1000, 1280, 720), { w: 1280, h: 320 });
  // A portrait picture in a portrait box.
  assert.deepEqual(fitInsideBox(2000, 3000, 1080, 1920), { w: 1080, h: 1620 });
  // Exact fits are left alone.
  assert.deepEqual(fitInsideBox(1280, 720, 1280, 720), { w: 1280, h: 720 });
  // Unlike fitDimensions, a small picture is scaled up to fill the box, as ffmpeg does.
  assert.deepEqual(fitInsideBox(320, 240, 1280, 720), { w: 960, h: 720 });
  // The aspect ratio survives.
  const fit = fitInsideBox(1000, 1500, 1280, 720);
  assert.ok(Math.abs(fit.w / fit.h - 1000 / 1500) < 0.01);
  assert.ok(fit.w <= 1280 && fit.h <= 720, 'never larger than the box');
});

test('fitInsideBox copes with junk input', () => {
  assert.deepEqual(fitInsideBox(0, 0, 1280, 720), { w: 720, h: 720 });
  assert.deepEqual(fitInsideBox(NaN, NaN, 1280, 720), { w: 720, h: 720 });
  const tiny = fitInsideBox(10000, 1, 1280, 720);
  assert.ok(tiny.w >= 1 && tiny.h >= 1, 'never collapses to zero');
});

test('coverBox turns the frame to match the picture', () => {
  // Portrait art must not sit in a landscape frame with huge black pillars.
  assert.deepEqual(coverBox(1280, 720, 900, 1400), { w: 720, h: 1280 }, 'portrait art flips a landscape box');
  assert.deepEqual(coverBox(1080, 1920, 1920, 1080), { w: 1920, h: 1080 }, 'landscape art flips a portrait box');
  assert.deepEqual(coverBox(1280, 720, 1920, 1080), { w: 1280, h: 720 }, 'matching orientation is left alone');
  assert.deepEqual(coverBox(1280, 720, 1000, 1000), { w: 1280, h: 720 }, 'square art keeps the preset orientation');
  assert.deepEqual(coverBox(1080, 1920, 1000, 1000), { w: 1080, h: 1920 }, 'square art keeps a portrait preset portrait');
  assert.deepEqual(coverBox(1280, 720), { w: 1280, h: 720 }, 'no picture: the preset box as written');
  assert.deepEqual(coverBox(1280, 720, 0, 0), { w: 1280, h: 720 }, 'unknown size: no flip');
});
