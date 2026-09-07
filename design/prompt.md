# Kenshi AudioPrep: design brief

You are designing the visual layer for an existing, working tool. The
encoding logic, the preset values and the page structure are fixed. What
needs your work is how it looks and feels, so that it reads as something
made by kenshi2K (see `persona.md`) rather than a generic dashboard.

Two front ends share one design:

1. **Web**: a single page at https://kenshi-audioprep.vercel.app/ (also
   served at https://ryuken25.github.io/audioprep/). Vanilla HTML, CSS and
   JS. Everything runs in the browser through ffmpeg.wasm; no server.
2. **Desktop (Windows)**: the same layout in a Go + Fyne window, one window
   760x820, downloadable from GitHub Releases. Fyne can take colours,
   spacing and an icon from you, but not custom fonts or arbitrary CSS, so
   the desktop mockup is a reference the developer will approximate.

Screenshots of the web app as it looks today (v0.2) are in
`current-screens/`. `02-custom-selected-dark.jpg` is the state the owner called
ugly: the selected card gets a muddy olive fill (amber on blue-grey), a
doubled border, and sits orphaned on its own row under a four-column grid.
The disabled Process button is a dim brown slab. Fix these first.

`05` to `08` show the v0.3 quick fix that shipped while this brief was
written: persona palette, avatar in the header, Custom as a full-width
strip, a grey disabled button. Treat it as a floor, not the target. One
caveat: the full-page capture tool darkens some light-theme surfaces
(`09-...-artifact.jpg`); `08-v03-light-presets.png` is what the browser
really shows.

## What the tool does, in one paragraph

X and TikTok re-encode every upload and crush the audio of vocal covers.
The user drops a phone video, picks a preset, clicks Process, and gets an
MP4 whose audio is loudness-normalised to -14 LUFS at 320 kb/s AAC with a
deliberately cheap H.264 video, so the platform's own encoder has the
cleanest possible input. Nothing is uploaded anywhere. The owner sings
Japanese covers and built this because their covers "pecah" (got crushed)
on X.

## Brand direction

Read `persona.md` and `brand.json`. In short: near-black surfaces, a dusty
mauve taken from the avatar, one warm gold highlight taken from the Bali
sunset banner, lavender-light for the light theme. Calm, late-night,
unhurried. The avatar (`assets/avatar.jpg`) appears in the header and is
the app icon. Do not use amber or orange as the main accent (that is the
current, unloved look). Do not use stock waveform clip art.

Typography: a rounded geometric sans for headings (Outfit, Sora or Plus
Jakarta Sans), system sans for body, a real monospace for the log and file
paths. Fonts must be self-hosted or system; the site sends no third-party
requests.

Both themes are required: dark (default) and light. The toggle is in the
header.

## The web page, top to bottom

There are no routes. It is one page whose sections appear and disappear
with state. Element ids in `code` are fixed and must survive; they are
what the JavaScript looks up.

