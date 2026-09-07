import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fitDimensions } from '../src/scale.js';

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
