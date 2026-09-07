import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  buildAF1, buildAF2, buildVideoFilter, buildEncodeArgs, buildMeasureArgs, buildCheckArgs,
  buildFrameArgs, buildCoverFilter, isPictureMode, computeWarnings, describeWarning, runPipeline,
} from '../src/pipeline.js';
import { settingsFromPreset, matchesPreset, PRESETS } from '../src/presets.js';
import { argsToShell } from '../src/format.js';

const probe1080p = {
  duration: 62.35, bitrate: 12000,
  video: { width: 1920, height: 1080, fps: 59.94, rotation: 0, codec: 'h264' },
  audio: { codec: 'aac', sampleRate: 44100, channels: 2 },
  rotation: 0,
};

const measured = { input_i: -19.83, input_tp: -0.42, input_lra: 9.7, input_thresh: -30.31, target_offset: 0.35 };

test('x-audio preset filters', () => {
  const s = settingsFromPreset('x-audio');
  assert.equal(buildAF1(s), 'lowpass=f=16000,loudnorm=I=-14:TP=-1:LRA=11:print_format=json');
  assert.equal(
    buildAF2(s, measured),
    'lowpass=f=16000,loudnorm=I=-14:TP=-1:LRA=11:measured_I=-19.83:measured_TP=-0.42:measured_LRA=9.7'
    + ':measured_thresh=-30.31:offset=0.35:linear=true:print_format=summary,aresample=48000',
  );
  // Unusable measurement (silence) falls back to single-pass dynamic loudnorm.
  assert.equal(
    buildAF2(s, { ...measured, input_i: -Infinity }),
    'lowpass=f=16000,loudnorm=I=-14:TP=-1:LRA=11:print_format=summary,aresample=48000',
  );
  const bal = settingsFromPreset('x-balanced');
  assert.equal(buildAF1(bal), 'loudnorm=I=-14:TP=-1:LRA=11:print_format=json');
});

test('video filter: scale + fps cap only when needed', () => {
  const s = settingsFromPreset('x-audio');
  assert.equal(buildVideoFilter(probe1080p, s).vf, 'scale=1280:720,fps=24');
  const fits = { ...probe1080p, video: { ...probe1080p.video, width: 1280, height: 720, fps: 24 } };
  assert.equal(buildVideoFilter(fits, s).vf, null);
  const portrait = { ...probe1080p, video: { ...probe1080p.video, rotation: 90, fps: 24 } };
  assert.equal(buildVideoFilter(portrait, s).vf, 'scale=720:1280');
  const thirty = { ...fits, video: { ...fits.video, fps: 30 } };
  assert.equal(buildVideoFilter(thirty, s).vf, 'fps=24', '30 fps input is capped to 24 even when no scaling is needed');
});

test('encode args for a normal video', () => {
  const s = { ...settingsFromPreset('x-audio'), bufsize: 3600 };
  const args = buildEncodeArgs({ input: 'in.mp4', output: 'out.mp4', probe: probe1080p, settings: s, measured });
  assert.deepEqual(args, [
    '-hide_banner', '-nostdin', '-i', 'in.mp4',
    '-map', '0:v:0', '-map', '0:a:0',
    '-vf', 'scale=1280:720,fps=24',
    '-c:v', 'libx264', '-preset', 'veryfast', '-profile:v', 'high', '-level', '4.1', '-pix_fmt', 'yuv420p',
    '-crf', '26', '-maxrate', '1800k', '-bufsize', '3600k',
    '-af', buildAF2(s, measured),
    '-c:a', 'aac', '-b:a', '320k', '-ar', '48000', '-ac', '2',
    '-movflags', '+faststart', 'out.mp4',
  ]);
  assert.ok(argsToShell(args).startsWith("ffmpeg -hide_banner -nostdin -i in.mp4"));
  assert.ok(argsToShell(args).includes("'scale=1280:720,fps=24'") === false, 'plain filter strings need no quotes');
});

