// How many threads ffmpeg.wasm may use, and whether the multi-threaded core can
// run at all.
//
// The MT core needs SharedArrayBuffer, which a browser only grants to a
// cross-origin-isolated page (COOP + COEP). Vercel sets those headers; on a host
// that cannot, public/coi-serviceworker.js adds them. When neither works we stay
// on the single-threaded core rather than failing.

/** Modes the user can pick. 'auto' is the default and does the right thing. */
export const THREAD_MODES = ['auto', 'multi', 'single'];

/** True when this page may use SharedArrayBuffer. */
export function isIsolated() {
  return typeof globalThis.crossOriginIsolated === 'boolean'
    ? globalThis.crossOriginIsolated
    : typeof SharedArrayBuffer !== 'undefined';
}

/** Logical cores the browser admits to, clamped to something sane. */
export function hardwareThreads() {
  const n = Number(globalThis.navigator?.hardwareConcurrency);
  return Number.isFinite(n) && n >= 1 ? Math.floor(n) : 4;
}

/**
 * Threads to actually hand libx264.
 *
 * Hard cap of 4, and it is not a performance guess: @ffmpeg/core-mt deadlocks
 * above it. Measured on a 16-core machine against a 3 s 720p60 encode:
 *
 *   -threads 2  2691 ms
 *   -threads 4  1909 ms
 *   -threads 6  never returns (pthread pool exhausted, no error, no frames)
 *
 * ffmpeg needs workers of its own for demux and filtering, so the encoder
 * asking for 6 leaves none and everyone waits forever. Four is the fastest
 * count that completes, and it is already ~2.5x the single-threaded core.
 */
export const MAX_THREADS = 4;

/** Plan the core and thread count for a mode. Pure, so it is unit-tested. */
export function planThreads(mode = 'auto', { isolated = isIsolated(), cores = hardwareThreads() } = {}) {
  if (mode === 'single' || !isolated) {
    return { multi: false, threads: 1, isolated, cores, reason: mode === 'single' ? 'chosen' : 'not-isolated' };
  }
  if (cores < 2) {
    return { multi: false, threads: 1, isolated, cores, reason: 'one-core' };
  }
  const threads = Math.max(2, Math.min(MAX_THREADS, cores - 1));
  return { multi: true, threads, isolated, cores, reason: mode === 'multi' ? 'chosen' : 'auto' };
}

/**
 * A short line for the footer, e.g.
 *   "multi-threaded wasm, 8 of 16 cores" / "single-threaded wasm (not isolated)"
 */
export function describePlan(plan) {
  if (plan.multi) return `multi-threaded wasm, ${plan.threads} of ${plan.cores} cores`;
  if (plan.reason === 'not-isolated') return 'single-threaded wasm (not cross-origin isolated)';
  if (plan.reason === 'one-core') return 'single-threaded wasm (one core)';
  return 'single-threaded wasm';
}
