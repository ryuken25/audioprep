import './style.css';
import { fetchFile } from '@ffmpeg/util';
import { FFmpegRunner } from './ffmpeg-runner.js';
import { PRESETS, PRESET_ORDER, DEFAULT_PRESET, TUNABLE_KEYS, settingsFromPreset, matchesPreset } from './presets.js';
import { parseProbe } from './probe.js';
import { buildProbeArgs, buildVideoFilter, runPipeline, STAGES } from './pipeline.js';
import { argsToShell, baseName, extOf, formatBytes, formatDuration, formatNumber } from './format.js';

const ACCEPTED = ['mp4', 'mov', 'mkv', 'webm', 'avi', 'm4a', 'wav', 'mp3'];
const LS = { theme: 'kxc.theme', preset: 'kxc.preset', custom: 'kxc.custom' };
const VIDEO_KEYS = ['crf', 'maxrate', 'fpsCap', 'maxW', 'maxH', 'staticVideo', 'staticTime'];

const $ = (id) => document.getElementById(id);
const els = {
  themeToggle: $('theme-toggle'),
  dropzone: $('dropzone'),
  fileInput: $('file-input'),
  notice: $('notice'),
  fileInfo: $('file-info'),
  fileName: $('file-name'),
  fileGrid: $('file-info-grid'),
  presetList: $('preset-list'),
  presetPlan: $('preset-plan'),
  advanced: $('advanced'),
  processBtn: $('process-btn'),
  status: $('status'),
  progress: $('progress'),
  stageLabel: $('stage-label'),
  progressPct: $('progress-pct'),
  progressBar: $('progress-bar'),
  progressFill: $('progress-fill'),
  progressEta: $('progress-eta'),
  cancelBtn: $('cancel-btn'),
  result: $('result'),
  resultSummary: $('result-summary'),
  warnings: $('warnings'),
  compareBody: document.querySelector('#compare tbody'),
  downloadBtn: $('download-btn'),
  anotherBtn: $('another-btn'),
  logDetails: $('log-details'),
  logCount: $('log-count'),
  logClear: $('log-clear'),
  log: $('log'),
  coreDot: document.querySelector('#core-status .dot'),
  coreText: $('core-status-text'),
};

const state = {
  file: null,
  inputName: null,
  inputWritten: false,
  probe: null,
  settings: settingsFromPreset(DEFAULT_PRESET),
  running: false,
  cancelled: false,
  stageStarted: 0,
  result: null,
  downloadURL: null,
};

/* ---------- Log panel ---------- */

const log = {
  count: 0,
  max: 4000,
  pending: [],
  timer: 0,
  push(text, cls) {
    this.pending.push({ text, cls });
    // Throttled DOM writes. A timer (not rAF) so the log keeps draining in a background tab.
    if (!this.timer) this.timer = setTimeout(() => this.flush(), 80);
  },
  flush() {
    this.timer = 0;
    const frag = document.createDocumentFragment();
    for (const { text, cls } of this.pending) {
      if (cls) {
        const span = document.createElement('span');
        span.className = cls;
        span.textContent = text;
        frag.appendChild(span);
      } else {
        frag.appendChild(document.createTextNode(text));
      }
      frag.appendChild(document.createTextNode('\n'));
      this.count++;
    }
    this.pending = [];
    els.log.appendChild(frag);
    while (this.count > this.max && els.log.firstChild) {
      const node = els.log.firstChild;
      els.log.removeChild(node);
      if (node.nodeType === Node.TEXT_NODE && node.textContent === '\n') this.count--;
    }
    els.logCount.textContent = `(${this.count} lines)`;
    els.log.scrollTop = els.log.scrollHeight;
  },
  clear() {
    clearTimeout(this.timer);
    this.timer = 0;
    this.pending = [];
    this.count = 0;
    els.log.textContent = '';
    els.logCount.textContent = '';
  },
};

/* ---------- FFmpeg ---------- */

