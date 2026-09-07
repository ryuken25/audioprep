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

**Not bundled.** 170 MB is too much for a Go binary and it would need
rebuilding on every ffmpeg release. Instead `internal/ffmpeg/locate.go`
searches: next to the exe, then `%APPDATA%\audioprep\bin`, then PATH.
First-run download comes from BtbN's `latest` release, which is a rolling
tag with a stable asset filename, so the URL never goes stale.

**The download forces HTTP/1.1.** The first real download test stalled past
ten minutes: Go's HTTP/2 client was pulling at 0.2 to 0.3 MB/s from GitHub's
release CDN while curl on the same machine got 6.6 MB/s. Re-running with
`GODEBUG=http2client=0` brought Go to 6.0 MB/s and a 30 s download. The
cause is Go's small HTTP/2 flow-control window on a high-latency link (a
known issue). `newDownloadClient` in `download.go` builds its own
`http.Transport` with a `tls.Config` that only offers `http/1.1` and an
empty `TLSNextProto`. Cloning the default transport was tried first and
failed with EOF: `Clone()` runs the default h2 setup, so the copied TLS
config still advertised h2 via ALPN, the server picked it, and the client
then spoke HTTP/1.1 on an h2 connection.

**Only ffmpeg.exe and ffprobe.exe are extracted** from the ~170 MB zip; the
rest (docs, ffplay, headers) is discarded. Extraction writes to `.part` and
renames, so a crash mid-extract can never leave a half-written binary that
`Locate` would pick up.

**Native `aac` encoder, `libfdk_aac` if present.** BtbN's GPL build does
not ship libfdk_aac (it is non-free), so the native encoder is what almost
everyone gets. At 256k and above the native encoder is transparent for this
use case. Detection is a real `ffmpeg -encoders` parse, not a guess.

**X (audio-first) in v0.2.0: 24 fps, CRF 26, 1800k cap, 320k AAC.** The
owner asked for the video to be cheaper still and the audio bitrate higher.
320k is where the AAC-LC ladder tops out; above it the native encoder
mostly pads. 24 fps drops a 30 fps phone clip to 24 with the `fps` filter
(a 24 fps input is left alone). One honest note: X gives audio a fixed
budget on its side regardless of the video, so the cheaper video mainly
buys a smaller upload, not better audio out of X. The balanced, TikTok and
audio-only presets keep 256k and 30 fps.

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
only; dropping anywhere in the window works.

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
(`internal/ffmpeg/proc_windows.go`).

**Local build used a portable mingw-w64 (WinLibs GCC 14.2)** because the dev
box had no C compiler and no admin rights for a package manager. CI uses
`msys2/setup-msys2` for the same toolchain. Both are documented in README.

**Cancel kills the whole process tree.** `exec.CommandContext`
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

## The design pass (v0.4)

The owner ran the brief in `design/prompt.md` through Claude Design and asked
for the result applied in full: "semua tampilan pake ini vrcstyle dengan bgnya
yang vrc style". The design project is imported under `design/` and
`design/from-claude-design/`; `design/tokens.css` is the authority for colour.

**VRChat skin is the default, not an option.** `<html data-skin="vrchat">`
switches every surface token to a translucent value and the panels get
`backdrop-filter: blur(22px) saturate(1.4)`. Behind them, a
`position: fixed; inset: 0` picture layer holds one of the owner's own images
(the birthday fan art by default, plus the chalkboard and shrine VRChat
captures) under a two-stop gradient. Fixed, not absolute, so opening Advanced
or a long result never moves or rescales it. Under 560 px it becomes a 760 px
band at the top with a mask fade, because a fixed cover layer on a phone
fights the address bar. `data-skin="studio"` gives the flat look back.

**Mauve is the structure, gold is the one action.** The v0.3 palette used gold
for everything accented, which made the selected preset, the focus ring and
the Process button compete. Now selection, focus, links, the drop-zone ring
and the "Output" column are the avatar's dusty mauve, and gold appears on
exactly three things: Process, Download and the progress fill.

**Fonts are self-hosted.** Sora for headings and JetBrains Mono for the log,
units and file paths, both OFL, latin + latin-ext woff2 only, 252 KB total in
`web/public/fonts`. Japanese falls through to the system Noto Sans JP rather
than shipping a CJK webfont, which would be megabytes. No Google Fonts request
at runtime; the site still makes zero third-party calls.

**Three languages, one table.** EN / ID / JP live in `web/src/i18n.js`, copied
verbatim from the design's copy pass in the owner's own voice, with the
formatter functions (eta, duration, summary, warnings) per language rather
than English sentences with substitutions. Errors stay literal in all three.
The language pill cycles and persists as `kxc.lang`, defaulting from
`navigator.language`. Preset descriptions moved out of `presets.js` into the
table; the numbers stayed in `presets.js`, because they are a product
decision and not copy.

## Audio in, MP4 out (cover mode)

**An audio file no longer forces the M4A preset.** It keeps the chosen video
preset and a Picture card appears: drop a JPG or PNG and it becomes the still
frame, or leave it empty and the video is a black frame at the preset's box.
This is what an audio-first tool should have done from the start, since X and
TikTok will not take a bare m4a.

