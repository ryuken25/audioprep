import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  buildAF1, buildAF2, buildVideoFilter, buildEncodeArgs, buildMeasureArgs, buildCheckArgs,
  buildFrameArgs, computeWarnings, runPipeline,
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
  assert.equal(buildVideoFilter(probe1080p, s).vf, 'scale=1280:720,fps=30');
  const fits = { ...probe1080p, video: { ...probe1080p.video, width: 1280, height: 720, fps: 30 } };
  assert.equal(buildVideoFilter(fits, s).vf, null);
  const portrait = { ...probe1080p, video: { ...probe1080p.video, rotation: 90, fps: 30 } };
  assert.equal(buildVideoFilter(portrait, s).vf, 'scale=720:1280');
});

test('encode args for a normal video', () => {
  const s = { ...settingsFromPreset('x-audio'), bufsize: 5000 };
  const args = buildEncodeArgs({ input: 'in.mp4', output: 'out.mp4', probe: probe1080p, settings: s, measured });
  assert.deepEqual(args, [
    '-hide_banner', '-nostdin', '-i', 'in.mp4',
    '-map', '0:v:0', '-map', '0:a:0',
    '-vf', 'scale=1280:720,fps=30',
    '-c:v', 'libx264', '-preset', 'veryfast', '-profile:v', 'high', '-level', '4.1', '-pix_fmt', 'yuv420p',
    '-crf', '23', '-maxrate', '2500k', '-bufsize', '5000k',
    '-af', buildAF2(s, measured),
    '-c:a', 'aac', '-b:a', '256k', '-ar', '48000', '-ac', '2',
    '-movflags', '+faststart', 'out.mp4',
  ]);
  assert.ok(argsToShell(args).startsWith("ffmpeg -hide_banner -nostdin -i in.mp4"));
  assert.ok(argsToShell(args).includes("'scale=1280:720,fps=30'") === false, 'plain filter strings need no quotes');
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
  // A file without a video stream is forced to audio-only regardless of preset.
  const noVideo = { ...probe1080p, video: null };
  const forced = buildEncodeArgs({ input: 'in.mp3', output: 'out.m4a', probe: noVideo, settings: settingsFromPreset('x-audio'), measured });
  assert.ok(forced.includes('-vn'));
  assert.ok(!forced.includes('-c:v'));
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
        if (args.includes('ebur128=peak=true')) {
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
  assert.deepEqual(stages, ['Measuring loudness (pass 1)', 'Encoding (pass 2)', 'Checking output']);
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
      if (args.includes('print_format=json')) return { code: 0, ms: 1, log: '{ "input_i" : "-19", "input_tp" : "-1", "input_lra" : "9", "input_thresh" : "-30", "target_offset" : "0" }' };
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
