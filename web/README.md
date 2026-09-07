# Kenshi X Convert (web)

Browser version of the `audioprep` encoder. Drop a video, pick a preset, click Process, download an MP4 that gives X (Twitter) or TikTok the cleanest possible input: loudness-normalized, true-peak-safe 48 kHz AAC at 256 kb/s, plus a simple, well-behaved H.264 stream.

Everything runs inside the page with [ffmpeg.wasm](https://ffmpegwasm.netlify.app/). There is no server and nothing is uploaded. X will still re-encode whatever you post. This tool maximizes the quality of the input to that re-encode, it does not bypass it.

Live: https://ryuken25.github.io/audioprep/
Desktop app (faster, no memory cap): https://github.com/ryuken25/audioprep/releases/latest

## Run locally

```sh
cd web
npm install
npm run dev
```

Open the printed URL (it includes the `/audioprep/` base path). `npm install` also copies the ffmpeg core into `public/ffmpeg/`; that folder is ignored by git and regenerated on every build.

```sh
npm test          # unit tests (node:test) for the scale math and log parsers
npm run build     # production build into web/dist
npm run preview   # serve the build locally
```

## What it does

1. Probe the file with `ffmpeg -i` and parse duration, streams, and rotation metadata.
2. Pass 1: `loudnorm` in measurement mode on the audio (optionally after a 16 kHz lowpass).
3. Pass 2: one ffmpeg run that applies linear `loudnorm` with the measured values, resamples to 48 kHz, encodes AAC 256k, and encodes video with libx264 (`-preset veryfast`, CRF 23, bitrate cap per preset, `+faststart`). Video is scaled down (never up) to the preset box, keeping aspect ratio and honoring portrait rotation, and capped at 30 fps.
4. Post-check: re-probe the output and measure it with `ebur128` so you see the actual integrated loudness and true peak next to the input values.

Presets:

| Preset | Box | Video cap | Audio | Notes |
|---|---|---|---|---|
| X (audio-first), default | 1280x720 | 2.5 Mb/s | 256k AAC, lowpass 16 kHz | warns above 140 s |
| X (balanced) | 1920x1080 | 6 Mb/s | 256k AAC | |
| TikTok / Shorts | 1080x1920 | 8 Mb/s | 256k AAC | |
| Audio only | none | none | 256k AAC in .m4a | picked automatically for audio files |
| Custom | yours | yours | yours | any edit in Advanced switches to this |

All video presets target -14 LUFS integrated, -1 dBTP true peak, LRA 11. A static-video mode (one frame plus audio, 1 fps, `-tune stillimage`) is available in Advanced for pure audio posts that still need to be a video.

## Build and deploy

`vite.config.js` sets `base: '/audioprep/'` because the site lives at `https://ryuken25.github.io/audioprep/`. The `build` script copies `ffmpeg-core.js` and `ffmpeg-core.wasm` from `node_modules/@ffmpeg/core/dist/esm/` into `public/ffmpeg/` and then runs `vite build`, so `dist/` is self-contained (about 31 MB, almost all of it the wasm core).

The single-threaded core is used on purpose. The multi-threaded one needs `SharedArrayBuffer`, which requires COOP/COEP headers that GitHub Pages cannot send.

A GitHub Actions workflow for Pages should do roughly this:

```yaml
- uses: actions/setup-node@v4
  with: { node-version: 22, cache: npm, cache-dependency-path: web/package-lock.json }
- run: npm ci
  working-directory: web
- run: npm test
  working-directory: web
- run: npm run build
  working-directory: web
- uses: actions/upload-pages-artifact@v3
  with: { path: web/dist }
- uses: actions/deploy-pages@v4
```

## Limitations

- **Speed.** Single-threaded WebAssembly is slow. Expect roughly 1x to 3x realtime for 720p with `-preset veryfast` (a 60 s clip takes one to three minutes), slower for 1080p. The desktop app is many times faster.
- **Memory.** ffmpeg.wasm is capped at 2 GB and the input file has to fit in that memory alongside the encoder. Large 1080p or 4K files may fail with an out-of-memory error. Use the desktop app for those.
- **Cancel** kills the worker and reloads the core (about 30 MB from cache), so the next run starts from scratch.
- Processing keeps the tab busy. Leave it in the foreground; some browsers throttle background tabs.

## Browser support

Needs WebAssembly, module workers, and `Blob` URLs. Verified end to end on current Chrome. Edge uses the same engine and Firefox supports the same APIs, so both are expected to work but have not been through the same test run. Safari 16.4+ should work but is untested.

## Layout

```
web/
  index.html            page structure
  src/main.js           UI wiring, state, log, download
  src/style.css         theme (dark default, light toggle)
  src/presets.js        preset table
  src/scale.js          fitDimensions(): rotation-aware, no-upscale, even sizes
  src/probe.js          ffmpeg -i log parsers
  src/loudnorm.js       loudnorm JSON + ebur128 summary parsers
  src/pipeline.js       command builders and the run orchestrator
  src/ffmpeg-runner.js  ffmpeg.wasm wrapper: load, exec with log capture, cancel
  scripts/copy-core.mjs copies the wasm core into public/ffmpeg/
  tests/*.test.mjs      node:test suites
```
