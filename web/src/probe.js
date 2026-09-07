// Parsers for the stderr text ffmpeg prints for `-i input` (no output).
// Every function is pure and takes the joined log text.

const DURATION_RE = /Duration:\s*(\d+):(\d+):(\d+)(?:\.(\d+))?/;
const OVERALL_BITRATE_RE = /Duration:[^\n]*?bitrate:\s*(\d+(?:\.\d+)?)\s*kb\/s/;
const STREAM_RE = /Stream #(\d+):(\d+)/;

const CHANNEL_LAYOUTS = {
  mono: 1, stereo: 2, '2.1': 3, '3.0': 3, '4.0': 4, quad: 4, '3.1': 4, '5.0': 5,
  '4.1': 5, '5.1': 6, '6.0': 6, hexagonal: 6, '6.1': 7, '7.0': 7, '7.1': 8, octagonal: 8,
};

function stripParens(s) {
  // Remove every "(...)" group, repeated for nested groups.
  let prev;
  do { prev = s; s = s.replace(/\([^()]*\)/g, ''); } while (s !== prev);
  return s.replace(/\s{2,}/g, ' ').trim();
}

function splitFields(line) {
  // Split on top-level commas only (commas inside parentheses stay put).
  const out = [];
  let depth = 0;
  let cur = '';
  for (const ch of line) {
    if (ch === '(') depth++;
    else if (ch === ')') depth = Math.max(0, depth - 1);
    if (ch === ',' && depth === 0) { out.push(cur.trim()); cur = ''; } else cur += ch;
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function parseRate(s) {
  if (/k$/.test(s)) return Number(s.slice(0, -1)) * 1000;
  return Number(s);
}

export function parseDuration(text) {
  const m = DURATION_RE.exec(text);
  if (!m) return null;
  const frac = m[4] ? Number(`0.${m[4]}`) : 0;
  return Number(m[1]) * 3600 + Number(m[2]) * 60 + Number(m[3]) + frac;
}

export function parseOverallBitrate(text) {
  const m = OVERALL_BITRATE_RE.exec(text);
  return m ? Number(m[1]) : null;
}

/** Normalize any rotation to 0 / 90 / 180 / 270. */
export function normalizeRotation(deg) {
  const r = Math.round(Number(deg) || 0);
  return ((r % 360) + 360) % 360;
}

export function parseRotation(text) {
  // ffmpeg >= 5 prints side data: "displaymatrix: rotation of -90.00 degrees"
  const dm = /displaymatrix:\s*rotation of\s*(-?\d+(?:\.\d+)?)\s*degrees/i.exec(text);
  if (dm) return normalizeRotation(-Number(dm[1])); // displaymatrix sign is inverted vs. the rotate tag
  // ffmpeg < 5 prints the metadata tag: "rotate          : 90"
  const rt = /^\s*rotate\s*:\s*(-?\d+)/im.exec(text);
  if (rt) return normalizeRotation(Number(rt[1]));
  return 0;
}

export function parseVideoStream(text) {
  const lines = String(text).split(/\r?\n/);
  for (const line of lines) {
    if (!/: Video: /.test(line) || !STREAM_RE.test(line)) continue;
    if (/attached pic/i.test(line)) continue; // cover art embedded in audio files
    const idx = STREAM_RE.exec(line);
    const body = line.slice(line.indexOf(': Video: ') + ': Video: '.length);
    const fields = splitFields(body);

    const codecField = fields[0] || '';
    const codec = stripParens(codecField).split(/\s+/)[0] || null;
    const profileMatch = /\(([^()]*)\)/.exec(codecField);
    const profile = profileMatch && !/\//.test(profileMatch[1]) ? profileMatch[1] : null;

    let pixFmt = null;
    let width = null;
    let height = null;
    let fps = null;
    let bitrate = null;
    for (let i = 1; i < fields.length; i++) {
      const f = stripParens(fields[i]);
      const dim = /^(\d{2,5})x(\d{2,5})\b/.exec(f);
      if (dim && width === null) { width = Number(dim[1]); height = Number(dim[2]); continue; }
      const br = /^(\d+(?:\.\d+)?)\s*kb\/s$/.exec(f);
      if (br && bitrate === null) { bitrate = Number(br[1]); continue; }
      const fr = /^(\d+(?:\.\d+)?k?)\s*fps$/.exec(f);
      if (fr && fps === null) { fps = parseRate(fr[1]); continue; }
      const tbr = /^(\d+(?:\.\d+)?k?)\s*tbr$/.exec(f);
      if (tbr && fps === null) { fps = parseRate(tbr[1]); continue; }
      const first = f.split(/\s+/)[0];
      if (i === 1 && pixFmt === null && /^[a-z][a-z0-9]*$/i.test(first)) pixFmt = first;
    }
    if (width === null) continue;
    return {
      index: Number(idx[2]), codec, profile, pixFmt, width, height, fps, bitrate,
      rotation: parseRotation(text),
    };
  }
  return null;
}

export function parseAudioStream(text) {
  const lines = String(text).split(/\r?\n/);
  for (const line of lines) {
    if (!/: Audio: /.test(line) || !STREAM_RE.test(line)) continue;
    const idx = STREAM_RE.exec(line);
    const body = line.slice(line.indexOf(': Audio: ') + ': Audio: '.length);
    const fields = splitFields(body);
    const codec = stripParens(fields[0] || '').split(/\s+/)[0] || null;

    let sampleRate = null;
    let channels = null;
    let layout = null;
    let bitrate = null;
    let sampleFmt = null;
    for (let i = 1; i < fields.length; i++) {
      const f = stripParens(fields[i]);
      const hz = /^(\d+)\s*Hz$/.exec(f);
      if (hz) { sampleRate = Number(hz[1]); continue; }
      const br = /^(\d+(?:\.\d+)?)\s*kb\/s$/.exec(f);
      if (br) { bitrate = Number(br[1]); continue; }
      const nch = /^(\d+)\s*channels?$/.exec(f);
      if (nch) { channels = Number(nch[1]); layout = f; continue; }
      const lay = f.toLowerCase();
      if (CHANNEL_LAYOUTS[lay] !== undefined) { channels = CHANNEL_LAYOUTS[lay]; layout = lay; continue; }
      if (/^(s16|s32|flt|dbl|u8|s16p|s32p|fltp|dblp|s64|s64p)$/.test(f)) sampleFmt = f;
    }
    if (sampleRate === null) continue;
    return { index: Number(idx[2]), codec, sampleRate, channels, layout, bitrate, sampleFmt };
  }
  return null;
}

/**
 * Parse everything we need from an `ffmpeg -i file` log.
 * @returns {{duration:number|null, bitrate:number|null, video:object|null, audio:object|null, rotation:number}}
 */
export function parseProbe(text) {
  const video = parseVideoStream(text);
  const audio = parseAudioStream(text);
  return {
    duration: parseDuration(text),
    bitrate: parseOverallBitrate(text),
    video,
    audio,
    rotation: video ? video.rotation : 0,
  };
}
