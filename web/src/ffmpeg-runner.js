// Thin wrapper around @ffmpeg/ffmpeg: loading the self-hosted core, serialized exec
// with per-command log capture, progress normalization, and cancel (terminate + reload).

import { FFmpeg } from '@ffmpeg/ffmpeg';
import { fetchToBlobURL } from './fetch-blob.js';

// Injected by vite.config.js from the installed @ffmpeg/core; 0 when unknown.
const CORE_WASM_BYTES = typeof __CORE_WASM_BYTES__ === 'number' ? __CORE_WASM_BYTES__ : 0;

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
    this.hooks.onStatus?.({ phase: 'loading', received: 0, total: 0 });
    try {
      // Own downloader instead of @ffmpeg/util's toBlobURL: that one fails on
      // gzipped responses (see fetch-blob.js), which is what GitHub Pages sends.
      const core = await fetchToBlobURL(`${base}ffmpeg/ffmpeg-core.js`, 'text/javascript');
      const wasm = await fetchToBlobURL(`${base}ffmpeg/ffmpeg-core.wasm`, 'application/wasm', {
        totalHint: CORE_WASM_BYTES,
        onProgress: ({ received, total }) => this.hooks.onStatus?.({ phase: 'downloading', received, total }),
      });
      this.coreBytes = wasm.bytes;
      await ff.load({ coreURL: core.url, wasmURL: wasm.url });
    } catch (err) {
      this.hooks.onStatus?.({ phase: 'error', error: err });
      throw err;
    }
    this.ffmpeg = ff;
    this.ready = true;
    this.hooks.onStatus?.({ phase: 'ready', bytes: this.coreBytes });
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