const runner = new FFmpegRunner({
  onLog: (message) => log.push(message),
  onCommand: (args) => log.push(`$ ${argsToShell(args)}`, 'cmd'),
  onStatus: (ev) => {
    if (ev.phase === 'loading') setCoreStatus('loading', 'FFmpeg core: loading…');
    else if (ev.phase === 'downloading') {
      const got = formatBytes(ev.received);
      setCoreStatus('loading', ev.total ? `FFmpeg core: loading… ${got} / ${formatBytes(ev.total)}` : `FFmpeg core: loading… ${got}`);
    } else if (ev.phase === 'ready') {
      setCoreStatus('ready', `FFmpeg core: ready (${formatBytes(ev.bytes)}, single-threaded wasm)`);
      // After a cancel the "Cancelled." line is more useful than "Ready to process."
      if (!state.running && !state.cancelled) setStatus(state.probe ? 'Ready to process.' : 'Drop a file to begin.');
    } else if (ev.phase === 'error') {
      setCoreStatus('error', `FFmpeg core failed to load: ${ev.error?.message || ev.error}`);
      setStatus('FFmpeg could not be loaded. Check the console and reload the page.', 'error');
    }
    updateProcessButton();
  },
});

function setCoreStatus(stateName, text) {
  els.coreDot.dataset.state = stateName;
  els.coreText.textContent = text;
}

/* ---------- Theme ---------- */

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  try { localStorage.setItem(LS.theme, theme); } catch { /* private mode */ }
}

els.themeToggle.addEventListener('click', () => {
  applyTheme(document.documentElement.dataset.theme === 'light' ? 'dark' : 'light');
});

/* ---------- Status helpers ---------- */

function setStatus(text, kind = '') {
  els.status.textContent = text;
  els.status.className = `status${kind ? ` is-${kind}` : ''}`;
}

function showNotice(text, kind = '') {
  els.notice.textContent = text;
  els.notice.className = `notice${kind ? ` is-${kind}` : ''}`;
  els.notice.hidden = !text;
}

function updateProcessButton() {
  els.processBtn.disabled = !(state.file && state.probe && runner.ready && !state.running);
}

function setBusy(busy) {
  state.running = busy;
  els.dropzone.classList.toggle('is-busy', busy);
  els.presetList.classList.toggle('is-disabled', busy);
  els.advanced.querySelectorAll('input').forEach((i) => { i.disabled = busy || i.dataset.forceDisabled === '1'; });
  els.cancelBtn.disabled = false;
  updateProcessButton();
}

/* ---------- Presets & settings ---------- */

function effectiveSettings() {
  const s = { ...state.settings };
  if (state.probe && !state.probe.video) s.audioOnly = true;
  s.bufsize = Number(s.maxrate) * 2;
  return s;
}

function renderPresets() {
  els.presetList.textContent = '';
  for (const id of PRESET_ORDER) {
    const p = PRESETS[id];
    const label = document.createElement('label');
    label.className = 'preset';
    const input = document.createElement('input');
    input.type = 'radio';
    input.name = 'preset';
    input.value = id;
    input.addEventListener('change', () => { if (input.checked) selectPreset(id); });
    const name = document.createElement('span');
    name.className = 'preset-name';
    name.textContent = p.name;
    const desc = document.createElement('span');
    desc.className = 'preset-desc';
    desc.textContent = p.description;
    label.append(input, name, desc);
    els.presetList.appendChild(label);
  }
}

function checkPresetRadio(id) {
  const radio = els.presetList.querySelector(`input[value="${id}"]`);
  if (radio) radio.checked = true;
}

function selectPreset(id) {
  const keep = { staticVideo: state.settings.staticVideo, staticTime: state.settings.staticTime };
  if (id === 'custom') {
    state.settings = { ...state.settings, preset: 'custom' };
  } else {
    state.settings = settingsFromPreset(id, keep);
  }
  checkPresetRadio(state.settings.preset);
  fillAdvanced();
  persistSettings();
  renderPlan();
}

function fillAdvanced() {
  const s = state.settings;
  const noVideo = !!(state.probe && !state.probe.video);
  const audioOnly = s.audioOnly || noVideo;
  for (const input of els.advanced.querySelectorAll('input[data-key]')) {
    const key = input.dataset.key;
    if (input.type === 'checkbox') input.checked = key === 'audioOnly' ? audioOnly : !!s[key];
    else if (input !== document.activeElement) input.value = s[key] ?? ''; // don't fight the user's typing
    // Video knobs are irrelevant when the output is audio only.
    const off = (VIDEO_KEYS.includes(key) && audioOnly) || (key === 'audioOnly' && noVideo) || (key === 'lowpassHz' && !s.lowpass) || (key === 'staticTime' && !s.staticVideo);
    input.dataset.forceDisabled = off ? '1' : '0';
    input.disabled = off || state.running;
  }
  els.advanced.querySelectorAll('[data-video]').forEach((g) => g.classList.toggle('is-off', audioOnly));
}