test('encode args for static video mode', () => {
  const s = { ...settingsFromPreset('x-audio'), staticVideo: true, staticTime: 2.5 };
  const args = buildEncodeArgs({ input: 'in.mp4', output: 'out.mp4', probe: probe1080p, settings: s, measured, frame: 'frame.png' });
  assert.deepEqual(args.slice(0, 15), [
    '-hide_banner', '-nostdin', '-framerate', '1', '-loop', '1', '-i', 'frame.png', '-i', 'in.mp4',
    '-map', '0:v', '-map', '1:a', '-shortest',
  ]);
  assert.ok(args.includes('-tune') && args[args.indexOf('-tune') + 1] === 'stillimage');
  assert.ok(args.includes('-r') && args[args.indexOf('-r') + 1] === '1');
  assert.ok(args.includes('-t') && args[args.indexOf('-t') + 1] === '62.35');
  assert.deepEqual(buildFrameArgs('in.mp4', 2.5), ['-hide_banner', '-nostdin', '-ss', '2.5', '-i', 'in.mp4', '-an', '-frames:v', '1', '-update', '1', 'frame.png']);
});

test('encode args for audio only', () => {
  const s = settingsFromPreset('audio-only');
  const args = buildEncodeArgs({ input: 'in.mp4', output: 'out.m4a', probe: probe1080p, settings: s, measured });
  assert.deepEqual(args, [
    '-hide_banner', '-nostdin', '-i', 'in.mp4', '-vn', '-map', '0:a:0',
    '-af', buildAF2(s, measured),
    '-c:a', 'aac', '-b:a', '256k', '-ar', '48000', '-ac', '2',
    '-movflags', '+faststart', 'out.m4a',
  ]);
  // Only the audio-only preset drops the video, whatever the input is.
  const noVideo = { ...probe1080p, video: null };
  const dropped = buildEncodeArgs({ input: 'in.mp3', output: 'out.m4a', probe: noVideo, settings: s, measured });
  assert.ok(dropped.includes('-vn'));
  assert.ok(!dropped.includes('-c:v'));
});

test('measure and check commands', () => {
  const s = settingsFromPreset('x-audio');
  assert.deepEqual(buildMeasureArgs('in.mov', s), [
    '-hide_banner', '-nostdin', '-i', 'in.mov', '-map', '0:a:0', '-af', buildAF1(s), '-f', 'null', '-',
  ]);
  assert.deepEqual(buildCheckArgs('out.mp4'), [
    '-hide_banner', '-nostdin', '-i', 'out.mp4', '-vn', '-af', 'ebur128=peak=true', '-f', 'null', '-',
  ]);
});

test('warnings reflect X limits', () => {
  assert.deepEqual(computeWarnings({ duration: 60, sizeBytes: 10e6, video: { width: 1280, height: 720, fps: 30 } }), []);
  const w = computeWarnings({ duration: 150, sizeBytes: 600 * 1024 * 1024, video: { width: 3840, height: 2160, fps: 120 } });
  assert.equal(w.length, 4);
  assert.deepEqual(w.map((x) => x.code), ['duration', 'size', 'resolution', 'fps']);
  assert.ok(describeWarning(w[0]).startsWith('Duration is 150.0 s.'));
  // Portrait 1080x1920 is within limits (long side 1920, short side 1080).
  assert.deepEqual(computeWarnings({ duration: 30, sizeBytes: 1, video: { width: 1080, height: 1920, fps: 30 } }), []);
});

test('presets: matching and custom detection', () => {
  const s = settingsFromPreset('x-audio');
  assert.equal(matchesPreset(s, 'x-audio'), true);
  assert.equal(matchesPreset({ ...s, crf: 20 }, 'x-audio'), false);
  assert.equal(matchesPreset({ ...s, lowpass: false }, 'x-audio'), false);
  assert.equal(matchesPreset({ ...s, staticVideo: true }, 'x-audio'), true, 'static mode is not a preset knob');
  assert.equal(PRESETS.tiktok.maxW, 1080);
  assert.equal(PRESETS['x-balanced'].maxrate, 6000);
});

