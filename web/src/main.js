import './style.css';
import { fetchFile } from '@ffmpeg/util';
import { FFmpegRunner } from './ffmpeg-runner.js';
import { PRESETS, PRESET_ORDER, DEFAULT_PRESET, TUNABLE_KEYS, settingsFromPreset, matchesPreset } from './presets.js';
import { parseProbe } from './probe.js';
import { buildProbeArgs, buildVideoFilter, describeWarning, isPictureMode, runPipeline, STAGES } from './pipeline.js';
import { coverBox, fitInsideBox } from './scale.js';
import { argsToShell, baseName, extOf, formatBytes, formatDuration, formatNumber } from './format.js';
import { LANGS, getLang, setLang, nextLang, t } from './i18n.js';

const ACCEPTED = ['mp4', 'mov', 'mkv', 'webm', 'avi', 'm4a', 'wav', 'mp3'];
const LS = { theme: 'kxc.theme', skin: 'kxc.skin', world: 'kxc.world', preset: 'kxc.preset', custom: 'kxc.custom' };
const VIDEO_KEYS = ['crf', 'maxrate', 'fpsCap', 'maxW', 'maxH', 'staticVideo', 'staticTime'];
const WORLDS = ['fanart', 'chalkboard', 'shrine'];
const COVER_EXT = { 'image/jpeg': 'jpg', 'image/png': 'png', 'image/webp': 'webp' };
const BASE = import.meta.env.BASE_URL;

const $ = (id) => document.getElementById(id);
const els = {
  worldImg: $('world-img'),
  themeToggle: $('theme-toggle'),
  langToggle: $('lang-toggle'),
  langCode: $('lang-code'),
  skinToggle: $('skin-toggle'),
  skinName: $('skin-name'),
  dropzone: $('dropzone'),
  fileInput: $('file-input'),
  notice: $('notice'),
  fileInfo: $('file-info'),
  fileName: $('file-name'),
  fileGrid: $('file-info-grid'),
  coverCard: $('cover-card'),
  coverDrop: $('cover-drop'),
  coverPreview: $('cover-preview'),
  coverInput: $('cover-input'),
  coverBtn: $('cover-btn'),
  coverClear: $('cover-clear'),
  coverLine: $('cover-line'),
  presetList: $('preset-list'),
  presetPlan: $('preset-plan'),
  advanced: $('advanced'),
  processBtn: $('process-btn'),
  processLabel: $('process-label'),
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
  artCredit: $('art-credit'),
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
  stageKey: null,
  // Thunks, not strings: every visible line has to survive a language switch.
  statusFn: null,
  statusKind: '',
  noticeFn: null,
  noticeKind: '',
  coreFn: null,
  etaFn: null,
  infoMode: null,        // 'loading' | 'ready'
  cover: null,           // { file, name, width, height }
  coverURL: null,
  lastResult: null,      // { result, settings, elapsed, blob, outName }
  downloadURL: null,
};

/* ---------- i18n ---------- */

/** Re-render every string in the live DOM. Called on boot and on each language switch. */
function renderI18n() {
  const T = t();
  document.querySelectorAll('[data-i18n]').forEach((el) => { el.textContent = t(el.dataset.i18n); });
  document.querySelectorAll('[data-i18n-label]').forEach((el) => { el.setAttribute('aria-label', t(el.dataset.i18nLabel)); });
  document.querySelectorAll('[data-i18n-title]').forEach((el) => { el.setAttribute('title', t(el.dataset.i18nTitle)); });

  els.langCode.textContent = T.code;
  els.log.dataset.empty = T.logEmpty;

  renderPresets();
  checkPresetRadio(state.settings.preset);
  paintStatus();
  paintNotice();
  paintCoreStatus();
  paintStage();
  paintEta();
  paintProcessLabel();
  paintLogCount();
  renderFileInfo();
  renderCover();
  renderPlan();
  paintResult();
}

