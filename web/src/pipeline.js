// Command builders (pure) and the run orchestrator for the encode pipeline.
// Nothing in here touches the DOM; `runPipeline` only needs a runner object with
// exec / readFile / deleteFile.

import { fitDimensions, coverBox } from './scale.js';
import { parseProbe } from './probe.js';
import { parseLoudnormJSON, measurementsUsable, parseEbur128Summary } from './loudnorm.js';

// Stage identifiers, not labels: the UI looks them up in STR[lang].stages so the
// progress card can switch language mid-run.
export const STAGES = {
  LOAD: 'load',
  READ: 'read',
  PROBE: 'probe',
  MEASURE: 'measure',
  FRAME: 'frame',
  ENCODE: 'encode',
  CHECK: 'check',
};

export const X_LIMITS = {
  maxDuration: 140,
  maxBytes: 512 * 1024 * 1024,
  maxLong: 1920,
  maxShort: 1200,
  maxFps: 60,
};

const num = (v, digits = 2) => {
  const n = Number(v);
  return Number.isFinite(n) ? String(+n.toFixed(digits)) : String(v);
};

export function loudnormTarget(s) {
  return `I=${num(s.I)}:TP=${num(s.TP)}:LRA=${num(s.LRA)}`;
}

/** Audio filter for the measurement pass. */
export function buildAF1(s) {
  const lp = s.lowpass ? `lowpass=f=${Math.round(s.lowpassHz)},` : '';
  return `${lp}loudnorm=${loudnormTarget(s)}:print_format=json`;
}

/** Audio filter for the encode pass. Falls back to dynamic loudnorm when the measurement is unusable. */
export function buildAF2(s, measured) {
  const lp = s.lowpass ? `lowpass=f=${Math.round(s.lowpassHz)},` : '';
  const m = measured && measurementsUsable(measured)
    ? `:measured_I=${num(measured.input_i)}:measured_TP=${num(measured.input_tp)}`
      + `:measured_LRA=${num(measured.input_lra)}:measured_thresh=${num(measured.input_thresh)}`
      + `:offset=${num(measured.target_offset)}:linear=true`
    : '';
  return `${lp}loudnorm=${loudnormTarget(s)}${m}:print_format=summary,aresample=48000`;
}

/** Build the -vf chain (scale + fps cap) or null when neither is needed. */
export function buildVideoFilter(probe, s) {
  const v = probe.video;
  const dims = fitDimensions(v.width, v.height, v.rotation || 0, s.maxW, s.maxH);
  const parts = [];
  if (dims.scaled) parts.push(`scale=${dims.w}:${dims.h}`);
  if (s.fpsCap && Number.isFinite(v.fps) && v.fps > Number(s.fpsCap) + 0.01) parts.push(`fps=${num(s.fpsCap)}`);
  return { vf: parts.length ? parts.join(',') : null, dims };
}

export function buildVideoCodecArgs(s, { still = false } = {}) {
  const bufsize = s.bufsize || Number(s.maxrate) * 2;
  const args = ['-c:v', 'libx264', '-preset', 'veryfast'];
  if (still) args.push('-tune', 'stillimage');
  args.push(
    '-profile:v', 'high', '-level', '4.1', '-pix_fmt', 'yuv420p',
    '-crf', num(s.crf, 0), '-maxrate', `${num(s.maxrate, 0)}k`, '-bufsize', `${num(bufsize, 0)}k`,
  );
  if (still) args.push('-r', '1');
  return args;
}

export function buildAudioCodecArgs(s) {
  return ['-c:a', 'aac', '-b:a', `${num(s.audioBitrate, 0)}k`, '-ar', '48000', '-ac', '2'];
}

export function buildProbeArgs(input) {
  return ['-hide_banner', '-i', input];
}

export function buildMeasureArgs(input, s) {
  return ['-hide_banner', '-nostdin', '-i', input, '-map', '0:a:0', '-af', buildAF1(s), '-f', 'null', '-'];
}

export function buildFrameArgs(input, time, output = 'frame.png') {
  return ['-hide_banner', '-nostdin', '-ss', num(time, 3), '-i', input, '-an', '-frames:v', '1', '-update', '1', output];
}

/**
 * -vf chain for a still picture: fit it inside the preset box without stretching,
 * then pad the rest of the frame black so the output is exactly the box size.
 * Also used, harmlessly, for the generated black frame (which is already box-sized).
 */
export function buildCoverFilter(box) {
  return `scale=${box.w}:${box.h}:force_original_aspect_ratio=decrease`
    + `,pad=${box.w}:${box.h}:(ow-iw)/2:(oh-ih)/2:color=black`;
}