function onAdvancedInput(ev) {
  const input = ev.target;
  const key = input.dataset.key;
  if (!key) return;
  let value;
  if (input.type === 'checkbox') value = input.checked;
  else {
    value = Number(input.value);
    if (!Number.isFinite(value)) return;
  }
  state.settings = { ...state.settings, [key]: value };
  if (key === 'maxrate') state.settings.bufsize = value * 2;

  const affectsPreset = TUNABLE_KEYS.includes(key) || key === 'audioOnly';
  if (affectsPreset && state.settings.preset !== 'custom' && !matchesPreset(state.settings, state.settings.preset)) {
    state.settings.preset = 'custom';
    checkPresetRadio('custom');
  }
  fillAdvanced();
  persistSettings();
  renderPlan();
}

function persistSettings() {
  try {
    localStorage.setItem(LS.preset, state.settings.preset);
    if (state.settings.preset === 'custom') localStorage.setItem(LS.custom, JSON.stringify(state.settings));
  } catch { /* ignore */ }
}

function restoreSettings() {
  let id = DEFAULT_PRESET;
  try { id = localStorage.getItem(LS.preset) || DEFAULT_PRESET; } catch { /* ignore */ }
  if (!PRESETS[id]) id = DEFAULT_PRESET;
  if (id === 'custom') {
    let saved = null;
    try { saved = JSON.parse(localStorage.getItem(LS.custom) || 'null'); } catch { /* ignore */ }
    state.settings = { ...settingsFromPreset('custom'), ...(saved || {}), preset: 'custom' };
  } else {
    state.settings = settingsFromPreset(id);
  }
  checkPresetRadio(state.settings.preset);
  fillAdvanced();
  renderPlan();
}

/** One-line description of what the current settings will produce for the loaded file. */
function renderPlan() {
  const p = state.probe;
  if (!p) { els.presetPlan.textContent = ''; return; }
  const s = effectiveSettings();
  if (s.audioOnly) {
    els.presetPlan.textContent = `Output: .m4a, AAC ${s.audioBitrate} kb/s, 48 kHz, ${s.I} LUFS`;
    return;
  }
  const { dims } = buildVideoFilter(p, s);
  const fps = p.video.fps ? Math.min(p.video.fps, s.fpsCap || p.video.fps) : s.fpsCap;
  const mode = s.staticVideo ? 'static frame' : `${dims.w}x${dims.h} @ ${formatNumber(fps, fps % 1 ? 2 : 0)} fps`;
  els.presetPlan.textContent = `Output: ${mode}, H.264 CRF ${s.crf} ≤ ${s.maxrate} kb/s, AAC ${s.audioBitrate} kb/s, ${s.I} LUFS`;
}

/* ---------- File handling ---------- */

function openPicker() {
  if (state.running) return;
  els.fileInput.click();
}

els.dropzone.addEventListener('click', openPicker);
els.dropzone.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openPicker(); }
});
els.fileInput.addEventListener('change', () => {
  const f = els.fileInput.files?.[0];
  if (f) handleFile(f);
  els.fileInput.value = '';
});

['dragenter', 'dragover'].forEach((t) => els.dropzone.addEventListener(t, (e) => {
  e.preventDefault();
  if (state.running) return;
  e.dataTransfer.dropEffect = 'copy';
  els.dropzone.classList.add('is-over');
}));
['dragleave', 'drop'].forEach((t) => els.dropzone.addEventListener(t, (e) => {
  e.preventDefault();
  els.dropzone.classList.remove('is-over');
}));
els.dropzone.addEventListener('drop', (e) => {
  if (state.running) return;
  const f = e.dataTransfer?.files?.[0];
  if (f) handleFile(f);
});
// Dropping a file anywhere else on the page must not navigate away from the app.
window.addEventListener('dragover', (e) => e.preventDefault());
window.addEventListener('drop', (e) => e.preventDefault());