### 1. Header (`header.topbar`)
- Avatar (`a.brand-mark`, 44 px, links to https://x.com/kenshi2k_).
- Title "Kenshi AudioPrep" and subtitle "Audio-first encoder for X /
  TikTok. Runs 100% in your browser."
- Right side: "Desktop app ↗" link (`a.link-desktop`, to the latest GitHub
  release) and the theme toggle (`#theme-toggle`, icon button, sun/moon).

### 2. Drop zone (`#dropzone`)
- Large dashed target. Text: "Drop a video here or click to browse", then
  "MP4, MOV, MKV, WebM, AVI, M4A, WAV or MP3. Nothing leaves your device."
- Clicking opens the hidden `#file-input`. Dragging a file over it adds
  class `is-over` (highlight). While processing it has `is-busy` (dimmed,
  not clickable).
- After a file is loaded the zone stays but the file info card appears
  under it; dropping another file replaces the current one.

### 3. File info card (`#file-info`, hidden until a file is probed)
- `#file-name` (the filename) and `#file-info-grid`, a label/value grid:
  Size, Duration, Video (codec, WxH, fps, plus a "rotated 90°" badge when
  the phone stored it sideways), Video bitrate, Audio (codec, sample rate,
  channels), Audio bitrate.
- `#notice`: an inline notice, e.g. "No video stream found, switched to
  Audio only."

### 4. Preset (`#preset-list`, a radiogroup of five cards)
Each card is a `label.preset` wrapping a hidden radio, a `.preset-name`
and a `.preset-desc`. Exactly one is selected. States: idle, hover,
selected, focus-visible, and disabled (the whole list gets `is-disabled`
while processing).

| Card | Name | Description shown |
|---|---|---|
| x-audio (default) | X (audio-first) | 720p at 24 fps, 1.8 Mb/s cap, 320k AAC, 16 kHz lowpass. Best for vocal covers. |
| x-balanced | X (balanced) | 1080p, 6 Mb/s cap, 256k AAC. Sharper picture, bigger file. |
| tiktok | TikTok / Shorts | 1080x1920 vertical, 8 Mb/s cap, 256k AAC. |
| audio-only | Audio only (M4A) | Drops the video. Normalized 256k AAC in an .m4a container. |
| custom | Custom | Every knob exposed. Starts from whatever you last selected. |

Custom is special: editing any field in Advanced switches the selection
to Custom automatically. Design it so five cards never leave one orphaned
(five columns on wide screens, or a full-width Custom row on narrow ones).
`#preset-plan` is a small line next to the "Preset" heading that shows the
planned output once a file is loaded, e.g. "720x1280 @ 24 fps, 320k AAC".

### 5. Advanced (`details#advanced`, collapsed by default)
A `<summary>` row "Advanced" that expands to four groups of inputs.
Every input has a label and a unit tag. Changing any of them flips the
preset to Custom.

- **Loudness**: Target (LUFS, default -14), True peak (dBTP, -1),
  Loudness range (LU, 11).
- **Audio**: Bitrate (kb/s), "Lowpass filter" checkbox, Cutoff (Hz, 16000),
  "Audio only (drop video, output .m4a)" checkbox.
- **Video**: CRF, Max rate (kb/s), FPS cap, Box width (px), Box height
  (px). These are disabled when Audio only is on.
- **Static video**: "Single frame + audio (tiny file)" checkbox and
  "Frame at" (seconds, default 1). Turning this on does not flip the
  preset to Custom; it is a mode, not a preset value.

### 6. Process (`#process-btn`) and status (`#status`)
- One large primary button, full width of the content column. Disabled
  until both a file is loaded and the FFmpeg core is ready. The disabled
  state must look disabled, not broken.
- Under it, one line of status text: "Drop a file to begin.", "Ready to
  process.", "Cancelled.", or an error sentence (error styling).

### 7. Progress (`#progress`, visible only while running)
- `#stage-label`: one of "Loading FFmpeg", "Reading file", "Probing",
  "Measuring loudness (pass 1)", "Extracting still frame",
  "Encoding (pass 2)", "Checking output".
- `#progress-bar` with `#progress-fill`, `#progress-pct` (e.g. "42%") and
  `#progress-eta` (e.g. "about 1 min left").
- `#cancel-btn` ("Cancel", secondary). Cancel kills the encoder; the page
  shows "Cancelled." and is ready for another run.

### 8. Result (`#result`, visible after a successful run)
- `#result-summary`: one sentence, e.g. "Done in 48 s. -14.0 LUFS,
  -1.0 dBTP, 3.9 MB."
- `#compare`: a before/after table with rows Size, Duration, Overall
  bitrate, Video, Video bitrate, Audio, Audio bitrate, Loudness, True
  peak, Loudness range.
- `#warnings`: zero or more yellow lines, e.g. "Duration is 152.0 s. X
  allows up to 140 s for most accounts."
- Buttons: `#download-btn` ("Download", primary; the output file name is
  `<input name>_<preset id>.mp4` or `.m4a`) and `#another-btn` ("Process
  another", secondary, clears the file and result).

### 9. Log (`details#log-details`, collapsed)
- Summary "Log" with `#log-count` (line count). Inside: `#log`, a
  monospace, scrollable, read-only block showing every ffmpeg command
  (prefixed `$`) and ffmpeg's own output, and `#log-clear` ("Clear",
  tiny button).

### 10. Footer
- `#core-status` with `#core-status-text`: "FFmpeg core: loading… 12 MB /
  32 MB", "FFmpeg core: ready (32 MB, single-threaded wasm)", or an error.
  A small coloured dot shows the state (loading / ready / error).
- One honest line: "X will still re-encode your video. This maximizes
  input quality, it does not bypass compression."

### Page states to draw (each as its own artboard, dark and light)
1. First load: core loading, no file, Process disabled.
2. Ready: core ready, no file.
3. File loaded: info card filled, `#preset-plan` filled, Process enabled.
4. Custom selected with Advanced open (the state to fix).
5. Audio file dropped: notice shown, Audio only selected, video inputs
   disabled.
6. Processing: stage "Encoding (pass 2)", 42%, ETA, Cancel.
7. Done: summary, table, one warning, Download and Process another.
8. Error: status line in error style, log expanded showing the tail.
9. Mobile (360 px wide) versions of 2, 4 and 7.

## The desktop window

Same order as the web page, in a Fyne window: title block with the
avatar; drop zone (click opens a file dialog; drag-and-drop works anywhere
in the window); info card (File, Duration, Video, Frame rate, Video
bitrate, Audio, Audio bitrate, Size); a Preset dropdown with the
description under it; an "Advanced" accordion with the same knobs plus an
Output folder field and a "Browse…" button; a centred Process button; a
progress panel (stage label, bar or spinner, a detail line like
"2.4x realtime · ETA 38s · 58 fps", Cancel); a "Done" card with the
before/after grid, the output path in monospace, warnings, and three
buttons: "Open folder", "Copy path", "Process another"; a Log accordion
with "Copy log" and "Clear"; and a status bar at the bottom showing the
detected ffmpeg version, the audio encoder in use, the version number and
a theme toggle.

Dialogs the desktop app shows: "FFmpeg is required" (first run, Yes /
No, explains the 170 MB one-time download), "Downloading FFmpeg" (progress
bar, Cancel), the error dialog (one human sentence, then the raw ffmpeg
tail in monospace, Close), and the OS file and folder pickers.

Fyne constraints: system font only; colours, padding and the accent are
adjustable; widgets are standard buttons, entries, checkboxes, a select,
accordions, a progress bar and a text grid. Give the developer a colour
sheet and spacing values rather than pixel-exact chrome.

## Icon

The app icon is the avatar in a rounded square with a thin mauve border
(`../assets/icon.png` in the repo is the current version). Refine it if
you like, but it must stay recognisably the avatar at 16 px, since that is
the size in the Windows taskbar and the browser tab. Also produce an
Open Graph image, 1200x630, for link previews on X.

## Constraints that will not move

- One page, no routes, no framework, no build-time CSS tooling beyond what
  Vite does. Plain CSS with custom properties is the contract.
- Element ids listed above stay. Classes may change.
- No third-party requests: no web fonts from a CDN, no analytics, no icon
  CDN. Icons are inline SVG.
- Works at 360 px wide and at 1440 px wide; the content column is capped
  around 720 px today and can stay so.
- The log is monospace and can hold two thousand lines; it must scroll
  inside itself, never the page.
- Preset values and copy in the table above are product decisions, not
  design decisions.

## Deliverables

1. Design tokens for both themes as CSS custom properties: backgrounds,
   surfaces, borders, text, muted text, accent, accent-on-text, focus
   ring, success, warning, error, radii, spacing scale, font stacks.
2. Artboards for the nine web states above, dark and light.
3. A component sheet: preset card (idle, hover, selected, focus, disabled),
   primary and secondary buttons (idle, hover, active, disabled, busy),
   text input with unit tag, checkbox, details/summary row, progress bar,
   the before/after table, the log block, the status dot.
4. The desktop window reference and its colour sheet.
5. Icon set (512, 256, 128, 64, 32, 16), favicon, apple-touch-icon,
   Open Graph image.
6. A short copy pass in the owner's voice (see `persona.md`, "Voice") for
   the subtitle, the drop zone, the status lines, the honest footer line
   and the five preset descriptions. Keep every number.
