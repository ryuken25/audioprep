# audioprep

Audio-first video encoder for X (Twitter) and TikTok. Drop a video, pick a
preset, click Process, get an MP4 whose audio is as ready as it can be for
the platform's re-encode.

Comes as a Windows desktop app (Go + Fyne) and a browser version that runs
entirely on your machine (ffmpeg.wasm). Same presets, same logic.

| | |
|---|---|
| **Desktop (Windows)** | [Download the latest release](https://github.com/ryuken25/audioprep/releases/latest) |
| **Browser** | [ryuken25.github.io/audioprep](https://ryuken25.github.io/audioprep/) |

_Screenshot: coming soon. Until then the [Using the desktop app](#using-the-desktop-app) section describes the window top to bottom._

## Why this exists

X re-encodes every video you upload. So does TikTok. You cannot turn that
off, and their audio encoder is not gentle: quiet uploads come out quieter,
loud uploads get squashed, and anything above 16 kHz mostly disappears.
What you *can* do is hand them the cleanest input possible so their encoder
has nothing to fix. audioprep does exactly that: it measures your audio's
loudness, brings it to -14 LUFS with a true peak no higher than -1 dBTP
(the level every major platform normalises toward), resamples to 48 kHz
stereo, encodes it as 256 kb/s AAC, and pairs it with a small, simple
H.264 video so the platform's transcoder has an easy job and spends its
budget where you want it.

It is built for vocal covers shot on a phone, 15 seconds to a couple of
minutes, but it works on anything with an audio track.

## What the X preset actually does

The default preset, **X (audio-first)**, makes these choices:

- **Loudness**: two-pass `loudnorm` to -14 LUFS integrated, -1 dBTP true
  peak, LRA 11. Pass one measures, pass two applies a single linear gain
  change. No limiter pumping, no dynamics squashing.
- **Low-pass at 16 kHz** before normalising. X's audio bitrate throws that
  band away anyway; removing it first lets the AAC encoder spend its bits
  on what survives. (Off in the other presets.)
- **48 kHz stereo, AAC-LC at 256 kb/s.** Mono input is upmixed.
- **Video: H.264 High 4.1, CRF 23, capped at 2500 kb/s**, at most 720p in
  whichever orientation you shot, at most 30 fps, never upscaled. Cheap on
  purpose.
- **`+faststart`** so the file streams immediately.
- **Optional "static video"**: replace the whole picture with one frame at
  1 fps. Nearly all of the container is then audio. Good for pure audio
  posts.

Other presets: **X (balanced)** (1080p, 6000 kb/s, no low-pass),
**TikTok / Shorts** (vertical 1080x1920, 8000 kb/s), **Audio only** (an
`.m4a`), and **Custom** (every knob exposed). All presets are one table in
[`internal/preset/preset.go`](internal/preset/preset.go).

**Honest note:** X will still re-encode your file. audioprep maximises the
quality of what goes *in*; it does not bypass their compression, and it
cannot make their encoder use a higher bitrate than it wants to.

## Using the desktop app

1. Run `audioprep-windows-amd64.exe`. Windows SmartScreen may complain
   because the binary is not code-signed; choose *More info → Run anyway*.
2. First launch: it looks for `ffmpeg.exe` and `ffprobe.exe` next to the
   app, then in `%APPDATA%\audioprep\bin`, then on `PATH`. If none is
   found it offers to download FFmpeg (about 170 MB, once) into that
   AppData folder. Or drop your own `ffmpeg.exe` + `ffprobe.exe` next to
   the app.
3. Drop a video onto the window (or click the drop zone). The file info
   card fills in from ffprobe.
4. Pick a preset. Open **Advanced** if you want to change anything;
   editing a field turns the preset into *Custom*.
5. **Process.** Progress, speed and ETA come from ffmpeg itself. **Cancel**
   kills ffmpeg immediately and deletes the partial output.
6. The result panel shows a before/after table: size, bitrate, resolution,
   loudness, true peak. Yellow warnings appear if the output would break
   X's hard limits (512 MB, 140 s, 1920x1200, 60 fps).

Output goes next to the input as `<name>_<preset>.mp4` (or to a folder you
choose). Existing files are never overwritten; `-1`, `-2` is appended.

The **Log** section shows every ffmpeg command and its output, which is
the fastest way to learn what the tool is doing.

## Using the browser version

Open [ryuken25.github.io/audioprep](https://ryuken25.github.io/audioprep/).
Nothing is uploaded; ffmpeg runs as WebAssembly inside the tab. It is
slower than the desktop app (single-threaded wasm, expect roughly 1x to 3x
realtime for 720p) and has a 2 GB memory ceiling, so very large 1080p files
may fail. For those, use the desktop app. Details in
[`web/README.md`](web/README.md).

## Building locally on Windows

You need Go 1.22+ and a C compiler, because Fyne uses cgo for the window.

**Go:** <https://go.dev/dl/>

**C compiler**, one of:

- *Portable, no admin needed:* download a WinLibs GCC build from
  <https://winlibs.com/> (the "UCRT runtime, POSIX threads" x86_64 zip
  without LLVM is enough), unzip anywhere, and put its `mingw64\bin` on
  `PATH` for your shell session.
- *MSYS2:* install from <https://www.msys2.org/>, then in a UCRT64 shell
  run `pacman -S mingw-w64-ucrt-x86_64-gcc`.
- *Chocolatey (admin):* `choco install mingw`.

Then, from Git Bash or MSYS2:

```sh
git clone https://github.com/ryuken25/audioprep
cd audioprep
go test -short ./...          # unit tests, no ffmpeg needed
go test ./...                 # + integration tests if ffmpeg is on PATH
CGO_ENABLED=1 go build -ldflags "-H windowsgui -s -w" -o dist/audioprep.exe ./cmd/audioprep
```

Or `make build`. `-H windowsgui` is what stops a console window from
opening behind the app. The icon and version info are embedded through the
committed `cmd/audioprep/rsrc_windows_*.syso` files (regenerate with
`make winres` after changing `assets/icon.png`).

For the browser version: `cd web && npm install && npm run dev`.

## How CI builds it

- [`ci.yml`](.github/workflows/ci.yml) runs on every push: `go vet` and
  `go test` on Ubuntu (with the X11/GL headers Fyne needs to compile, and
  ffmpeg so the integration tests run for real), plus the web build and
  tests.
- [`release.yml`](.github/workflows/release.yml) runs on a `v*` tag: builds
  the `.exe` on `windows-latest` inside an MSYS2 UCRT64 shell, zips it with
  a short README, writes SHA256 sums, and attaches everything to a GitHub
  Release.
- [`pages.yml`](.github/workflows/pages.yml) deploys `web/dist` to GitHub
  Pages whenever `web/` changes on `main`.

## Go concepts used in this codebase

If you are reading the code to learn Go, these are the places where each
idea first shows up, with a comment explaining it:

| Concept | Where |
|---|---|
| Packages and `internal/` | [`internal/preset/preset.go`](internal/preset/preset.go) (the layout), any `internal/` import |
| Structs and methods | `Preset` in [`preset.go`](internal/preset/preset.go); `(v *VideoStream) DisplayWidth()` in [`probe.go`](internal/ffmpeg/probe.go) |
| The `value, ok` idiom | `preset.ByID` in [`preset.go`](internal/preset/preset.go) |
| Errors, wrapping with `%w`, `errors.Is` / `errors.As` | [`runner.go`](internal/ffmpeg/runner.go) (`RunError`, `Unwrap`), [`run.go`](internal/pipeline/run.go) (`wrapStage`) |
| Goroutines and `sync.WaitGroup` | `Runner.Run` in [`runner.go`](internal/ffmpeg/runner.go), draining two pipes |
| Channels, non-blocking send | progress channel in [`runner.go`](internal/ffmpeg/runner.go) and [`run.go`](internal/pipeline/run.go) |
| `context.Context` and cancellation | [`runner.go`](internal/ffmpeg/runner.go) (`exec.CommandContext`), `process()` in [`app.go`](internal/ui/app.go) |
| Interfaces (satisfied implicitly) | `forcedTheme` in [`theme.go`](internal/ui/theme.go); `DropZone` implementing `fyne.Tappable` in [`dropzone.go`](internal/ui/dropzone.go) |
| Build constraints (`//go:build`) | [`hide_windows.go`](internal/ffmpeg/hide_windows.go) / [`hide_other.go`](internal/ffmpeg/hide_other.go) |
| `//go:embed` | [`assets/assets.go`](assets/assets.go) |
| JSON decoding with struct tags | [`probe.go`](internal/ffmpeg/probe.go), [`config.go`](internal/config/config.go) |
| Injecting a function for testability | `OutputPath(..., exists func(string) bool)` in [`naming.go`](internal/pipeline/naming.go) |
| Table-driven tests | [`scale_test.go`](internal/pipeline/scale_test.go) |
| Golden tests | [`pipeline_test.go`](internal/pipeline/pipeline_test.go) |
| Integration tests that skip themselves | [`integration_test.go`](internal/pipeline/integration_test.go) |
| `-ldflags -X` to set a variable at build time | `version` in [`cmd/audioprep/main.go`](cmd/audioprep/main.go) |

## Layout

```
cmd/audioprep/        main.go, Windows resource files
internal/ffmpeg/      find, download and run ffmpeg; parse its output
internal/preset/      the preset table
internal/pipeline/    turn (input, preset, options) into ffmpeg jobs; run them
internal/ui/          the Fyne window
internal/config/      %APPDATA%\audioprep\config.json
assets/               icon (embedded)
tools/genicon/        draws the icon
web/                  the browser version (Vite + ffmpeg.wasm)
```

Every non-obvious decision is written down in [DECISIONS.md](DECISIONS.md).

## License

MIT. FFmpeg is downloaded separately and is licensed under the GPL by its
authors; audioprep does not link against it, it runs it as a separate
process.