els.langToggle.addEventListener('click', () => {
  setLang(nextLang());
  renderI18n();
});

/* ---------- Theme and skin ---------- */

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  try { localStorage.setItem(LS.theme, theme); } catch { /* private mode */ }
}

els.themeToggle.addEventListener('click', () => {
  applyTheme(document.documentElement.dataset.theme === 'light' ? 'dark' : 'light');
});

/** Show a world picture, or the flat studio look when `skin` is "studio". */
function applySkin(skin, world) {
  const d = document.documentElement;
  d.dataset.skin = skin;
  d.dataset.world = world;
  try {
    localStorage.setItem(LS.skin, skin);
    localStorage.setItem(LS.world, world);
  } catch { /* private mode */ }

  if (skin === 'vrchat') {
    const src = `${BASE}world/${world}.jpg`;
    if (!els.worldImg.src.endsWith(`/${world}.jpg`)) els.worldImg.src = src;
  } else {
    els.worldImg.removeAttribute('src'); // an empty src would re-request the page
  }
  els.skinName.textContent = skin === 'vrchat' ? world : 'plain';
  els.skinToggle.title = `${t().background}: ${skin === 'vrchat' ? world : 'plain'}`;
  els.skinToggle.setAttribute('aria-label', els.skinToggle.title);
  // The credit belongs to the picture, so it goes when the picture goes.
  els.artCredit.hidden = skin !== 'vrchat';
}

// fanart -> chalkboard -> shrine -> plain -> fanart
els.skinToggle.addEventListener('click', () => {
  const d = document.documentElement;
  if (d.dataset.skin !== 'vrchat') { applySkin('vrchat', WORLDS[0]); return; }
  const next = WORLDS.indexOf(d.dataset.world) + 1;
  if (next >= WORLDS.length) applySkin('studio', d.dataset.world);
  else applySkin('vrchat', WORLDS[next]);
});

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
    paintLogCount();
    els.log.scrollTop = els.log.scrollHeight;
  },
  clear() {
    clearTimeout(this.timer);
    this.timer = 0;
    this.pending = [];
    this.count = 0;
    els.log.textContent = '';
    paintLogCount();
  },
};

function paintLogCount() {
  els.logCount.textContent = log.count ? t().fn.lines(log.count) : '';
}

/* ---------- FFmpeg ---------- */

const runner = new FFmpegRunner({
  onLog: (message) => log.push(message),
  onCommand: (args) => log.push(`$ ${argsToShell(args)}`, 'cmd'),
  onStatus: (ev) => {
    if (ev.phase === 'loading') setCoreStatus('loading', () => t().coreLoading);
    else if (ev.phase === 'downloading') {
      const got = formatBytes(ev.received);
      const total = ev.total ? ` / ${formatBytes(ev.total)}` : '';
      setCoreStatus('loading', () => `${t().coreLoading} ${got}${total}`);
    } else if (ev.phase === 'ready') {
      setCoreStatus('ready', () => `${t().coreReady} (${formatBytes(ev.bytes)}, ${t().coreReadyNote})`);
      // After a cancel the "Cancelled." line is more useful than "Ready to process."
      if (!state.running && !state.cancelled) setStatus(() => (state.probe ? t().status.ready : t().status.idle));
    } else if (ev.phase === 'error') {
      setCoreStatus('error', () => t().coreErr);
      setStatus(() => `${t().coreErr} ${ev.error?.message || ev.error || ''}`.trim(), 'error');
    }
    updateProcessButton();
  },
});

function setCoreStatus(stateName, fn) {
  els.coreDot.dataset.state = stateName;
  state.coreFn = fn;
  paintCoreStatus();
}

function paintCoreStatus() {
  if (state.coreFn) els.coreText.textContent = state.coreFn();
}

/* ---------- Status helpers ---------- */

