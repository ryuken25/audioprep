// Parsers for loudnorm (pass 1, print_format=json) and the ebur128 summary block.

const KEYS = ['input_i', 'input_tp', 'input_lra', 'input_thresh', 'target_offset',
  'output_i', 'output_tp', 'output_lra', 'output_thresh'];

function toNumber(v) {
  if (typeof v === 'number') return v;
  const s = String(v).trim().toLowerCase();
  if (s === 'inf' || s === '+inf') return Infinity;
  if (s === '-inf') return -Infinity;
  const n = Number.parseFloat(s);
  return Number.isNaN(n) ? null : n;
}

/**
 * Extract the JSON block loudnorm prints at the end of pass 1.
 * Returns null when no block is found. Values are numbers (may be +-Infinity for silence).
 */
export function parseLoudnormJSON(logText) {
  const text = String(logText || '');
  const re = /\{[^{}]*"input_i"[^{}]*\}/g;
  let match;
  let last = null;
  while ((match = re.exec(text)) !== null) last = match[0];
  if (!last) return null;

  let obj;
  try {
    obj = JSON.parse(last);
  } catch {
    // Fall back to a key/value scan if the block is slightly malformed.
    obj = {};
    const kv = /"([a-z_]+)"\s*:\s*"?(-?[\w.+]+)"?/gi;
    let m;
    while ((m = kv.exec(last)) !== null) obj[m[1]] = m[2];
  }

  const out = { normalization_type: obj.normalization_type || null };
  for (const k of KEYS) out[k] = k in obj ? toNumber(obj[k]) : null;
  return out;
}

/** True when every value needed for the linear second pass is a finite number. */
export function measurementsUsable(m) {
  if (!m) return false;
  return ['input_i', 'input_tp', 'input_lra', 'input_thresh', 'target_offset']
    .every((k) => Number.isFinite(m[k]));
}

/**
 * Parse the "Summary:" block printed by the ebur128 filter.
 * @returns {{I:number|null, LRA:number|null, peak:number|null, threshold:number|null}|null}
 */
export function parseEbur128Summary(logText) {
  const text = String(logText || '');
  const idx = text.lastIndexOf('Summary:');
  if (idx < 0) return null;
  const block = text.slice(idx);
  const num = (re) => {
    const m = re.exec(block);
    return m ? toNumber(m[1]) : null;
  };
  return {
    I: num(/^\s*I:\s*(-?[\d.]+|-?inf)\s*LUFS/mi),
    threshold: num(/^\s*Threshold:\s*(-?[\d.]+|-?inf)\s*LUFS/mi),
    LRA: num(/^\s*LRA:\s*(-?[\d.]+|-?inf)\s*LU\b/mi),
    peak: num(/^\s*Peak:\s*(-?[\d.]+|-?inf)\s*dBFS/mi),
  };
}
