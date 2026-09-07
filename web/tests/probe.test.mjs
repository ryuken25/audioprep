import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseProbe, parseDuration, parseRotation, parseVideoStream, parseAudioStream } from '../src/probe.js';

// iPhone portrait recording: ffmpeg 5.x style with a displaymatrix side-data line.
const IPHONE = `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'in.mov':
  Metadata:
    major_brand     : qt
    minor_version   : 0
    compatible_brands: qt
    creation_time   : 2024-03-10T10:20:30.000000Z
  Duration: 00:01:23.45, start: 0.000000, bitrate: 17234 kb/s
  Stream #0:0[0x1](und): Video: hevc (Main) (hvc1 / 0x31637668), yuv420p(tv, bt709), 1920x1080 [SAR 1:1 DAR 16:9], 16987 kb/s, 29.98 fps, 29.97 tbr, 600 tbn (default)
    Metadata:
      creation_time   : 2024-03-10T10:20:30.000000Z
      handler_name    : Core Media Video
      vendor_id       : [0][0][0][0]
    Side data:
      displaymatrix: rotation of -90.00 degrees
  Stream #0:1[0x2](und): Audio: aac (LC) (mp4a / 0x6134706D), 48000 Hz, stereo, fltp, 191 kb/s (default)
    Metadata:
      creation_time   : 2024-03-10T10:20:30.000000Z
      handler_name    : Core Media Audio
      vendor_id       : [0][0][0][0]
At least one output file must be specified`;

test('parses an iPhone MOV with displaymatrix rotation', () => {
  const p = parseProbe(IPHONE);
  assert.equal(p.duration, 83.45);
  assert.equal(p.bitrate, 17234);
  assert.equal(p.rotation, 90);
  assert.deepEqual(p.video, {
    index: 0, codec: 'hevc', profile: 'Main', pixFmt: 'yuv420p', width: 1920, height: 1080,
    fps: 29.98, bitrate: 16987, rotation: 90,
  });
  assert.deepEqual(p.audio, {
    index: 1, codec: 'aac', sampleRate: 48000, channels: 2, layout: 'stereo', bitrate: 191, sampleFmt: 'fltp',
  });
});

// Android MP4, ffmpeg 4.x style with the legacy rotate tag and no displaymatrix.
const ANDROID = `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'in.mp4':
  Metadata:
    major_brand     : mp42
    minor_version   : 0
    compatible_brands: isommp42
    creation_time   : 2023-11-02T08:15:00.000000Z
    com.android.version: 13
  Duration: 00:00:45.12, start: 0.000000, bitrate: 20345 kb/s
    Stream #0:0(eng): Video: h264 (High) (avc1 / 0x31637661), yuv420p, 3840x2160, 20000 kb/s, SAR 1:1 DAR 16:9, 30 fps, 30 tbr, 90k tbn, 180k tbc (default)
    Metadata:
      rotate          : 270
      creation_time   : 2023-11-02T08:15:00.000000Z
      handler_name    : VideoHandle
    Side data:
      displaymatrix: rotation of 90.00 degrees
    Stream #0:1(eng): Audio: aac (LC) (mp4a / 0x6134706D), 48000 Hz, stereo, fltp, 256 kb/s (default)
    Metadata:
      creation_time   : 2023-11-02T08:15:00.000000Z
      handler_name    : SoundHandle`;

test('parses an Android 4K MP4 with a rotate tag', () => {
  const p = parseProbe(ANDROID);
  assert.equal(p.duration, 45.12);
  assert.equal(p.video.width, 3840);
  assert.equal(p.video.height, 2160);
  assert.equal(p.video.codec, 'h264');
  assert.equal(p.video.profile, 'High');
  assert.equal(p.video.fps, 30);
  assert.equal(p.video.bitrate, 20000);
  assert.equal(p.rotation, 270);
  assert.equal(p.audio.bitrate, 256);
});