async function handleFile(file) {
  if (state.running) return;
  state.cancelled = false;
  const ext = extOf(file.name);
  if (!ACCEPTED.includes(ext)) {
    showNotice(`Unsupported file type ".${ext || '?'}". Use MP4, MOV, MKV, WebM, AVI, M4A, WAV or MP3.`, 'error');
    return;
  }
  showNotice('');
  resetResult();
  state.file = file;
  state.probe = null;
  updateProcessButton();
  renderFileInfoLoading(file);

  try {
    setStatus('Waiting for FFmpeg core…');
    await runner.whenReady();
    setBusy(true);
    showProgress(STAGES.READ, null);

    if (state.inputName && state.inputWritten) {
      await runner.deleteFile(state.inputName).catch(() => {});
      state.inputWritten = false;
    }
    state.inputName = `in.${ext}`;
    const data = await fetchFile(file);
    await runner.writeFile(state.inputName, data);
    state.inputWritten = true;

    showProgress(STAGES.PROBE, null);
    const r = await runner.exec(buildProbeArgs(state.inputName));
    const probe = parseProbe(r.log);
    if (!probe.audio) throw new Error('No audio stream found in this file.');
    state.probe = probe;

    if (!probe.video) {
      if (state.settings.preset !== 'audio-only') selectPreset('audio-only');
      showNotice('No video stream found. Switched to the Audio only preset.');
    }
    renderFileInfo(file, probe);
    fillAdvanced();
    renderPlan();
    setStatus('Ready to process.');
  } catch (err) {
    if (FFmpegRunner.isTerminated(err)) {
      state.cancelled = true;
      setStatus('Cancelled.');
    } else {
      console.error(err);
      log.push(`ERROR: ${err.message || err}`, 'err');
      setStatus(`Could not read this file: ${err.message || err}`, 'error');
    }
    log.flush();
    state.file = null;
    state.probe = null;
    els.fileInfo.hidden = true;
  } finally {
    setBusy(false);
    hideProgress();
    updateProcessButton();
  }
}

function renderFileInfoLoading(file) {
  els.fileInfo.hidden = false;
  els.fileName.textContent = file.name;
  els.fileGrid.textContent = '';
  addInfo('Size', formatBytes(file.size));
  addInfo('Status', 'Reading…');
}

function addInfo(label, value, badges = []) {
  const item = document.createElement('div');
  item.className = 'info-item';
  const dt = document.createElement('dt');
  dt.textContent = label;
  const dd = document.createElement('dd');
  dd.append(document.createTextNode(value));
  for (const b of badges) {
    const span = document.createElement('span');
    span.className = `badge${b.kind ? ` is-${b.kind}` : ''}`;
    span.textContent = b.text;
    dd.appendChild(span);
  }
  item.append(dt, dd);
  els.fileGrid.appendChild(item);
}

function describeVideo(v) {
  if (!v) return '–';
  const parts = [v.codec || '?'];
  if (v.profile) parts[0] += ` (${v.profile})`;
  if (v.pixFmt) parts.push(v.pixFmt);
  if (Number.isFinite(v.bitrate)) parts.push(`${Math.round(v.bitrate)} kb/s`);
  return parts.join(' · ');
}

function describeAudio(a) {
  if (!a) return '–';
  const parts = [a.codec || '?'];
  if (a.sampleRate) parts.push(`${(a.sampleRate / 1000).toFixed(a.sampleRate % 1000 ? 1 : 0)} kHz`);
  if (a.layout) parts.push(a.layout);
  else if (a.channels) parts.push(`${a.channels} ch`);
  if (Number.isFinite(a.bitrate)) parts.push(`${Math.round(a.bitrate)} kb/s`);
  return parts.join(' · ');
}

function renderFileInfo(file, probe) {
  els.fileInfo.hidden = false;
  els.fileName.textContent = file.name;
  els.fileGrid.textContent = '';
  const v = probe.video;
  const durBadges = [];
  const warnDur = PRESETS[state.settings.preset]?.warnDuration;
  if (warnDur && probe.duration > warnDur) durBadges.push({ text: `over ${warnDur} s`, kind: 'warn' });

  addInfo('Size', formatBytes(file.size));
  addInfo('Duration', formatDuration(probe.duration), durBadges);
  if (v) {
    const rotBadges = v.rotation ? [{ text: `rotated ${v.rotation}°`, kind: 'accent' }] : [];
    addInfo('Resolution', `${v.width}x${v.height}`, rotBadges);
    addInfo('Frame rate', v.fps ? `${formatNumber(v.fps, v.fps % 1 ? 2 : 0)} fps` : '–');
    addInfo('Video', describeVideo(v));
  } else {
    addInfo('Video', 'none', [{ text: 'audio only', kind: 'accent' }]);
  }
  addInfo('Audio', describeAudio(probe.audio));
}

/* ---------- Progress ---------- */