function setStatus(fn, kind = '') {
  state.statusFn = typeof fn === 'function' ? fn : () => fn;
  state.statusKind = kind;
  paintStatus();
}

function paintStatus() {
  els.status.textContent = state.statusFn ? state.statusFn() : '';
  els.status.className = `status${state.statusKind ? ` is-${state.statusKind}` : ''}`;
}

function showNotice(fn, kind = '') {
  state.noticeFn = fn ? (typeof fn === 'function' ? fn : () => fn) : null;
  state.noticeKind = kind;
  paintNotice();
}

function paintNotice() {
  const text = state.noticeFn ? state.noticeFn() : '';
  els.notice.textContent = text;
  els.notice.className = `notice${state.noticeKind ? ` is-${state.noticeKind}` : ''}`;
  els.notice.hidden = !text;
}

function paintProcessLabel() {
  els.processLabel.textContent = state.running ? t().processing : t().process;
}

function updateProcessButton() {
  els.processBtn.disabled = !(state.file && state.probe && runner.ready && !state.running);
}

function setBusy(busy) {
  state.running = busy;
  els.dropzone.classList.toggle('is-busy', busy);
  els.presetList.classList.toggle('is-disabled', busy);
  els.processBtn.classList.toggle('is-busy', busy);
  els.advanced.querySelectorAll('input').forEach((i) => { i.disabled = busy || i.dataset.forceDisabled === '1'; });
  els.coverBtn.disabled = busy;
  els.coverClear.disabled = busy;
  els.cancelBtn.disabled = false;
  paintProcessLabel();
  updateProcessButton();
}

/* ---------- Presets & settings ---------- */

function effectiveSettings() {
  const s = { ...state.settings };
  s.bufsize = Number(s.maxrate) * 2;
  return s;
}

/** True when the loaded file has no video and the preset still wants one. */
function pictureMode(s = effectiveSettings()) {
  return !!(state.probe && isPictureMode(state.probe, s));
}

function renderPresets() {
  const T = t();
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
    desc.textContent = T.presets[id] || '';
    label.append(input, name, desc);
    els.presetList.appendChild(label);
  }
  els.presetList.classList.toggle('is-disabled', state.running);
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
  renderCover();
  renderPlan();
}