test('runPipeline drives a fake runner through all stages', async () => {
  const calls = [];
  const stages = [];
  const runner = {
    async exec(args, opts) {
      calls.push(args);
      if (opts?.onProgress) { opts.onProgress(1.7); opts.onProgress(NaN); }
      if (args.includes('-f')) {
        // pass 1 or check
        if (hasArg(args, 'ebur128=peak=true')) {
          return { code: 0, ms: 5, log: 'Duration: 00:01:02.35, start: 0.000000, bitrate: 900 kb/s\n  Stream #0:0: Video: h264 (High), yuv420p, 1280x720, 700 kb/s, 30 fps, 30 tbr\n  Stream #0:1: Audio: aac (LC), 48000 Hz, stereo, fltp, 256 kb/s\nSummary:\n    I:         -14.0 LUFS\n    LRA:         8.0 LU\n    Peak:       -1.0 dBFS' };
        }
        return { code: 0, ms: 5, log: '{ "input_i" : "-19.83", "input_tp" : "-0.42", "input_lra" : "9.70", "input_thresh" : "-30.31", "target_offset" : "0.35" }' };
      }
      return { code: 0, ms: 5, log: '' };
    },
    async readFile() { return new Uint8Array([1, 2, 3]); },
    async deleteFile(name) { calls.push(['rm', name]); },
  };
  const seen = [];
  const res = await runPipeline({
    runner, inputName: 'in.mp4', probe: probe1080p, settings: settingsFromPreset('x-audio'),
    onStage: (s) => stages.push(s), onProgress: (r) => seen.push(r),
  });
  // STAGES are keys now; the UI looks the label up in STR[lang].stages.
  assert.deepEqual(stages, ['measure', 'encode', 'check']);
  assert.equal(calls.length, 4); // measure, encode, check, rm out.mp4
  assert.equal(res.outputName, 'out.mp4');
  assert.equal(res.linear, true);
  assert.equal(res.ebur.I, -14);
  assert.equal(res.after.video.width, 1280);
  assert.deepEqual(res.warnings, []);
  assert.ok(seen.every((r) => r >= 0 && r <= 1), 'progress is clamped to [0,1]');
});

test('runPipeline surfaces a non-zero encode exit code', async () => {
  const runner = {
    async exec(args) {
      if (hasArg(args, 'print_format=json')) return { code: 0, ms: 1, log: '{ "input_i" : "-19", "input_tp" : "-1", "input_lra" : "9", "input_thresh" : "-30", "target_offset" : "0" }' };
      return { code: 234, ms: 1, log: 'boom' };
    },
    async readFile() { return new Uint8Array(0); },
    async deleteFile() {},
  };
  await assert.rejects(
    runPipeline({ runner, inputName: 'in.mp4', probe: probe1080p, settings: settingsFromPreset('x-audio') }),
    /exit code 234/,
  );
});

/* ---------- Audio input + a still picture ---------- */

/** ffmpeg filter chains arrive as one long argument, so match inside the strings. */
const hasArg = (args, needle) => args.some((a) => String(a).includes(needle));

const probeAudio = {
  duration: 191.2, bitrate: 320,
  video: null,
  audio: { codec: 'mp3', sampleRate: 44100, channels: 2, bitrate: 320 },
  rotation: 0,
};

test('picture mode is on for an audio input unless the preset drops the video', () => {
  assert.equal(isPictureMode(probeAudio, settingsFromPreset('x-audio')), true);
  assert.equal(isPictureMode(probeAudio, settingsFromPreset('tiktok')), true);
  assert.equal(isPictureMode(probeAudio, settingsFromPreset('audio-only')), false, 'audio-only stays audio-only');
  assert.equal(isPictureMode(probe1080p, settingsFromPreset('x-audio')), false, 'a video input is not picture mode');
  // A box typed out of range leaves nothing to pad the picture into.
  assert.equal(isPictureMode(probeAudio, { ...settingsFromPreset('x-audio'), maxW: 0 }), false);
});

