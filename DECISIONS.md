# Decisions

Every non-obvious choice made while building audioprep, and why. Newest at
the bottom. If you disagree with one, the fix is usually a one-line change in
the file named next to it.

## Language, toolchain, layout

**Go 1.22 in go.mod, built with whatever Go is on the machine.**
Fyne v2.8 requires Go 1.22+. The dev box had Go 1.21, so the `go` directive
was set to 1.22.0 and Go's automatic toolchain download took care of the
rest. CI uses `go-version-file: go.mod` so it always matches.

**Fyne v2.8.1.** Latest stable at build time. The one API rule that matters:
since Fyne 2.6, widgets may only be touched from the Fyne goroutine, so every
background result goes through `fyne.Do`. See `internal/ui/app.go` header.

**Monorepo: desktop app at the root, browser version in `web/`.** Both
implement the same preset table and the same ffmpeg argument logic, one in
Go and one in JS. Keeping them in one repo means a preset change is one
commit and one review. The web version has its own `package.json` and
deploys to GitHub Pages from `.github/workflows/pages.yml`.

**Preset table is data, not code** (`internal/preset/preset.go`). The
pipeline reads fields; there is no `switch preset.ID` anywhere. "Custom" is
just a Preset the user edited, relabelled by `preset.AsCustom`.

## FFmpeg

**Not bundled.** ~100 MB is too much for a Go binary and it would need
rebuilding on every ffmpeg release. Instead `internal/ffmpeg/locate.go`
searches: next to the exe, then `%APPDATA%\audioprep\bin`, then PATH.
First-run download comes from BtbN's `latest` release, which is a rolling
tag with a stable asset filename, so the URL never goes stale.

**The download forces HTTP/1.1.** The first real download test stalled past
ten minutes: Go's HTTP/2 client was pulling at 0.2 to 0.3 MB/s from GitHub's
release CDN while curl on the same machine got 6.6 MB/s. Re-running with
`GODEBUG=http2client=0` brought Go to 6.0 MB/s and a 30 s download. The
cause is Go's small HTTP/2 flow-control window on a high-latency link (a
known issue). `newDownloadClient` in `download.go` clones the default
transport and empties `TLSNextProto`, which is the documented way to turn
h2 off for one client without touching anything else.

**Only ffmpeg.exe and ffprobe.exe are extracted** from the ~170 MB zip; the
rest (docs, ffplay, headers) is discarded. Extraction writes to `.part` and
renames, so a crash mid-extract can never leave a half-written binary that
`Locate` would pick up.

**Native `aac` encoder at 256k, `libfdk_aac` if present.** BtbN's GPL build
does not ship libfdk_aac (it is non-free), so the native encoder is what
almost everyone gets. At 256k stereo the native encoder is transparent for
this use case. Detection is a real `ffmpeg -encoders` parse, not a guess.

**Arguments are a `[]string`, never a shell string.** `exec.Command` passes
them straight to CreateProcess. Filenames with spaces, quotes or unicode
are safe by construction. The log panel shows a quoted version for reading
only.

**`-progress pipe:1` on stdout, stderr kept separately.** Both pipes are
drained by their own goroutine; reading one to EOF before the other
deadlocks once a pipe buffer fills. ffmpeg's `out_time_ms` field is
actually microseconds (long-standing quirk), so the parser prefers
`out_time_us`, then treats `out_time_ms` as µs, then falls back to the
`HH:MM:SS.ffffff` string. There is a test pinning this.

**Stderr is capped at 512 KB per run**, keeping the newest half when it
overflows. loudnorm's JSON, ebur128's summary and any error are all at the
tail, so nothing we parse is ever lost.

**ffprobe rotation comes from `side_data_list` first, then the legacy
`rotate` tag.** ffmpeg 5+ moved rotation into the display matrix. A phone
clip is typically stored 1920x1080 with rotation -90 and *displayed*
portrait; `FitDimensions` swaps axes for 90/270 before fitting so a
portrait cover ends up 720x1280, not squeezed into 1280x720.

**`avg_frame_rate` is preferred over `r_frame_rate`.** Phone videos are
variable-frame-rate and `r_frame_rate` can report the timebase (e.g. 90000)
rather than anything resembling the real rate.