function fillAdvanced() {
  const s = state.settings;
  const audioOnly = !!s.audioOnly;
  const picture = pictureMode();
  for (const input of els.advanced.querySelectorAll('input[data-key]')) {
    const key = input.dataset.key;
    if (input.type === 'checkbox') {
      // A picture encode is a still-video encode: the box is ticked and locked.
      if (key === 'staticVideo') input.checked = picture || !!s.staticVideo;
      else input.checked = !!s[key];
    } else if (input !== document.activeElement) {
      input.value = s[key] ?? ''; // don't fight the user's typing
    }
    // Video knobs are irrelevant when the output is audio only; the frame time is
    // irrelevant when there is no video to take a frame from.
    const off = (VIDEO_KEYS.includes(key) && audioOnly)
      || (key === 'staticVideo' && picture)
      || (key === 'staticTime' && (picture || !s.staticVideo))
      || (key === 'lowpassHz' && !s.lowpass);
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
  renderCover();
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
  const T = t();
  const s = effectiveSettings();
  if (s.audioOnly) {
    els.presetPlan.textContent = `${T.output}: .m4a, AAC ${s.audioBitrate} kb/s, 48 kHz, ${s.I} LUFS`;
    return;
  }
  const codec = `H.264 CRF ${s.crf} ≤ ${s.maxrate} kb/s, AAC ${s.audioBitrate} kb/s, ${s.I} LUFS`;
  if (!p.video) {
    const box = coverBox(s.maxW, s.maxH, state.cover?.width || 0, state.cover?.height || 0);
    if (!box) { els.presetPlan.textContent = ''; return; } // box knobs typed out of range
    els.presetPlan.textContent = `${T.output}: ${box.w}x${box.h} ${T.staticPicture}, ${codec}`;
    return;
  }
  const { dims } = buildVideoFilter(p, s);
  const fps = p.video.fps ? Math.min(p.video.fps, s.fpsCap || p.video.fps) : s.fpsCap;
  const mode = s.staticVideo ? `${dims.w}x${dims.h} ${T.staticFrame}` : `${dims.w}x${dims.h} @ ${formatNumber(fps, fps % 1 ? 2 : 0)} fps`;
  els.presetPlan.textContent = `${T.output}: ${mode}, ${codec}`;
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

['dragenter', 'dragover'].forEach((type) => els.dropzone.addEventListener(type, (e) => {
  e.preventDefault();
  if (state.running) return;
  e.dataTransfer.dropEffect = 'copy';
  els.dropzone.classList.add('is-over');
}));
['dragleave', 'drop'].forEach((type) => els.dropzone.addEventListener(type, (e) => {
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
    showNotice(() => `.${ext || '?'} — ${t().dropHint}`, 'error');
    return;
  }
  showNotice(null);
  resetResult();
  state.file = file;
  state.probe = null;
  updateProcessButton();
  renderFileInfoLoading(file);

  try {
    setStatus(() => t().status.loading);
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

    // An audio file keeps the chosen video preset: it becomes an MP4 with a picture.
    if (!probe.video) showNotice(() => t().noVideo);

    state.infoMode = 'ready';
    renderFileInfo();
    fillAdvanced();
    renderCover();
    renderPlan();
    setStatus(() => t().status.ready);
  } catch (err) {
    if (FFmpegRunner.isTerminated(err)) {
      state.cancelled = true;
      setStatus(() => t().status.cancelled);
    } else {
      console.error(err);
      log.push(`ERROR: ${err.message || err}`, 'err');
      setStatus(() => err.message || String(err), 'error');
    }
    log.flush();
    state.file = null;
    state.probe = null;
    state.infoMode = null;
    els.fileInfo.hidden = true;
    renderCover();
  } finally {
    setBusy(false);
    hideProgress();
    updateProcessButton();
  }
}

function renderFileInfoLoading(file) {
  state.infoMode = 'loading';
  renderFileInfo(file);
}

function addInfo(label, value, badges = [], mono = false) {
  const item = document.createElement('div');
  item.className = 'info-item';
  const dt = document.createElement('dt');
  dt.textContent = label;
  const dd = document.createElement('dd');
  if (mono) dd.classList.add('value-mono');
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
  const parts = [`${v.width}x${v.height}`];
  if (Number.isFinite(v.fps)) parts.push(`${formatNumber(v.fps, v.fps % 1 ? 2 : 0)} fps`);
  let codec = v.codec || '?';
  if (v.profile) codec += ` (${v.profile})`;
  parts.push(codec);
  if (v.pixFmt) parts.push(v.pixFmt);
  return parts.join(' · ');
}

function describeAudio(a) {
  if (!a) return '–';
  const parts = [a.codec || '?'];
  if (a.sampleRate) parts.push(`${(a.sampleRate / 1000).toFixed(a.sampleRate % 1000 ? 1 : 0)} kHz`);
  if (a.layout) parts.push(a.layout);
  else if (a.channels) parts.push(`${a.channels} ch`);
  return parts.join(' · ');
}

const kbps = (n) => (Number.isFinite(n) ? `${Math.round(n)} kb/s` : '–');

/** File info card. Re-runs on a language switch, so it reads everything from state. */
function renderFileInfo(file = state.file) {
  if (!state.infoMode || !file) return;
  const T = t();
  els.fileInfo.hidden = false;
  els.fileName.textContent = file.name;
  els.fileGrid.textContent = '';

  if (state.infoMode === 'loading') {
    addInfo(T.size, formatBytes(file.size), [], true);
    addInfo(T.statusRow, T.reading);
    return;
  }

  const probe = state.probe;
  const v = probe.video;
  const durBadges = [];
  const warnDur = PRESETS[state.settings.preset]?.warnDuration;
  if (warnDur && probe.duration > warnDur) durBadges.push({ text: T.over, kind: 'warn' });

  addInfo(T.size, formatBytes(file.size), [], true);
  addInfo(T.duration, formatDuration(probe.duration), durBadges, true);
  if (v) {
    const rotBadges = v.rotation ? [{ text: T.rotated, kind: 'accent' }] : [];
    addInfo(T.video, describeVideo(v), rotBadges, true);
    addInfo(T.videoBitrate, kbps(v.bitrate), [], true);
  } else {
    addInfo(T.video, T.none, [{ text: T.audioOnlyBadge, kind: 'accent' }]);
  }
  addInfo(T.audio, describeAudio(probe.audio), [], true);
  addInfo(T.audioBitrate, kbps(probe.audio?.bitrate), [], true);
}

/* ---------- Picture card ---------- */

function clearCover() {
  if (state.coverURL) URL.revokeObjectURL(state.coverURL);
  state.coverURL = null;
  state.cover = null;
}

async function setCover(file) {
  const ext = COVER_EXT[file.type] || (['jpg', 'jpeg', 'png', 'webp'].includes(extOf(file.name)) ? extOf(file.name).replace('jpeg', 'jpg') : null);
  if (!ext) {
    showNotice(() => t().pictureHint, 'error');
    return;
  }
  const url = URL.createObjectURL(file);
  try {
    const dims = await new Promise((resolve, reject) => {
      const img = new Image();
      img.onload = () => resolve({ width: img.naturalWidth, height: img.naturalHeight });
      img.onerror = () => reject(new Error('Could not read that picture.'));
      img.src = url;
    });
    clearCover();
    state.cover = { file, name: `cover.${ext}`, width: dims.width, height: dims.height };
    state.coverURL = url;
    showNotice(null);
  } catch (err) {
    URL.revokeObjectURL(url);
    showNotice(() => err.message || String(err), 'error');
  }
  renderCover();
  renderPlan();
}

/** The tile mirrors the output frame, so its aspect ratio follows the preset box. */
function renderCover() {
  const T = t();
  const s = effectiveSettings();
  const on = pictureMode(s);
  els.coverCard.hidden = !on;
  if (!on) return;

  const box = coverBox(s.maxW, s.maxH, state.cover?.width || 0, state.cover?.height || 0);
  els.coverDrop.style.setProperty('--cover-ar', `${box.w} / ${box.h}`);

  const c = state.cover;
  els.coverDrop.classList.toggle('has-picture', !!c);
  els.coverPreview.style.background = c
    ? `#000 url("${state.coverURL}") center/contain no-repeat`
    : '#000';
  els.coverBtn.textContent = c ? T.changePicture : T.choosePicture;
  els.coverClear.hidden = !c;
  if (c) {
    const fit = fitInsideBox(c.width, c.height, box.w, box.h);
    els.coverLine.textContent = T.fn.coverSet(c.file.name, fit.w, fit.h, c.width, c.height);
  } else {
    els.coverLine.textContent = T.fn.coverEmpty(box.w, box.h);
  }
}

els.coverBtn.addEventListener('click', () => { if (!state.running) els.coverInput.click(); });
els.coverDrop.addEventListener('click', () => { if (!state.running) els.coverInput.click(); });
els.coverDrop.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); if (!state.running) els.coverInput.click(); }
});
els.coverInput.addEventListener('change', () => {
  const f = els.coverInput.files?.[0];
  if (f) setCover(f);
  els.coverInput.value = '';
});
els.coverClear.addEventListener('click', (e) => {
  e.stopPropagation();
  if (state.running) return;
  clearCover();
  renderCover();
});
['dragenter', 'dragover'].forEach((type) => els.coverDrop.addEventListener(type, (e) => {
  e.preventDefault();
  e.stopPropagation();
  if (state.running) return;
  e.dataTransfer.dropEffect = 'copy';
  els.coverDrop.classList.add('is-over');
}));
['dragleave', 'drop'].forEach((type) => els.coverDrop.addEventListener(type, (e) => {
  e.preventDefault();
  els.coverDrop.classList.remove('is-over');
}));
els.coverDrop.addEventListener('drop', (e) => {
  e.stopPropagation();
  if (state.running) return;
  const f = e.dataTransfer?.files?.[0];
  if (f) setCover(f);
});

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