test('cover filter fits the picture in the box and pads the rest black', () => {
  assert.equal(
    buildCoverFilter({ w: 1280, h: 720 }),
    'scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2:color=black',
  );
  assert.equal(
    buildCoverFilter({ w: 1080, h: 1920 }),
    'scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2:color=black',
  );
});

test('encode args for an audio input with a picture', () => {
  const s = { ...settingsFromPreset('x-audio'), bufsize: 3600 };
  const args = buildEncodeArgs({ input: 'in.mp3', output: 'out.mp4', probe: probeAudio, settings: s, measured, cover: 'cover.jpg' });
  assert.deepEqual(args, [
    '-hide_banner', '-nostdin',
    '-framerate', '1', '-loop', '1', '-i', 'cover.jpg',
    '-i', 'in.mp3',
    '-map', '0:v', '-map', '1:a', '-shortest', '-t', '191.2',
    '-vf', 'scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2:color=black',
    '-c:v', 'libx264', '-preset', 'veryfast', '-tune', 'stillimage',
    '-profile:v', 'high', '-level', '4.1', '-pix_fmt', 'yuv420p',
    '-crf', '26', '-maxrate', '1800k', '-bufsize', '3600k', '-r', '1',
    '-af', buildAF2(s, measured),
    '-c:a', 'aac', '-b:a', '320k', '-ar', '48000', '-ac', '2',
    '-movflags', '+faststart', 'out.mp4',
  ]);
});

test('encode args for an audio input with no picture: a generated black frame', () => {
  const s = { ...settingsFromPreset('tiktok'), bufsize: 16000 };
  const args = buildEncodeArgs({ input: 'in.wav', output: 'out.mp4', probe: probeAudio, settings: s, measured, cover: null });
  assert.deepEqual(args, [
    '-hide_banner', '-nostdin',
    '-f', 'lavfi', '-i', 'color=c=black:s=1080x1920:r=1',
    '-i', 'in.wav',
    '-map', '0:v', '-map', '1:a', '-shortest', '-t', '191.2',
    '-vf', 'scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(ow-iw)/2:(oh-ih)/2:color=black',
    '-c:v', 'libx264', '-preset', 'veryfast', '-tune', 'stillimage',
    '-profile:v', 'high', '-level', '4.1', '-pix_fmt', 'yuv420p',
    '-crf', '23', '-maxrate', '8000k', '-bufsize', '16000k', '-r', '1',
    '-af', buildAF2(s, measured),
    '-c:a', 'aac', '-b:a', '256k', '-ar', '48000', '-ac', '2',
    '-movflags', '+faststart', 'out.mp4',
  ]);
  // No -t when the duration is unknown; -shortest still ends the encode with the audio.
  const noDur = buildEncodeArgs({ input: 'in.wav', output: 'out.mp4', probe: { ...probeAudio, duration: NaN }, settings: s, measured });
  assert.ok(!noDur.includes('-t'));
  assert.ok(noDur.includes('-shortest'));
});

test('the still-frame path for video inputs is unchanged', () => {
  const s = { ...settingsFromPreset('x-audio'), staticVideo: true, staticTime: 2.5 };
  const args = buildEncodeArgs({ input: 'in.mp4', output: 'out.mp4', probe: probe1080p, settings: s, measured, frame: 'frame.png', cover: 'cover.jpg' });
  assert.deepEqual(args.slice(0, 10), [
    '-hide_banner', '-nostdin', '-framerate', '1', '-loop', '1', '-i', 'frame.png', '-i', 'in.mp4',
  ], 'a video input still uses the extracted frame, never the cover picture');
  assert.equal(args[args.indexOf('-vf') + 1], 'scale=1280:720,fps=24', 'and the normal scale + fps filter');
});

