// Small display helpers shared by the UI and the log.

export function formatBytes(n) {
  if (!Number.isFinite(n)) return '–';
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB'];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(v >= 100 ? 0 : 1)} ${units[i]}`;
}

export function formatDuration(s) {
  if (!Number.isFinite(s)) return '–';
  const m = Math.floor(s / 60);
  const sec = s - m * 60;
  return m > 0 ? `${m}m ${sec.toFixed(1)}s` : `${sec.toFixed(2)}s`;
}

export function formatNumber(n, digits = 1, suffix = '') {
  if (!Number.isFinite(n)) return '–';
  return `${n.toFixed(digits)}${suffix}`;
}

/** Render an argv array as a copy-pasteable shell command. */
export function argsToShell(args, bin = 'ffmpeg') {
  const quote = (a) => {
    const s = String(a);
    if (/^[A-Za-z0-9_+\-.:/=,@%]+$/.test(s)) return s;
    return `'${s.replace(/'/g, "'\\''")}'`;
  };
  return [bin, ...args.map(quote)].join(' ');
}

/** File name without its extension. */
export function baseName(name) {
  const i = name.lastIndexOf('.');
  return i > 0 ? name.slice(0, i) : name;
}

export function extOf(name) {
  const i = name.lastIndexOf('.');
  return i >= 0 ? name.slice(i + 1).toLowerCase() : '';
}
