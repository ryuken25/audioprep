// Encoding presets. Values mirror the desktop app exactly.
// The video box is "maxW x maxH" for landscape input; scale.js swaps it for portrait input.

export const PRESET_ORDER = ['x-audio', 'x-balanced', 'tiktok', 'audio-only', 'custom'];
export const DEFAULT_PRESET = 'x-audio';

const LOUDNESS = { I: -14, TP: -1.0, LRA: 11 };

export const PRESETS = {
  'x-audio': {
    id: 'x-audio',
    name: 'X (audio-first)',
    description: '720p, 2.5 Mb/s cap, 256k AAC, 16 kHz lowpass. Best for vocal covers.',
    audioOnly: false,
    maxW: 1280, maxH: 720, fpsCap: 30, crf: 23, maxrate: 2500, bufsize: 5000,
    audioBitrate: 256, ...LOUDNESS, lowpass: true, lowpassHz: 16000,
    warnDuration: 140,
  },
  'x-balanced': {
    id: 'x-balanced',
    name: 'X (balanced)',
    description: '1080p, 6 Mb/s cap, 256k AAC. Sharper picture, bigger file.',
    audioOnly: false,
    maxW: 1920, maxH: 1080, fpsCap: 30, crf: 23, maxrate: 6000, bufsize: 12000,
    audioBitrate: 256, ...LOUDNESS, lowpass: false, lowpassHz: 16000,
    warnDuration: null,
  },
  tiktok: {
    id: 'tiktok',
    name: 'TikTok / Shorts',
    description: '1080x1920 vertical, 8 Mb/s cap, 256k AAC.',
    audioOnly: false,
    maxW: 1080, maxH: 1920, fpsCap: 30, crf: 23, maxrate: 8000, bufsize: 16000,
    audioBitrate: 256, ...LOUDNESS, lowpass: false, lowpassHz: 16000,
    warnDuration: null,
  },
  'audio-only': {
    id: 'audio-only',
    name: 'Audio only (M4A)',
    description: 'Drops the video. Normalized 256k AAC in an .m4a container.',
    audioOnly: true,
    maxW: null, maxH: null, fpsCap: null, crf: null, maxrate: null, bufsize: null,
    audioBitrate: 256, ...LOUDNESS, lowpass: false, lowpassHz: 16000,
    warnDuration: null,
  },
  custom: {
    id: 'custom',
    name: 'Custom',
    description: 'Every knob exposed. Starts from whatever you last selected.',
    audioOnly: false,
    // Custom keeps the current values; these are only the initial defaults.
    maxW: 1280, maxH: 720, fpsCap: 30, crf: 23, maxrate: 2500, bufsize: 5000,
    audioBitrate: 256, ...LOUDNESS, lowpass: true, lowpassHz: 16000,
    warnDuration: null,
  },
};

// Knobs a user can edit in the Advanced panel. Changing any of these
// switches the preset selector to "custom".
export const TUNABLE_KEYS = [
  'I', 'TP', 'LRA', 'audioBitrate', 'lowpass', 'lowpassHz',
  'crf', 'maxrate', 'fpsCap', 'maxW', 'maxH',
];

/** Flat settings object for a preset id. Static-video options are preset-independent. */
export function settingsFromPreset(id, extra = {}) {
  const p = PRESETS[id] || PRESETS[DEFAULT_PRESET];
  return {
    preset: p.id,
    audioOnly: p.audioOnly,
    maxW: p.maxW, maxH: p.maxH, fpsCap: p.fpsCap,
    crf: p.crf, maxrate: p.maxrate, bufsize: p.bufsize,
    audioBitrate: p.audioBitrate,
    I: p.I, TP: p.TP, LRA: p.LRA,
    lowpass: p.lowpass, lowpassHz: p.lowpassHz,
    staticVideo: false, staticTime: 1.0,
    ...extra,
  };
}

/** True when every tunable knob equals the preset's value. */
export function matchesPreset(settings, id) {
  const p = PRESETS[id];
  if (!p || p.id === 'custom') return false;
  if (!!settings.audioOnly !== !!p.audioOnly) return false;
  return TUNABLE_KEYS.every((k) => {
    if (p.audioOnly && ['crf', 'maxrate', 'fpsCap', 'maxW', 'maxH'].includes(k)) return true;
    if (typeof p[k] === 'boolean') return !!settings[k] === p[k];
    return Number(settings[k]) === Number(p[k]);
  });
}