function revealProgress() {
  els.progress.hidden = false;
  els.result.hidden = true;
}

function showProgress(stage, ratio) {
  revealProgress();
  setStage(stage);
  setProgress(ratio);
}

function hideProgress() {
  els.progress.hidden = true;
}

function setStage(stage) {
  els.stageLabel.textContent = stage;
  state.stageStarted = performance.now();
  els.progressEta.textContent = '';
  setStatus(`${stage}…`);
  log.push(`--- ${stage} ---`);
  log.flush();
}

function setProgress(ratio) {
  if (ratio === null || ratio === undefined) {
    els.progressFill.classList.add('is-indeterminate');
    els.progressPct.textContent = '';
    els.progressBar.removeAttribute('aria-valuenow');
    return;
  }
  const pct = Math.round(Math.min(1, Math.max(0, ratio)) * 100);
  els.progressFill.classList.remove('is-indeterminate');
  els.progressFill.style.width = `${pct}%`;
  els.progressPct.textContent = `${pct}%`;
  els.progressBar.setAttribute('aria-valuenow', String(pct));
  const elapsed = (performance.now() - state.stageStarted) / 1000;
  if (ratio > 0.03 && elapsed > 2 && ratio < 1) {
    const left = elapsed * (1 - ratio) / ratio;
    els.progressEta.textContent = `about ${formatEta(left)} left`;
  } else if (ratio >= 1) {
    els.progressEta.textContent = '';
  }
}

function formatEta(s) {
  if (s < 60) return `${Math.max(1, Math.round(s))} s`;
  const m = Math.floor(s / 60);
  return `${m} min ${Math.round(s - m * 60)} s`;
}

/* ---------- Processing ---------- */

els.processBtn.addEventListener('click', process);
els.cancelBtn.addEventListener('click', cancel);

async function process() {
  if (!state.file || !state.probe || state.running || !runner.ready) return;
  resetResult();
  showNotice('');
  state.cancelled = false;
  setBusy(true);
  const settings = effectiveSettings();
  const started = performance.now();
  try {
    if (!state.inputWritten) {
      // The worker was terminated (cancel) since the file was read, so the FS is empty.
      showProgress(STAGES.READ, null);
      await runner.writeFile(state.inputName, await fetchFile(state.file));
      state.inputWritten = true;
    }
    revealProgress();
    const result = await runPipeline({
      runner,
      inputName: state.inputName,
      probe: state.probe,
      settings,
      onStage: (stage) => { setStage(stage); setProgress(0); },
      onProgress: setProgress,
    });
    const elapsed = (performance.now() - started) / 1000;
    showResult(result, settings, elapsed);
    setStatus('Done.', 'ok');
  } catch (err) {
    if (FFmpegRunner.isTerminated(err)) {
      state.cancelled = true;
      setStatus('Cancelled.');
      log.push('--- cancelled ---');
      log.flush();
    } else {
      console.error(err);
      log.push(`ERROR: ${err.message || err}`, 'err');
      setStatus(err.message || String(err), 'error');
      els.logDetails.open = true;
    }
  } finally {
    setBusy(false);
    hideProgress();
    updateProcessButton();
  }
}

async function cancel() {
  if (!state.running) return;
  els.cancelBtn.disabled = true;
  els.stageLabel.textContent = 'Cancelling…';
  state.inputWritten = false; // the wasm FS dies with the worker
  try {
    await runner.cancel();
  } catch (err) {
    console.error(err);
  }
}

window.addEventListener('beforeunload', (e) => {
  if (state.running) { e.preventDefault(); e.returnValue = ''; }
});

/* ---------- Result ---------- */

function resetResult() {
  if (state.downloadURL) URL.revokeObjectURL(state.downloadURL);
  state.downloadURL = null;
  state.result = null;
  els.result.hidden = true;
  els.compareBody.textContent = '';
  els.warnings.textContent = '';
  els.warnings.hidden = true;
}

function addRow(label, before, after, afterClass = '') {
  const tr = document.createElement('tr');
  const th = document.createElement('th');
  th.scope = 'row';
  th.textContent = label;
  const td1 = document.createElement('td');
  td1.textContent = before;
  const td2 = document.createElement('td');
  td2.textContent = after;
  if (afterClass) td2.className = afterClass;
  tr.append(th, td1, td2);
  els.compareBody.appendChild(tr);
}