/** True when the input has no video and the preset still asks for one: picture + audio. */
export function isPictureMode(probe, s) {
  return !s.audioOnly && !probe.video && !!coverBox(s.maxW, s.maxH);
}

/**
 * Main encode command.
 * @param {{input:string, output:string, probe:object, settings:object, measured?:object,
 *   frame?:string, cover?:string|null}} o `cover` is the picture already written to the
 *   ffmpeg FS; null means a generated black frame.
 */
export function buildEncodeArgs({ input, output, probe, settings: s, measured = null, frame = 'frame.png', cover = null, coverW = 0, coverH = 0 }) {
  const af = buildAF2(s, measured);
  const audio = buildAudioCodecArgs(s);
  const tail = ['-af', af, ...audio, '-movflags', '+faststart', output];

  if (s.audioOnly || (!probe.video && !isPictureMode(probe, s))) {
    return ['-hide_banner', '-nostdin', '-i', input, '-vn', '-map', '0:a:0', ...tail];
  }

  // Audio input, video output: one still picture (or black) stretched over the track.
  if (!probe.video) {
    const box = coverBox(s.maxW, s.maxH, coverW, coverH);
    // -shortest ends at the audio, but a looped image can overrun it, so cap with -t too.
    const tArgs = Number.isFinite(probe.duration) ? ['-t', num(probe.duration, 3)] : [];
    const source = cover
      ? ['-framerate', '1', '-loop', '1', '-i', cover]
      : ['-f', 'lavfi', '-i', `color=c=black:s=${box.w}x${box.h}:r=1`];
    return [
      '-hide_banner', '-nostdin',
      ...source,
      '-i', input,
      '-map', '0:v', '-map', '1:a', '-shortest', ...tArgs,
      '-vf', buildCoverFilter(box),
      ...buildVideoCodecArgs(s, { still: true }),
      ...tail,
    ];
  }

  const { vf } = buildVideoFilter(probe, s);
  const vfArgs = vf ? ['-vf', vf] : [];

  if (s.staticVideo) {
    // -shortest alone can overrun with a looped image input, so also cap with -t when the duration is known.
    const tArgs = Number.isFinite(probe.duration) ? ['-t', num(probe.duration, 3)] : [];
    return [
      '-hide_banner', '-nostdin',
      '-framerate', '1', '-loop', '1', '-i', frame,
      '-i', input,
      '-map', '0:v', '-map', '1:a', '-shortest', ...tArgs,
      ...vfArgs, ...buildVideoCodecArgs(s, { still: true }),
      ...tail,
    ];
  }

  return [
    '-hide_banner', '-nostdin', '-i', input,
    '-map', '0:v:0', '-map', '0:a:0',
    ...vfArgs, ...buildVideoCodecArgs(s),
    ...tail,
  ];
}

/** Post-check: loudness summary of the output. -vn keeps the wasm decoder off the video stream. */
export function buildCheckArgs(output) {
  return ['-hide_banner', '-nostdin', '-i', output, '-vn', '-af', 'ebur128=peak=true', '-f', 'null', '-'];
}

/**
 * Warnings for an encoded (or planned) output, as data rather than sentences: the UI
 * localizes the duration one through STR.fn.warnDur and renders the rest with
 * describeWarning below.
 */
export function computeWarnings({ duration, sizeBytes, video }) {
  const out = [];
  if (Number.isFinite(duration) && duration > X_LIMITS.maxDuration) {
    out.push({ code: 'duration', duration, max: X_LIMITS.maxDuration });
  }
  if (Number.isFinite(sizeBytes) && sizeBytes > X_LIMITS.maxBytes) {
    out.push({ code: 'size', mb: sizeBytes / 1048576, maxMb: X_LIMITS.maxBytes / 1048576 });
  }
  if (video && Number.isFinite(video.width) && Number.isFinite(video.height)) {
    const long = Math.max(video.width, video.height);
    const short = Math.min(video.width, video.height);
    if (long > X_LIMITS.maxLong || short > X_LIMITS.maxShort) {
      out.push({ code: 'resolution', width: video.width, height: video.height, maxLong: X_LIMITS.maxLong, maxShort: X_LIMITS.maxShort });
    }
  }
  if (video && Number.isFinite(video.fps) && video.fps > X_LIMITS.maxFps) {
    out.push({ code: 'fps', fps: video.fps, max: X_LIMITS.maxFps });
  }
  return out;
}