**Cover art is not video.** ffprobe reports an MP3's embedded JPEG as a
video stream with `disposition.attached_pic=1`. It is skipped so an MP3
correctly auto-switches to the Audio-only preset.

## Audio pipeline

**Two-pass loudnorm, and the low-pass runs in BOTH passes.** If pass 1
measured the unfiltered signal and pass 2 filtered it, the measured values
would not describe what loudnorm is actually normalising, and linear mode
would land off-target. `Request.audioFilterPrefix()` is the single source
for the pre-loudnorm chain.

**`aresample=48000` after loudnorm.** loudnorm upsamples to 192 kHz
internally and outputs at that rate; without an explicit resample the
encoder would get 192 kHz input. `-ar 48000` is set too, belt and braces.

**`-ac 2` always.** Mono phone recordings are upmixed to stereo. X and
TikTok both expect stereo and some players handle mono AAC oddly.

**Silent input falls back to single-pass dynamic loudnorm.** loudnorm
prints `-inf` for digital silence and its linear mode cannot use those
numbers. `LoudnormStats.IsSilent()` detects it and `ApplyFilter` drops the
`measured_*` keys.

**Known quirk, not a bug: pure test tones get "Dynamic" mode.** ffmpeg's
loudnorm refuses linear mode when the measured LRA is exactly 0, which only
happens on a synthetic sine. Real audio always has LRA > 0 and gets linear
mode, which is the clean single-gain-change result we want. The integration
test therefore checks the output loudness, not the mode string.

**Lowpass is ON only for the X (audio-first) preset.** X's transcoder runs
audio at a bitrate that kills everything above ~16 kHz anyway. Removing it
ourselves, before loudnorm, means the encoder spends its bits on the band
that survives. For the balanced and TikTok presets it is off: those
platforms leave more headroom and a vocal cover can keep its air.

## Video pipeline

**Scale math is done in Go, not with ffmpeg expressions.** `FitDimensions`
in `internal/pipeline/scale.go` computes the exact target size from the
probe and emits a plain `scale=W:H`. The ffmpeg expression form
(`scale='min(iw,1280)':-2` with orientation logic) is hard to read, harder
to test and easy to get subtly wrong. This way it is a 40-line pure
function with a table test and a sweep test.

**Even dimensions are enforced even when no scaling is needed.** libx264
with yuv420p requires even width and height; odd sizes exist in the wild
(screen recordings). Rounding is always *down*, so we never exceed the box.

**Orientation-aware bounding box.** A preset's `MaxWidth x MaxHeight` is
written landscape. If the input is portrait the box flips, so "1280x720"
means "720p in whichever orientation you shot". This matches how X actually
treats vertical video.

**FPS cap only when needed.** The `fps=30` filter is only added when the
input's rate is above the cap. A 24 fps input stays 24 fps.

**Static-video mode uses a looped PNG as input 0** plus the original as
input 1 for audio, with `-shortest` *and* an explicit `-t <duration>`. The
loop input has no end, and `-shortest` alone has historically misbehaved
with `-loop 1`; the explicit `-t` is the safety net. `-tune stillimage`
tells x264 to spend bits on detail instead of motion.

**`-preset` is left at x264's default (medium) in the desktop app** because
CPU time is cheap on a desktop and the video budget is small anyway. The web
version uses `veryfast` because WebAssembly is 10x slower.

## Output

**Never overwrite.** `OutputPath` appends `-1`, `-2`... until the name is
free. The existence check is injected as a function so the collision test
does not touch the disk.

**Partial output is deleted on any failure or cancel.** A half-encoded MP4
next to someone's video is worse than nothing.

**Post-check failures are non-fatal.** If ebur128 cannot run on the output
(rare), the file is still good; the result table just shows dashes for
loudness. Cancellation during the check *is* fatal so Cancel always means
Cancel.

**"Before" loudness comes from pass 1.** We already measured the input to
do the normalisation, so no separate ebur128 run on the input.

## UI

**Custom widget for the drop zone, hover via desktop.Hoverable.** Fyne has
no "dashed border" primitive, so the zone is a rounded rectangle with a
2 px stroke that turns amber on hover. Real dashes would need drawing ~40
small rectangles; not worth it.