function showResult(result, settings, elapsed) {
  const { data, after, ebur, measured, warnings, audioOnly } = result;
  const inProbe = state.probe;
  const inFile = state.file;

  const mime = audioOnly ? 'audio/mp4' : 'video/mp4';
  const ext = audioOnly ? 'm4a' : 'mp4';
  const blob = new Blob([data], { type: mime });
  state.downloadURL = URL.createObjectURL(blob);
  const outName = `${baseName(inFile.name)}_${settings.preset}.${ext}`;
  els.downloadBtn.href = state.downloadURL;
  els.downloadBtn.download = outName;
  els.downloadBtn.textContent = `Download ${outName} (${formatBytes(blob.size)})`;

  const dur = after.duration ?? inProbe.duration;
  const speed = dur ? (dur / elapsed).toFixed(1) : null;
  els.resultSummary.textContent = `Encoded in ${formatEta(elapsed)}${speed ? ` (${speed}x realtime)` : ''}${result.linear ? '' : ', dynamic loudnorm fallback'}`;

  els.compareBody.textContent = '';
  addRow('Size', formatBytes(inFile.size), formatBytes(blob.size));
  addRow('Duration', formatDuration(inProbe.duration), formatDuration(after.duration));
  const inV = inProbe.video;
  const outV = after.video;
  addRow('Video', inV ? `${inV.width}x${inV.height}${inV.rotation ? ` (rotated ${inV.rotation}°)` : ''} @ ${inV.fps ? formatNumber(inV.fps, inV.fps % 1 ? 2 : 0) : '?'} fps`
    : 'none', outV ? `${outV.width}x${outV.height} @ ${outV.fps ? formatNumber(outV.fps, outV.fps % 1 ? 2 : 0) : '?'} fps` : 'none (audio only)');
  addRow('Video codec / bitrate', describeVideo(inV), describeVideo(outV));
  addRow('Audio', describeAudio(inProbe.audio), describeAudio(after.audio));
  addRow('Overall bitrate', inProbe.bitrate ? `${inProbe.bitrate} kb/s` : '–', after.bitrate ? `${after.bitrate} kb/s` : '–');

  const outI = ebur?.I;
  const outPeak = ebur?.peak;
  const iClass = Number.isFinite(outI) ? (Math.abs(outI - settings.I) <= 1 ? 'is-good' : 'is-warn') : '';
  const pClass = Number.isFinite(outPeak) ? (outPeak <= settings.TP + 0.2 ? 'is-good' : 'is-warn') : '';
  addRow('Integrated loudness', formatNumber(measured?.input_i, 1, ' LUFS'), formatNumber(outI, 1, ' LUFS'), iClass);
  addRow('True peak', formatNumber(measured?.input_tp, 1, ' dBTP'), formatNumber(outPeak, 1, ' dBTP'), pClass);
  addRow('Loudness range', formatNumber(measured?.input_lra, 1, ' LU'), formatNumber(ebur?.LRA, 1, ' LU'));

  els.warnings.textContent = '';
  for (const w of warnings) {
    const li = document.createElement('li');
    li.textContent = w;
    els.warnings.appendChild(li);
  }
  els.warnings.hidden = warnings.length === 0;

  state.result = result;
  els.result.hidden = false;
  els.result.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

els.anotherBtn.addEventListener('click', async () => {
  if (state.running) return;
  resetResult();
  if (state.inputName && state.inputWritten && runner.ready) {
    await runner.deleteFile(state.inputName).catch(() => {});
  }
  state.inputWritten = false;
  state.file = null;
  state.probe = null;
  state.inputName = null;
  els.fileInfo.hidden = true;
  showNotice('');
  fillAdvanced();
  renderPlan();
  updateProcessButton();
  setStatus('Drop a file to begin.');
  els.dropzone.focus();
  els.dropzone.scrollIntoView({ behavior: 'smooth', block: 'start' });
});

/* ---------- Log controls ---------- */

els.logClear.addEventListener('click', () => log.clear());

/* ---------- Boot ---------- */

renderPresets();
restoreSettings();
els.advanced.addEventListener('input', onAdvancedInput);
updateProcessButton();

if (typeof WebAssembly === 'undefined') {
  setCoreStatus('error', 'This browser has no WebAssembly support.');
  setStatus('WebAssembly is required. Use a current Chrome, Edge, Firefox or Safari.', 'error');
} else {
  runner.load().catch((err) => console.error('ffmpeg load failed', err));
}