function setStage(key) {
  state.stageKey = key;
  state.stageStarted = performance.now();
  state.etaFn = null;
  paintStage();
  paintEta();
  setStatus(() => t().stages[key] || key);
  log.push(`--- ${t().stages[key] || key} ---`);
  log.flush();
}

function paintStage() {
  if (!state.stageKey) { els.stageLabel.textContent = ''; return; }
  els.stageLabel.textContent = t().stages[state.stageKey] || state.stageKey;
}

function paintEta() {
  els.progressEta.textContent = state.etaFn ? state.etaFn() : '';
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
    const [m, s] = splitTime(elapsed * (1 - ratio) / ratio);
    state.etaFn = () => t().fn.eta(m, s);
  } else if (ratio >= 1) {
    state.etaFn = null;
  }
  paintEta();
}

/** Seconds as [minutes, seconds] for the fn.eta / fn.dur formatters. */
function splitTime(seconds) {
  const total = Math.max(1, Math.round(seconds));
  const m = Math.floor(total / 60);
  return [m, total - m * 60];
}

/* ---------- Processing ---------- */

els.processBtn.addEventListener('click', process);
els.cancelBtn.addEventListener('click', cancel);

async function process() {
  if (!state.file || !state.probe || state.running || !runner.ready) return;
  resetResult();
  showNotice(null);
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
    // The picture is handed to the pipeline as bytes; it owns writing and deleting it.
    let cover = null;
    if (pictureMode(settings) && state.cover) {
      cover = { name: state.cover.name, data: await fetchFile(state.cover.file) };
    }
    revealProgress();
    const result = await runPipeline({
      runner,
      inputName: state.inputName,
      probe: state.probe,
      settings,
      cover,
      onStage: (stage) => { setStage(stage); setProgress(0); },
      onProgress: setProgress,
    });
    const elapsed = (performance.now() - started) / 1000;
    showResult(result, settings, elapsed);
    setStatus(() => t().status.done, 'ok');
  } catch (err) {
    if (FFmpegRunner.isTerminated(err)) {
      state.cancelled = true;
      setStatus(() => t().status.cancelled);
      log.push('--- cancelled ---');
      log.flush();
    } else {
      console.error(err);
      log.push(`ERROR: ${err.message || err}`, 'err');
      setStatus(() => err.message || String(err), 'error');
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
  els.stageLabel.textContent = t().cancelling;
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
  state.lastResult = null;
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

function warningText(w) {
  if (w.code === 'duration') return t().fn.warnDur(w.duration.toFixed(1), w.max);
  return describeWarning(w, t().fn);
}

function showResult(result, settings, elapsed) {
  const { data, audioOnly } = result;
  const mime = audioOnly ? 'audio/mp4' : 'video/mp4';
  const ext = audioOnly ? 'm4a' : 'mp4';
  const blob = new Blob([data], { type: mime });
  state.downloadURL = URL.createObjectURL(blob);
  state.lastResult = {
    result,
    settings,
    elapsed,
    blob,
    outName: `${baseName(state.file.name)}_${settings.preset}.${ext}`,
  };
  paintResult();
  els.result.hidden = false;
  els.result.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

/** Render the result card from state. Safe to call again after a language switch. */
function paintResult() {
  if (!state.lastResult || !state.probe || !state.file) return;
  const T = t();
  const { result, settings, elapsed, blob, outName } = state.lastResult;
  const { after, ebur, measured, warnings } = result;
  const inProbe = state.probe;

  els.downloadBtn.href = state.downloadURL;
  els.downloadBtn.download = outName;
  els.downloadBtn.textContent = `${T.download} ${outName} (${formatBytes(blob.size)})`;

  els.resultSummary.textContent = T.fn.summary(
    T.fn.dur(...splitTime(elapsed)),
    formatNumber(ebur?.I, 1),
    formatNumber(ebur?.peak, 1),
    formatBytes(blob.size),
  );

  els.compareBody.textContent = '';
  const inV = inProbe.video;
  const outV = after.video;
  addRow(T.size, formatBytes(state.file.size), formatBytes(blob.size));
  addRow(T.duration, formatDuration(inProbe.duration), formatDuration(after.duration));
  addRow(T.video,
    inV ? describeVideo(inV) : T.none,
    outV ? describeVideo(outV) : T.none);
  addRow(T.videoBitrate, kbps(inV?.bitrate), kbps(outV?.bitrate));
  addRow(T.audio, describeAudio(inProbe.audio), describeAudio(after.audio));
  addRow(T.audioBitrate, kbps(inProbe.audio?.bitrate), kbps(after.audio?.bitrate));
  addRow(T.overall, kbps(inProbe.bitrate), kbps(after.bitrate));

  const outI = ebur?.I;
  const outPeak = ebur?.peak;
  const iClass = Number.isFinite(outI) ? (Math.abs(outI - settings.I) <= 1 ? 'is-good' : 'is-warn') : '';
  const pClass = Number.isFinite(outPeak) ? (outPeak <= settings.TP + 0.2 ? 'is-good' : 'is-warn') : '';
  addRow(T.loudnessRow, formatNumber(measured?.input_i, 1, ' LUFS'), formatNumber(outI, 1, ' LUFS'), iClass);
  addRow(T.truePeak, formatNumber(measured?.input_tp, 1, ' dBTP'), formatNumber(outPeak, 1, ' dBTP'), pClass);
  addRow(T.lra, formatNumber(measured?.input_lra, 1, ' LU'), formatNumber(ebur?.LRA, 1, ' LU'));

  els.warnings.textContent = '';
  for (const w of warnings) {
    const li = document.createElement('li');
    li.textContent = warningText(w);
    els.warnings.appendChild(li);
  }
  els.warnings.hidden = warnings.length === 0;
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
  state.infoMode = null;
  clearCover();
  els.fileInfo.hidden = true;
  showNotice(null);
  fillAdvanced();
  renderCover();
  renderPlan();
  updateProcessButton();
  setStatus(() => t().status.idle);
  els.dropzone.focus();
  els.dropzone.scrollIntoView({ behavior: 'smooth', block: 'start' });
});

/* ---------- Log controls ---------- */

els.logClear.addEventListener('click', () => log.clear());

/* ---------- Boot ---------- */

const d = document.documentElement;
setLang(LANGS.includes(d.dataset.lang) ? d.dataset.lang : getLang());
applySkin(d.dataset.skin === 'studio' ? 'studio' : 'vrchat', WORLDS.includes(d.dataset.world) ? d.dataset.world : WORLDS[0]);
restoreSettings();
setStatus(() => t().status.loading);
setCoreStatus('loading', () => t().coreLoading);
renderI18n();
els.advanced.addEventListener('input', onAdvancedInput);
updateProcessButton();

if (typeof WebAssembly === 'undefined') {
  setCoreStatus('error', () => t().coreErr);
  setStatus(() => 'WebAssembly is required. Use a current Chrome, Edge, Firefox or Safari.', 'error');
} else {
  runner.load().catch((err) => console.error('ffmpeg load failed', err));
}
