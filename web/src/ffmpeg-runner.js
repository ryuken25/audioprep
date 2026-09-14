// Thin wrapper around @ffmpeg/ffmpeg: loading the self-hosted core, serialized exec
// with per-command log capture, progress normalization, and cancel (terminate + reload).

import { FFmpeg } from '@ffmpeg/ffmpeg';
import { fetchToBlobURL } from './fetch-blob.js';
import { planThreads } from './threads.js';

// Injected by vite.config.js from the installed cores; 0 when unknown.
const CORE_WASM_BYTES = typeof __CORE_WASM_BYTES__ === 'number' ? __CORE_WASM_BYTES__ : 0;
const CORE_MT_WASM_BYTES = typeof __CORE_MT_WASM_BYTES__ === 'number' ? __CORE_MT_WASM_BYTES__ : 0;

const TERMINATED_MSG = 'called FFmpeg.terminate()';

export class FFmpegRunner {
  /**
   * @param {{onLog?:Function, onCommand?:Function, onStatus?:Function}} hooks
   *  onLog(message, type)          every ffmpeg log line
   *  onCommand(args)               before each exec
   *  onStatus({phase, ...})        loading / downloading / ready / error
   */
  constructor(hooks = {}) {
    this.hooks = hooks;
    this.ffmpeg = null;
    this.ready = false;
    this.busy = false;
    this.coreBytes = 0;
    this._execLog = [];
    this._current = null;
    this._readyPromise = null;
    // 'auto' | 'multi' | 'single'. Changing it reloads the core.
    this.threadMode = 'auto';
    this.plan = planThreads('auto');
  }

  /**
   * Switch between the single- and multi-threaded cores. Reloads the core, so
   * only call it while idle. Returns the plan actually in force.
   */
  async setThreadMode(mode) {
    if (mode === this.threadMode) return this.plan;
    this.threadMode = mode;
    if (this.busy) return this.plan; // applied on the next load
    await this.load();
    return this.plan;
  }

  /** Resolves when the core is loaded (re-created after a cancel). */
  whenReady() {
    return this._readyPromise || this.load();
  }

  load() {
    this._readyPromise = this._load();
    return this._readyPromise;
  }

  async _load() {
    this.ready = false;
    const ff = new FFmpeg();
    ff.on('log', ({ type, message }) => {
      this._execLog.push(message);
      this.hooks.onLog?.(message, type);
    });
    ff.on('progress', ({ progress, time }) => {
      const ctx = this._current;
      if (!ctx || !ctx.onProgress) return;
      // `time` is microseconds of output written. Using it against the known input
      // duration is far more reliable than `progress`, which can be NaN or > 1.
      let ratio = progress;
      if (ctx.duration > 0 && Number.isFinite(time)) ratio = time / 1e6 / ctx.duration;
      ctx.onProgress(Number.isFinite(ratio) ? Math.min(1, Math.max(0, ratio)) : 0);
    });

    const base = import.meta.env.BASE_URL;
    const plan = planThreads(this.threadMode);
    this.plan = plan;
    // The MT core lives in a subfolder because it ships a third file
    // (ffmpeg-core.worker.js) that must sit next to its core js.
    const dir = plan.multi ? `${base}ffmpeg/mt/` : `${base}ffmpeg/`;
    this.hooks.onStatus?.({ phase: 'loading', received: 0, total: 0, plan });
    try {
      // Own downloader instead of @ffmpeg/util's toBlobURL: that one fails on
      // gzipped responses (see fetch-blob.js), which is what GitHub Pages sends.
      const wasm = await fetchToBlobURL(`${dir}ffmpeg-core.wasm`, 'application/wasm', {
        totalHint: plan.multi ? CORE_MT_WASM_BYTES : CORE_WASM_BYTES,
        onProgress: ({ received, total }) => this.hooks.onStatus?.({ phase: 'downloading', received, total, plan }),
      });
      this.coreBytes = wasm.bytes;

      const opts = { wasmURL: wasm.url };
      if (plan.multi) {
        // The multi-threaded core spawns its pthread workers by resolving
        // ffmpeg-core.worker.js *relative to its own script URL*. A blob: URL
        // has no directory, so that resolution yields undefined and the first
        // exec dies with "Cannot read properties of undefined". Hand it the
        // real paths instead; the browser handles any gzip transparently, and
        // only the wasm (the big download) still streams through our own
        // fetcher for the progress bar.
        opts.coreURL = `${dir}ffmpeg-core.js`;
        opts.workerURL = `${dir}ffmpeg-core.worker.js`;
      } else {
        // Single-threaded: keep the blob, which is what dodges the broken
        // gzip handling in @ffmpeg/util (see fetch-blob.js).
        const core = await fetchToBlobURL(`${dir}ffmpeg-core.js`, 'text/javascript');
        opts.coreURL = core.url;
      }
      await ff.load(opts);
    } catch (err) {
      // A multi-threaded load can fail on a browser that reports isolation but
      // refuses the worker. Fall back rather than leaving the app dead.
      if (plan.multi) {
        this.hooks.onLog?.(`multi-threaded core failed to load (${err?.message || err}); falling back to single-threaded`, 'warn');
        this.threadMode = 'single';
        return this._load();
      }
      this.hooks.onStatus?.({ phase: 'error', error: err });
      throw err;
    }
    this.ffmpeg = ff;
    this.ready = true;
    this.hooks.onStatus?.({ phase: 'ready', bytes: this.coreBytes, plan: this.plan });
    return true;
  }

  /**
   * Run one ffmpeg command. Resolves `{code, log, ms}`; rejects if the worker is terminated.
   * @param {string[]} args
   * @param {{duration?:number|null, onProgress?:(ratio:number)=>void}} opts
   */
  async exec(args, opts = {}) {
    if (!this.ready) throw new Error('FFmpeg is not loaded yet.');
    if (this.busy) throw new Error('FFmpeg is busy with another command.');
    this.busy = true;
    this._execLog = [];
    this._current = { duration: opts.duration || 0, onProgress: opts.onProgress || null };
    this.hooks.onCommand?.(args);
    const started = performance.now();
    try {
      const code = await this.ffmpeg.exec(args);
      return { code, log: this._execLog.join('\n'), ms: performance.now() - started };
    } finally {
      this.busy = false;
      this._current = null;
    }
  }

  writeFile(name, data) { return this.ffmpeg.writeFile(name, data); }
  readFile(name) { return this.ffmpeg.readFile(name); }
  deleteFile(name) { return this.ffmpeg.deleteFile(name); }

  /** Kill the worker (rejects any in-flight exec) and load a fresh instance. */
  async cancel() {
    const ff = this.ffmpeg;
    this.ready = false;
    this.ffmpeg = null;
    try { ff?.terminate(); } catch { /* already dead */ }
    return this.load();
  }

  static isTerminated(err) {
    return !!err && String(err.message || err).includes(TERMINATED_MSG);
  }
}
