import test from 'node:test';
import assert from 'node:assert/strict';
import { planThreads, describePlan, THREAD_MODES, MAX_THREADS } from '../src/threads.js';
import { buildVideoCodecArgs } from '../src/pipeline.js';

const iso = (cores) => ({ isolated: true, cores });
const notIso = (cores) => ({ isolated: false, cores });

test('auto uses the multi-threaded core only when the page is isolated', () => {
  const on = planThreads('auto', iso(16));
  assert.equal(on.multi, true);
  // Not a performance guess: @ffmpeg/core-mt deadlocks above 4 (see threads.js).
  assert.equal(on.threads, 4, 'capped at 4 even on a 16-core machine');

  const off = planThreads('auto', notIso(16));
  assert.equal(off.multi, false);
  assert.equal(off.threads, 1);
  assert.equal(off.reason, 'not-isolated');
});

test('asking for multi on a host that cannot isolate still falls back', () => {
  // The MT core would throw at load; better to plan the single-threaded one.
  const p = planThreads('multi', notIso(8));
  assert.equal(p.multi, false);
  assert.equal(p.threads, 1);
});

test('single is honoured even when multi is available', () => {
  const p = planThreads('single', iso(16));
  assert.equal(p.multi, false);
  assert.equal(p.threads, 1);
  assert.equal(p.reason, 'chosen');
});

test('thread count leaves a core for the page and never exceeds the cap', () => {
  assert.equal(planThreads('auto', iso(4)).threads, 3);
  assert.equal(planThreads('auto', iso(5)).threads, 4);
  assert.equal(planThreads('auto', iso(8)).threads, 4);
  assert.equal(planThreads('auto', iso(32)).threads, 4, 'never above the deadlock ceiling');
  // Two cores is the smallest machine worth threading.
  assert.equal(planThreads('auto', iso(2)).threads, 2);
  const one = planThreads('auto', iso(1));
  assert.equal(one.multi, false, 'a single core cannot benefit');
  assert.equal(one.reason, 'one-core');
});

test('every mode is a valid choice and produces a plan', () => {
  for (const mode of THREAD_MODES) {
    const p = planThreads(mode, iso(8));
    assert.equal(typeof p.threads, 'number');
    assert.ok(p.threads >= 1);
    assert.equal(typeof describePlan(p), 'string');
    assert.ok(describePlan(p).length > 0);
  }
});

test('describePlan says why it is single-threaded', () => {
  assert.match(describePlan(planThreads('auto', iso(16))), /multi-threaded wasm, 4 of 16 cores/);
  assert.match(describePlan(planThreads('auto', notIso(16))), /not cross-origin isolated/);
  assert.match(describePlan(planThreads('auto', iso(1))), /one core/);
});

test('-threads reaches libx264 only when there is more than one', () => {
  const many = buildVideoCodecArgs({ crf: 23, maxrate: 8000, bufsize: 16000 }, { threads: 8 });
  const i = many.indexOf('-threads');
  assert.ok(i >= 0, 'the flag is present');
  assert.equal(many[i + 1], '8');

  const one = buildVideoCodecArgs({ crf: 23, maxrate: 8000, bufsize: 16000 }, { threads: 1 });
  assert.equal(one.includes('-threads'), false, 'single-threaded needs no flag');

  const none = buildVideoCodecArgs({ crf: 23, maxrate: 8000, bufsize: 16000 });
  assert.equal(none.includes('-threads'), false, 'defaults to no flag');
});

test('a still-picture encode is threaded too', () => {
  const args = buildVideoCodecArgs({ crf: 26, maxrate: 1800, bufsize: 3600 }, { still: true, threads: 4 });
  assert.ok(args.includes('-threads'));
  assert.ok(args.includes('-tune') && args.includes('stillimage'));
  assert.ok(args.includes('-r') && args.includes('1'));
});

test('the thread cap is the measured safe maximum, not a round number', () => {
  // 6 threads deadlocks @ffmpeg/core-mt: no error, no frames, never returns.
  // If anyone raises this, they need to re-run that measurement first.
  assert.equal(MAX_THREADS, 4);
  for (const cores of [2, 4, 8, 12, 16, 24, 64]) {
    assert.ok(planThreads('auto', iso(cores)).threads <= MAX_THREADS);
  }
});