/** English sentence for a warning. The string table only covers `duration` (fn.warnDur). */
export function describeWarning(w) {
  switch (w.code) {
    case 'duration': return `Duration is ${w.duration.toFixed(1)} s. X allows up to ${w.max} s for most accounts.`;
    case 'size': return `File is ${w.mb.toFixed(0)} MB. X rejects uploads over ${w.maxMb} MB.`;
    case 'resolution': return `Resolution ${w.width}x${w.height} exceeds X's ${w.maxLong}x${w.maxShort} limit.`;
    case 'fps': return `Frame rate ${w.fps} fps is above ${w.max}. X may reject or downsample it.`;
    default: return String(w.code || '');
  }
}

const clamp01 = (x) => (Number.isFinite(x) ? Math.min(1, Math.max(0, x)) : 0);

/**
 * Run the whole pipeline on a file that is already in the ffmpeg FS.
 * `runner.exec(args, {duration, onProgress})` must resolve to `{code, log, ms}`.
 */
export async function runPipeline({ runner, inputName, probe, settings, cover = null, onStage = () => {}, onProgress = () => {} }) {
  // An input without a video stream is only forced to audio-only when the preset has no
  // video box; otherwise it becomes an MP4 with a still picture.
  const picture = isPictureMode(probe, settings);
  const audioOnly = !!settings.audioOnly || (!probe.video && !picture);
  const s = { ...settings, audioOnly };
  const outputName = audioOnly ? 'out.m4a' : 'out.mp4';
  const duration = Number.isFinite(probe.duration) ? probe.duration : null;
  const timings = {};
  const progress = (ratio) => onProgress(clamp01(ratio));

  // Pass 1: measure loudness.
  onStage(STAGES.MEASURE);
  progress(0);
  const r1 = await runner.exec(buildMeasureArgs(inputName, s), { duration, onProgress: progress });
  timings.measure = r1.ms;
  if (r1.code !== 0) throw new Error(`Loudness measurement failed (ffmpeg exit code ${r1.code}). See the log.`);
  const measured = parseLoudnormJSON(r1.log);
  if (!measured) throw new Error('Could not find the loudnorm measurement block in the ffmpeg log.');
  const linear = measurementsUsable(measured);

  // The cover picture lives in the ffmpeg FS only for the length of the encode.
  let coverName = null;
  if (picture && cover && cover.data && cover.name) {
    await runner.writeFile(cover.name, cover.data);
    coverName = cover.name;
  }

  // Optional still frame for static video mode (video inputs only).
  let frame = null;
  if (!audioOnly && !picture && s.staticVideo) {
    onStage(STAGES.FRAME);
    progress(0);
    const maxT = duration ? Math.max(0, duration - 0.05) : Infinity;
    const t = Math.min(Math.max(0, Number(s.staticTime) || 0), maxT);
    const rf = await runner.exec(buildFrameArgs(inputName, t, 'frame.png'));
    timings.frame = rf.ms;
    if (rf.code !== 0) throw new Error(`Could not extract a frame at ${t.toFixed(2)} s (exit code ${rf.code}).`);
    frame = 'frame.png';
  }

  // Pass 2: encode.
  onStage(STAGES.ENCODE);
  progress(0);
  const encodeArgs = buildEncodeArgs({
    input: inputName, output: outputName, probe, settings: s, measured,
    frame: frame || 'frame.png', cover: coverName,
    coverW: cover?.width || 0, coverH: cover?.height || 0,
  });
  const r2 = await runner.exec(encodeArgs, { duration, onProgress: progress });
  timings.encode = r2.ms;
  if (r2.code !== 0) throw new Error(`Encoding failed (ffmpeg exit code ${r2.code}). See the log.`);
  progress(1);

  const data = await runner.readFile(outputName);
  if (!data || !data.byteLength) throw new Error('Encoding produced an empty file.');

  // Post-check: re-probe the output and measure its loudness.
  onStage(STAGES.CHECK);
  progress(0);
  const r3 = await runner.exec(buildCheckArgs(outputName), { duration, onProgress: progress });
  timings.check = r3.ms;
  const after = parseProbe(r3.log);
  const ebur = r3.code === 0 ? parseEbur128Summary(r3.log) : null;
  progress(1);

  // Free wasm memory. The input stays so the user can re-run with another preset.
  await runner.deleteFile(outputName).catch(() => {});
  if (frame) await runner.deleteFile(frame).catch(() => {});
  if (coverName) await runner.deleteFile(coverName).catch(() => {});

  const warnings = computeWarnings({ duration: after.duration ?? duration, sizeBytes: data.byteLength, video: after.video });

  return { outputName, audioOnly, picture, data, measured, linear, after, ebur, warnings, timings };
}