test('parseRotation understands both tag styles and normalizes', () => {
  assert.equal(parseRotation('      rotate          : 90'), 90);
  assert.equal(parseRotation('      rotate          : -90'), 270);
  assert.equal(parseRotation('      displaymatrix: rotation of -90.00 degrees'), 90);
  assert.equal(parseRotation('      displaymatrix: rotation of 180.00 degrees'), 180);
  assert.equal(parseRotation('nothing here'), 0);
});

// WebM screen recording: no bitrate on the stream, fps given only as tbr, unusual layout.
const WEBM = `Input #0, matroska,webm, from 'in.webm':
  Metadata:
    encoder         : Chrome
  Duration: N/A, start: 0.000000, bitrate: N/A
  Stream #0:0(eng): Video: vp9 (Profile 0), yuv420p(tv, bt709), 1280x720, SAR 1:1 DAR 16:9, 1k tbr, 1k tbn (default)
  Stream #0:1(eng): Audio: opus, 48000 Hz, mono, fltp (default)`;

test('handles WebM with N/A duration and tbr-only frame rate', () => {
  const p = parseProbe(WEBM);
  assert.equal(p.duration, null);
  assert.equal(p.bitrate, null);
  assert.equal(p.video.width, 1280);
  assert.equal(p.video.height, 720);
  assert.equal(p.video.codec, 'vp9');
  assert.equal(p.video.fps, 1000);
  assert.equal(p.video.bitrate, null);
  assert.equal(p.rotation, 0);
  assert.equal(p.audio.codec, 'opus');
  assert.equal(p.audio.channels, 1);
  assert.equal(p.audio.bitrate, null);
});

// MP3 with embedded cover art: the mjpeg "attached pic" must not count as a video stream.
const MP3 = `Input #0, mp3, from 'in.mp3':
  Metadata:
    title           : Cover
    artist          : Someone
  Duration: 00:03:12.60, start: 0.025057, bitrate: 320 kb/s
  Stream #0:0: Audio: mp3, 44100 Hz, stereo, fltp, 320 kb/s
    Metadata:
      encoder         : LAME3.100
  Stream #0:1: Video: mjpeg (Baseline), yuvj420p(pc, bt470bg/unknown/unknown), 500x500 [SAR 1:1 DAR 1:1], 90k tbr, 90k tbn (attached pic)`;

test('ignores attached cover art and reports audio-only', () => {
  const p = parseProbe(MP3);
  assert.equal(p.video, null);
  assert.equal(p.duration, 192.6);
  assert.equal(p.audio.codec, 'mp3');
  assert.equal(p.audio.sampleRate, 44100);
  assert.equal(p.audio.channels, 2);
  assert.equal(p.audio.bitrate, 320);
});

test('audio-first stream order and "N channels" layouts', () => {
  const text = `  Duration: 00:00:10.00, start: 0.000000, bitrate: 1536 kb/s
  Stream #0:0: Audio: pcm_s16le ([1][0][0][0] / 0x0001), 48000 Hz, 6 channels, s16, 4608 kb/s
  Stream #0:1: Video: mpeg4 (Simple Profile) (mp4v / 0x7634706D), yuv420p, 720x576 [SAR 16:15 DAR 4:3], 25 fps, 25 tbr, 25 tbn`;
  const a = parseAudioStream(text);
  assert.equal(a.channels, 6);
  assert.equal(a.sampleRate, 48000);
  const v = parseVideoStream(text);
  assert.equal(v.width, 720);
  assert.equal(v.height, 576);
  assert.equal(v.fps, 25);
  assert.equal(v.codec, 'mpeg4');
});

test('parseDuration handles hours and missing fractions', () => {
  assert.equal(parseDuration('Duration: 01:02:03.50, start'), 3723.5);
  assert.equal(parseDuration('Duration: 00:00:07, start'), 7);
  assert.equal(parseDuration('Duration: N/A'), null);
});

test('returns nulls for garbage input', () => {
  const p = parseProbe('ffmpeg version 5.1.4 Copyright (c) 2000-2023');
  assert.equal(p.duration, null);
  assert.equal(p.video, null);
  assert.equal(p.audio, null);
});