test('runPipeline writes the picture, encodes an mp4, then deletes it', async () => {
  const written = [];
  const deleted = [];
  let encodeArgs = null;
  const runner = {
    async exec(args) {
      if (hasArg(args, 'print_format=json')) {
        return { code: 0, ms: 1, log: '{ "input_i" : "-19.83", "input_tp" : "-0.42", "input_lra" : "9.70", "input_thresh" : "-30.31", "target_offset" : "0.35" }' };
      }
      if (hasArg(args, 'ebur128=peak=true')) {
        return { code: 0, ms: 1, log: 'Duration: 00:03:11.20, start: 0.000000, bitrate: 500 kb/s\n  Stream #0:0: Video: h264 (High), yuv420p, 1280x720, 120 kb/s, 1 fps, 1 tbr\n  Stream #0:1: Audio: aac (LC), 48000 Hz, stereo, fltp, 320 kb/s\nSummary:\n    I:         -14.0 LUFS\n    LRA:         8.0 LU\n    Peak:       -1.0 dBFS' };
      }
      encodeArgs = args;
      return { code: 0, ms: 1, log: '' };
    },
    async writeFile(name, data) { written.push([name, data]); },
    async readFile() { return new Uint8Array([1, 2, 3]); },
    async deleteFile(name) { deleted.push(name); },
  };
  const stages = [];
  const res = await runPipeline({
    runner,
    inputName: 'in.mp3',
    probe: probeAudio,
    settings: settingsFromPreset('x-audio'),
    cover: { name: 'cover.jpg', data: new Uint8Array([9, 9]) },
    onStage: (s) => stages.push(s),
  });
  assert.deepEqual(written, [['cover.jpg', new Uint8Array([9, 9])]]);
  assert.deepEqual(stages, ['measure', 'encode', 'check'], 'no frame-extraction stage for an audio input');
  assert.equal(res.outputName, 'out.mp4');
  assert.equal(res.audioOnly, false);
  assert.equal(res.picture, true);
  assert.ok(encodeArgs.includes('cover.jpg'));
  assert.ok(deleted.includes('cover.jpg'), 'the picture is removed from the ffmpeg FS');
  assert.ok(deleted.includes('out.mp4'));
});

test('runPipeline falls back to a black frame when no picture was chosen', async () => {
  let encodeArgs = null;
  const runner = {
    async exec(args) {
      if (hasArg(args, 'print_format=json')) return { code: 0, ms: 1, log: '{ "input_i" : "-19", "input_tp" : "-1", "input_lra" : "9", "input_thresh" : "-30", "target_offset" : "0" }' };
      if (hasArg(args, 'ebur128=peak=true')) return { code: 0, ms: 1, log: 'Summary:\n    I:         -14.0 LUFS' };
      encodeArgs = args;
      return { code: 0, ms: 1, log: '' };
    },
    async writeFile() { throw new Error('there is nothing to write'); },
    async readFile() { return new Uint8Array([1]); },
    async deleteFile() {},
  };
  const res = await runPipeline({ runner, inputName: 'in.m4a', probe: probeAudio, settings: settingsFromPreset('x-audio') });
  assert.equal(res.picture, true);
  assert.ok(encodeArgs.includes('color=c=black:s=1280x720:r=1'));
});

test('the audio-only preset on an audio input still produces an m4a', async () => {
  const runner = {
    async exec(args) {
      if (hasArg(args, 'print_format=json')) return { code: 0, ms: 1, log: '{ "input_i" : "-19", "input_tp" : "-1", "input_lra" : "9", "input_thresh" : "-30", "target_offset" : "0" }' };
      return { code: 0, ms: 1, log: 'Summary:\n    I:         -14.0 LUFS' };
    },
    async readFile() { return new Uint8Array([1]); },
    async deleteFile() {},
  };
  const res = await runPipeline({ runner, inputName: 'in.mp3', probe: probeAudio, settings: settingsFromPreset('audio-only') });
  assert.equal(res.outputName, 'out.m4a');
  assert.equal(res.audioOnly, true);
  assert.equal(res.picture, false);
});