**Drag-and-drop is window-level** (`Window.SetOnDropped`) because Fyne does
not route drops to individual widgets. The drop zone is a visual target
only; dropping anywhere in the window works, which is friendlier anyway.

**Log lines are coalesced.** ffmpeg emits hundreds of stderr lines per
second. `LogPanel.Append` buffers and schedules a single `fyne.Do` flush;
if one is already pending it does nothing. The log is capped at 2000 lines.

**TextGrid for the log, not a multiline Entry.** Entry is editable and gets
slow with large text; TextGrid is built for monospace display. The trade
is no in-widget selection, so there is a Copy log button instead.

**Editing any Advanced field flips the preset dropdown to Custom** so the
output filename honestly says `_custom` instead of pretending to be a stock
preset. `Advanced.Read` returns base values for unparseable fields, so a
half-typed number never breaks an encode.

**Theme variant is forced through a wrapper theme** rather than the
deprecated `theme.DarkTheme()`. Primary colour is overridden to the same
amber as the web version.

**Config is a tiny JSON file** at `%APPDATA%\audioprep\config.json`,
written via temp-and-rename. Fyne's own Preferences API would also work
but the JSON is human-editable and the code is 60 lines.

## Windows packaging

**Icon and manifest are embedded via committed `.syso` files** generated
once with `go-winres` (`make winres`). Plain `go build` picks them up
automatically, so neither CI nor a contributor needs the `fyne` CLI. The
manifest declares the app DPI-aware and GUI-subsystem.

**`-H windowsgui -s -w`** hides the console and strips symbols. Child ffmpeg
processes get `HideWindow: true` so they do not flash a console either
(`internal/ffmpeg/hide_windows.go`).

**Local build used a portable mingw-w64 (WinLibs GCC 14.2)** because the dev
box had no C compiler and no admin rights for a package manager. CI uses
`msys2/setup-msys2` for the same toolchain. Both are documented in README.

**Cancel kills the process tree, not just the child.** `exec.CommandContext`
kills only the process it started. The first release build failed its
cancel test on the GitHub runner: ffmpeg there comes from Chocolatey, whose
`ffmpeg.exe` is a shim, so the shim died and the real encoder ran on for
another 20 s holding our pipes. Scoop shims and `.cmd` wrappers behave the
same way. `cmd.Cancel` now calls `killTree` (`taskkill /T /F` on Windows,
then `Process.Kill` as a fallback) and `cmd.WaitDelay` bounds `Wait`. A
Windows-only integration test drives the pipeline through an `ffmpeg.cmd`
wrapper to keep this honest. The `v0.1.0` tag was moved to the fixed
commit before any release existed, so no published release ever pointed at
the broken behaviour.

## Testing

**Integration tests self-skip when ffmpeg is missing** and under `-short`,
so `go test ./...` passes on any box. CI installs ffmpeg so they run for
real there.

**The cancel test asserts on wall-clock time and on the absence of a
partial file**, because "Cancel actually stops ffmpeg" was a definition-of-
done item and the only honest way to prove it is to kill a real process.

**Fixtures are captured ffmpeg output**, not hand-written. `testdata/`
holds real ffprobe JSON, a real loudnorm pass-1 stderr and a real ebur128
summary from ffmpeg 7.1, so the parsers are tested against the actual
format, quirks included.

## Web version

**ffmpeg.wasm single-threaded core.** The multithreaded core needs
SharedArrayBuffer, which needs COOP/COEP headers, which GitHub Pages cannot
set. Single-threaded works everywhere at the cost of speed. If the site
ever moves to Cloudflare Pages (headers supported), switching to `core-mt`
is a one-line change in `web/src/ffmpeg.js`.

**Core files are self-hosted, copied at build time**, not loaded from a CDN.
Some CDNs serve `.wasm` with the wrong MIME type, and a CDN outage would
break the tool. They are in `.gitignore` because `npm run build` regenerates
them.

**Deployed to GitHub Pages under `/audioprep/`** because it needs no extra
credentials or accounts beyond the repo itself. Vite `base` is set
accordingly.