`Request.IsCoverMode()` is true when the input has no video stream and the
preset is not audio-only. `CoverBox()` returns the output frame: the preset's
box, flipped to match the picture's orientation (a square picture keeps the
preset's own). The output is *always* exactly that box; the picture is scaled
down to fit and padded with black
(`scale=W:H:force_original_aspect_ratio=decrease,pad=W:H:(ow-iw)/2:(oh-ih)/2`).

That last choice went the other way first. Fitting the output to the picture
avoids black bars and wastes no pixels, but it makes the output size depend on
whatever art the user dropped. The design's own preview tile letterboxes on
black (`center/contain` on a black tile), so the encode matches the tile: one
predictable frame per preset. A picture smaller than the box is not enlarged;
`force_original_aspect_ratio=decrease` only shrinks, so it just sits in the
middle with wider bars.

`preset.Vertical` is new, marking TikTok as the one portrait-intent preset.
The box in the table is written landscape and normally flipped to match the
input video; with no input video and no picture, nothing else could decide.

**WebP is accepted by ffmpeg but not by the desktop picture picker.** Go's
standard library cannot decode it, so the preview tile would be blank. The web
version takes it, since the browser decodes it natively.

## Desktop, same design

**The Fyne theme is now the full colour sheet**, not a one-colour override.
`forcedTheme` answers every `ColorName` the design lists, plus sizes (padding
6, input radius 8, heading 22). A `glass` flag switches the surfaces to alpha
values so the backdrop shows through.

**Fyne cannot blur.** The VRChat skin is a `container.NewStack` of the picture
(`assets/world.jpg`, pre-cropped to the window aspect at 2x), a flat veil
rectangle and the content. Translucent surfaces over an already-soft picture
read as frosted glass; a real blur would need a custom renderer for a small
gain. The gradient in the mockup is one flat overlay here, as the design note
allows.

**The window title block is the avatar plus the product name**, matching the
web header, and the status bar gained a leading dot in success / warning /
error, mirroring the web footer.

## Web version

**ffmpeg.wasm single-threaded core.** The multithreaded core needs
SharedArrayBuffer, which needs COOP/COEP headers, which GitHub Pages cannot
set. Single-threaded works everywhere at the cost of speed. If the site
ever moves to Cloudflare Pages (headers supported), switching to `core-mt`
is a one-line change in `web/src/ffmpeg-runner.js`.

**The core is fetched with our own downloader, not `@ffmpeg/util`'s
`toBlobURL`.** The first live deploy failed to load with "body stream
already read". GitHub Pages gzips `ffmpeg-core.wasm` (10 MB on the wire
for 32 MB of content) and `toBlobURL`'s progress path compares
Content-Length with the decompressed bytes it received, decides the
download is incomplete, then re-reads a body that is already consumed.
`web/src/fetch-blob.js` streams with progress and treats Content-Length as
a hint. The real wasm size is injected by Vite at build time so the bar
still has a true total. It worked under `vite preview` because that server
does not compress. Moving to Vercel would not have helped; it gzips too.

**Core files are self-hosted, copied at build time**, not loaded from a CDN.
Some CDNs serve `.wasm` with the wrong MIME type, and a CDN outage would
break the tool. They are in `.gitignore` because `npm run build` regenerates
them.

**Deployed to GitHub Pages under `/audioprep/`** because it needs no extra
credentials or accounts beyond the repo itself. Vite `base` is set
accordingly.

**Also on Vercel as kenshi-audioprep.vercel.app.** The owner asked for
that domain. The Vercel project is git-linked with root directory `web`,
so every push to `main` builds it; `web/vercel.json` pins the commands.
Vite's `base` is picked at build time: `/` when `VERCEL=1`, `/audioprep/`
otherwise, `VITE_BASE` to override. Vercel serves the wasm core with
Brotli, which the tolerant downloader above already handles.

## Name, icon, palette (v0.3)

**Product name is Kenshi AudioPrep; the repo, Go module and binary stay
`audioprep`.** Renaming the module would touch every import and the
release links already out there, for no user-visible gain. The window
title, the web title and the resource block carry the real name.

**The app icon is the owner's avatar** (`assets/icon.png`, a rounded
square with a thin mauve border, made from `design/assets/avatar.jpg`).
The old waveform icon is kept as `assets/icon-waveform.png`. The web uses
the same picture as a round header mark linking to the X profile, and as
favicon and apple-touch-icon.

**Palette comes from the persona images, not from a template.** Dark:
near-black and dusty mauve from the avatar, gold from the Bali sunset
banner as the only accent. Light: lavender-white surfaces with deep mauve
as the accent (gold has no contrast on white). Every token lists its
source image in `design/brand.json`. The previous amber over blue-grey
produced the muddy olive selected state the owner disliked.

**Custom is a full-width strip, the four stock presets are a row.** Five
equal cards left one orphaned. Custom is also the odd one out in
behaviour (it is where you land after editing anything), so giving it a
different shape is honest, not just tidy.

**Disabled primary button is grey, not dimmed gold.** Opacity on a gold
button over a dark card read as a broken brown slab.

**`design/` is a hand-off folder, not app code.** Brief, persona, tokens,
assets and before/after screenshots for a designer or Claude Design. The
one rule in the brief that matters for engineering: element ids stay.
