// Package ui is the Fyne front end. It owns the window, wires the widgets in
// the other files together, and is the only package that talks to both the
// pipeline and the user.
//
// Threading rule, once and for all: Fyne widgets may only be touched from the
// Fyne goroutine. Widget callbacks (OnTapped, OnChanged, ...) already run
// there. Anything we do in our own goroutines (probing, encoding,
// downloading) must hand UI updates back with fyne.Do(func() { ... }).
package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ryuken25/audioprep/assets"
	"github.com/ryuken25/audioprep/internal/config"
	"github.com/ryuken25/audioprep/internal/ffmpeg"
	"github.com/ryuken25/audioprep/internal/pipeline"
	"github.com/ryuken25/audioprep/internal/preset"
)

// acceptedExts is what the file dialog filters on. Dropped files skip the
// filter and go straight to ffprobe, which is the real authority.
var acceptedExts = []string{".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4a", ".wav", ".mp3"}

// App holds all UI state. One instance per process.
type App struct {
	fyneApp fyne.App
	win     fyne.Window
	version string
	cfg     config.Config

	runner  *ffmpeg.Runner // nil until ffmpeg has been located or downloaded
	encoder string         // "aac" or "libfdk_aac"

	probe      *ffmpeg.Probe
	stock      preset.Preset // the preset chosen in the dropdown
	current    preset.Preset // stock plus any Advanced edits
	cancel     context.CancelFunc
	busy       bool
	lastOutput string

	drop       *DropZone
	info       *InfoCard
	cover      *CoverCard
	presetSel  *widget.Select
	presetDesc *widget.Label
	adv        *Advanced
	processBtn *widget.Button
	progress   *ProgressPanel
	result     *ResultPanel
	logPanel   *LogPanel
	status     *widget.Label
	statusDot  *canvas.Circle
	themeBtn   *widget.Button
	skinBtn    *widget.Button
	backdrop   *backdrop
}

// New builds the window but does not show it. version is baked in by the
// build (see cmd/audioprep/main.go).
func New(fyneApp fyne.App, version string) *App {
	a := &App{fyneApp: fyneApp, version: version}

	cfg, err := config.Load()
	if err != nil {
		// A corrupt config is not worth bothering the user about; log it and
		// start fresh.
		fmt.Println("config:", err)
	}
	a.cfg = cfg

	dark := cfg.Theme != "light"
	glass := cfg.Skin != "studio" // the VRChat skin is the default
	fyneApp.Settings().SetTheme(newTheme(dark, glass))

	a.win = fyneApp.NewWindow("Kenshi AudioPrep")
	a.win.Resize(fyne.NewSize(760, 820))
	a.win.CenterOnScreen()
	a.buildUI()
	a.restoreConfig()
	return a
}

// Run shows the window and blocks until it closes.
func (a *App) Run() {
	a.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 || a.busy {
			return
		}
		a.loadFile(uris[0].Path())
	})
	go a.bootstrapFFmpeg()
	a.win.ShowAndRun()
}

// ---------------------------------------------------------------------------
// layout

func (a *App) buildUI() {
	a.drop = NewDropZone(a.browse)
	a.info = NewInfoCard()
	a.cover = NewCoverCard(a.pickCover, a.onCoverChanged)

	a.presetSel = widget.NewSelect(preset.Names(), a.onPresetSelected)
	a.presetDesc = widget.NewLabel("")
	a.presetDesc.Wrapping = fyne.TextWrapWord
	presetRow := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabelWithStyle("Preset", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, a.presetSel),
		a.presetDesc,
	)

	a.adv = NewAdvanced(a.pickOutputDir)
	a.adv.OnChanged = a.onAdvancedChanged

	a.processBtn = widget.NewButtonWithIcon("Process", theme.MediaPlayIcon(), a.process)
	a.processBtn.Importance = widget.HighImportance
	a.processBtn.Disable()

	a.progress = NewProgressPanel(a.cancelRun)
	a.result = NewResultPanel(a.openFolder, a.copyPath, a.processAnother)
	a.logPanel = NewLogPanel(func(text string) { a.fyneApp.Clipboard().SetContent(text) })

	a.status = widget.NewLabel("Looking for FFmpeg…")
	a.status.Truncation = fyne.TextTruncateEllipsis
	a.themeBtn = widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), a.toggleTheme)
	versionLbl := widget.NewLabel("v" + a.version)
	statusBar := container.NewBorder(widget.NewSeparator(), nil, nil, container.NewHBox(versionLbl, a.themeBtn), a.status)

	title := widget.NewRichTextFromMarkdown("## Kenshi AudioPrep\nAudio-first video encoder for X and TikTok. Loudness-normalised high-bitrate AAC, cheap H.264, ready to upload.")

	body := container.NewVBox(
		title,
		a.drop,
		a.info.Widget(),
		a.cover.Widget(),
		presetRow,
		a.adv.Widget(),
		container.NewGridWithColumns(3, layoutSpacer(), a.processBtn, layoutSpacer()),
		a.progress.Widget(),
		a.result.Widget(),
		a.logPanel.Widget(),
	)
	scroll := container.NewVScroll(container.NewPadded(body))
	// The VRChat skin puts the owner's picture behind everything; the
	// translucent surfaces in theme.go let it show through.
	a.backdrop = newBackdrop(scroll, isDark(a.fyneApp.Settings().Theme()), a.cfg.Skin != "studio")
	a.win.SetContent(container.NewBorder(nil, statusBar, nil, nil, a.backdrop.Widget()))
}

func layoutSpacer() fyne.CanvasObject { return widget.NewLabel("") }

func (a *App) restoreConfig() {
	p, ok := preset.ByID(a.cfg.LastPreset)
	if !ok {
		p = preset.Default()
	}
	a.presetSel.SetSelected(p.Name) // triggers onPresetSelected
	if a.cfg.OutputDir != "" {
		a.adv.SetOutputDir(a.cfg.OutputDir)
	}
}

func (a *App) saveConfig() {
	a.cfg.LastPreset = a.stock.ID
	a.cfg.OutputDir = a.adv.OutputDir()
	if err := config.Save(a.cfg); err != nil {
		a.logPanel.Append("config: " + err.Error())
	}
}

// ---------------------------------------------------------------------------
// ffmpeg bootstrap (runs in its own goroutine)

func (a *App) bootstrapFFmpeg() {
	ffmpegPath, ffprobePath, err := ffmpeg.Locate()
	if err == nil {
		a.useFFmpeg(ffmpegPath, ffprobePath)
		return
	}

	fyne.Do(func() {
		a.setStatus("FFmpeg not found.")
		dialog.ShowConfirm("FFmpeg is required",
			"audioprep needs FFmpeg (about 170 MB) to encode video.\n\nDownload it now into your user folder?\nIt is only downloaded once.",
			func(yes bool) {
				if !yes {
					a.setStatus("FFmpeg not found. Put ffmpeg.exe and ffprobe.exe next to audioprep.exe and restart.")
					return
				}
				go a.downloadFFmpeg()
			}, a.win)
	})
}

func (a *App) downloadFFmpeg() {
	dest, err := ffmpeg.AppDataBinDir()
	if err != nil {
		fyne.Do(func() { a.showError("Could not find a folder to install FFmpeg into.", err) })
		return
	}

	bar := widget.NewProgressBar()
	label := widget.NewLabel("Connecting…")
	ctx, cancel := context.WithCancel(context.Background())
	var dlg dialog.Dialog

	fyne.Do(func() {
		content := container.NewVBox(label, bar)
		content.Resize(fyne.NewSize(400, 80))
		dlg = dialog.NewCustom("Downloading FFmpeg", "Cancel", content, a.win)
		dlg.SetOnClosed(cancel)
		dlg.Show()
		a.setStatus("Downloading FFmpeg…")
	})

	err = ffmpeg.Download(ctx, dest, func(p ffmpeg.DownloadProgress) {
		fyne.Do(func() {
			switch p.Stage {
			case "extracting":
				label.SetText("Extracting…")
				bar.SetValue(1)
			default:
				if p.Total > 0 {
					bar.SetValue(float64(p.Done) / float64(p.Total))
					label.SetText(fmt.Sprintf("%s of %s", fmtBytes(p.Done), fmtBytes(p.Total)))
				} else {
					label.SetText(fmtBytes(p.Done))
				}
			}
		})
	})

	fyne.Do(func() {
		if dlg != nil {
			dlg.SetOnClosed(nil)
			dlg.Hide()
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				a.setStatus("FFmpeg download cancelled.")
				return
			}
			msg := "The FFmpeg download failed."
			if ffmpeg.IsNetworkError(err) {
				msg = "The FFmpeg download failed. Check your internet connection and restart the app to try again."
			}
			a.setStatus("FFmpeg download failed.")
			a.showError(msg, err)
			return
		}
	})
	if err != nil {
		return
	}

	ffmpegPath, ffprobePath, err := ffmpeg.Locate()
	if err != nil {
		fyne.Do(func() { a.showError("FFmpeg was downloaded but could not be found afterwards.", err) })
		return
	}
	a.useFFmpeg(ffmpegPath, ffprobePath)
}

// useFFmpeg finishes bootstrap: detect the version and the best AAC encoder,
// then enable the UI. Runs on a background goroutine.
func (a *App) useFFmpeg(ffmpegPath, ffprobePath string) {
	ctx := context.Background()
	ver, err := ffmpeg.Version(ctx, ffmpegPath)
	if err != nil {
		fyne.Do(func() { a.showError("FFmpeg was found but does not run.", err) })
		return
	}
	enc := "aac"
	if ffmpeg.HasEncoder(ctx, ffmpegPath, "libfdk_aac") {
		enc = "libfdk_aac"
	}

	rn := &ffmpeg.Runner{FFmpeg: ffmpegPath, FFprobe: ffprobePath, Log: a.logPanel.Append}
	fyne.Do(func() {
		a.runner = rn
		a.encoder = enc
		a.setStatus(fmt.Sprintf("%s  ·  audio: %s  ·  %s", ver, enc, filepath.Dir(ffmpegPath)))
		a.logPanel.Append("ffmpeg: " + ffmpegPath)
		a.logPanel.Append("ffprobe: " + ffprobePath)
		a.updateProcessEnabled()
	})
}

// ---------------------------------------------------------------------------
// file loading

func (a *App) browse() {
	if a.busy {
		return
	}
	fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		path := rc.URI().Path()
		rc.Close()
		a.loadFile(path)
	}, a.win)
	fd.SetFilter(storage.NewExtensionFileFilter(acceptedExts))
	fd.Resize(fyne.NewSize(700, 500))
	fd.Show()
}

func (a *App) loadFile(path string) {
	if a.runner == nil {
		a.showError("FFmpeg is not ready yet.", errors.New("wait for the status bar to show an ffmpeg version"))
		return
	}
	a.result.Hide()
	a.info.Clear()
	a.drop.SetText("Reading "+filepath.Base(path)+"…", "")
	a.probe = nil
	a.updateProcessEnabled()

	go func() {
		p, err := a.runner.ProbeFile(context.Background(), path)
		fyne.Do(func() {
			if err != nil {
				a.drop.SetText("Drop a video here or click to browse", "mp4  mov  mkv  webm  avi  ·  m4a  wav  mp3")
				a.showError("Could not read "+filepath.Base(path)+".", err)
				return
			}
			if p.Audio == nil {
				a.drop.SetText("Drop a video here or click to browse", "mp4  mov  mkv  webm  avi  ·  m4a  wav  mp3")
				a.showError(filepath.Base(path)+" has no audio stream.", errors.New("there is nothing to normalise"))
				return
			}
			a.probe = p
			a.info.Set(p)
			a.drop.SetText(filepath.Base(path), "Drop another file to replace it")
			a.logPanel.Append("probe: " + p.Summary())

			// An audio input keeps the chosen video preset: the Picture card
			// turns it into an MP4 with a still frame. "Audio only" is still
			// one click away in the preset list.
			a.updateCoverCard()
			a.updateProcessEnabled()
		})
	}()
}

// updateCoverCard shows the Picture card exactly when it applies: the loaded
// input has no video stream and the preset still wants a video.
func (a *App) updateCoverCard() {
	if a.probe == nil || a.probe.HasVideo() || a.stock.AudioOnly {
		a.cover.Hide()
		return
	}
	req := pipeline.Request{Probe: a.probe, Preset: a.current}
	w, h := req.CoverBox()
	if a.cover.HasPicture() {
		cw, ch := a.cover.Size()
		req.CoverWidth, req.CoverHeight = cw, ch
		w, h = req.CoverBox()
	}
	a.cover.SetBox(w, h)
	a.cover.Show()
}

func (a *App) pickCover() {
	fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil || rc == nil {
			return
		}
		path := rc.URI().Path()
		rc.Close()
		if err := a.cover.Load(path); err != nil {
			a.showError("That picture could not be used.", err)
			return
		}
		a.onCoverChanged()
	}, a.win)
	fd.SetFilter(storage.NewExtensionFileFilter(coverExts))
	fd.Resize(fyne.NewSize(700, 500))
	fd.Show()
}

func (a *App) onCoverChanged() {
	a.updateCoverCard()
	a.logPanel.Append("picture: " + coverLogLine(a.cover))
}

func coverLogLine(c *CoverCard) string {
	if !c.HasPicture() {
		return "none (black frame)"
	}
	w, h := c.Size()
	return fmt.Sprintf("%s (%dx%d)", c.Path(), w, h)
}

// ---------------------------------------------------------------------------
// presets

func (a *App) onPresetSelected(name string) {
	p, ok := preset.ByName(name)
	if !ok {
		return
	}
	if p.ID == preset.IDCustom && a.stock.ID != "" && a.stock.ID != preset.IDCustom {
		// User picked Custom by hand: keep whatever they had, just relabel.
		a.current = preset.AsCustom(a.current)
	} else {
		a.current = p
	}
	a.stock = p
	a.presetDesc.SetText(p.Description)
	a.adv.Load(a.current)
	a.updateCoverCard()
	a.saveConfig()
	a.updateProcessEnabled()
}

// onAdvancedChanged runs on every keystroke in the Advanced panel. Reading the
// form is cheap, so we just do it and see whether anything differs from the
// stock preset; if so the dropdown flips to Custom (silently, via the guard
// on Select's OnChanged).
func (a *App) onAdvancedChanged() {
	a.current = a.adv.Read(a.stock)
	if a.stock.ID != preset.IDCustom && a.current != a.stock {
		a.current = preset.AsCustom(a.current)
		a.stock = a.current
		prev := a.presetSel.OnChanged
		a.presetSel.OnChanged = nil
		a.presetSel.SetSelected(a.current.Name)
		a.presetSel.OnChanged = prev
		a.presetDesc.SetText(a.current.Description)
	}
	a.saveConfig()
}

func (a *App) pickOutputDir() {
	fd := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
		if err != nil || lu == nil {
			return
		}
		a.adv.SetOutputDir(lu.Path())
		a.saveConfig()
	}, a.win)
	fd.Resize(fyne.NewSize(700, 500))
	fd.Show()
}

// ---------------------------------------------------------------------------
// processing

func (a *App) updateProcessEnabled() {
	if a.runner != nil && a.probe != nil && !a.busy {
		a.processBtn.Enable()
	} else {
		a.processBtn.Disable()
	}
}

func (a *App) setBusy(b bool) {
	a.busy = b
	a.adv.Enable(!b)
	a.presetSel.Enable()
	if b {
		a.presetSel.Disable()
	}
	a.updateProcessEnabled()
}

func (a *App) process() {
	if a.probe == nil || a.runner == nil || a.busy {
		return
	}
	static, at := a.adv.Static()
	req := pipeline.Request{
		InputPath:    a.probe.Path,
		Probe:        a.probe,
		Preset:       a.adv.Read(a.stock),
		StaticVideo:  static,
		StaticAt:     at,
		OutputDir:    a.adv.OutputDir(),
		AudioEncoder: a.encoder,
	}
	if req.IsCoverMode() && a.cover.HasPicture() {
		req.CoverPath = a.cover.Path()
		req.CoverWidth, req.CoverHeight = a.cover.Size()
	}

	for _, w := range pipeline.Warnings(a.probe, req.Preset) {
		a.logPanel.Append("warning: " + w)
	}

	// context.WithCancel gives us a ctx to pass down and a function that
	// cancels it. The Cancel button calls that function; every ffmpeg
	// process started with this ctx dies immediately.
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.setBusy(true)
	a.result.Hide()
	a.progress.Start()
	a.setStatus("Processing " + filepath.Base(req.InputPath) + "…")

	hooks := pipeline.Hooks{
		OnStage: func(name string) {
			fyne.Do(func() {
				determinate := name == pipeline.StageMeasure || name == pipeline.StageEncode
				a.progress.SetStage(name, determinate)
			})
		},
		OnProgress: func(p ffmpeg.Progress) {
			fyne.Do(func() { a.progress.Update(p) })
		},
	}

	go func() {
		res, err := pipeline.Run(ctx, a.runner, req, hooks)
		cancel()
		fyne.Do(func() {
			a.cancel = nil
			a.setBusy(false)
			a.progress.Stop()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					a.setStatus("Cancelled.")
					return
				}
				a.setStatus("Failed.")
				a.showError(humanError(err), err)
				return
			}
			a.setStatus("Done: " + filepath.Base(res.OutputPath))
			a.result.Show(res)
			a.lastOutput = res.OutputPath
		})
	}()
}

func (a *App) cancelRun() {
	if a.cancel != nil {
		a.progress.Cancelling()
		a.cancel()
	}
}

// ---------------------------------------------------------------------------
// result actions

func (a *App) openFolder() {
	if a.lastOutput == "" {
		return
	}
	if err := revealInFolder(a.lastOutput); err != nil {
		a.showError("Could not open the folder.", err)
	}
}

func (a *App) copyPath() {
	if a.lastOutput != "" {
		a.fyneApp.Clipboard().SetContent(a.lastOutput)
		a.setStatus("Path copied.")
	}
}

func (a *App) processAnother() {
	a.result.Hide()
	a.info.Clear()
	a.probe = nil
	a.drop.SetText("Drop a video here or click to browse", "mp4  mov  mkv  webm  avi  ·  m4a  wav  mp3")
	a.updateProcessEnabled()
	a.setStatus("Ready.")
}

// ---------------------------------------------------------------------------
// misc

func (a *App) toggleTheme() {
	dark := !isDark(a.fyneApp.Settings().Theme())
	glass := isGlass(a.fyneApp.Settings().Theme())
	a.fyneApp.Settings().SetTheme(newTheme(dark, glass))
	if dark {
		a.cfg.Theme = "dark"
	} else {
		a.cfg.Theme = "light"
	}
	if a.backdrop != nil {
		a.backdrop.SetDark(dark)
	}
	a.saveConfig()
}

// toggleSkin flips between the VRChat skin (picture behind frosted panels) and
// the flat studio look.
func (a *App) toggleSkin() {
	dark := isDark(a.fyneApp.Settings().Theme())
	glass := !isGlass(a.fyneApp.Settings().Theme())
	a.fyneApp.Settings().SetTheme(newTheme(dark, glass))
	if glass {
		a.cfg.Skin = "vrchat"
	} else {
		a.cfg.Skin = "studio"
	}
	if a.backdrop != nil {
		a.backdrop.SetOn(glass, dark)
	}
	a.saveConfig()
}

// buildTitle is the header block: the avatar, the product name and one line of
// what the tool does.
func (a *App) buildTitle() fyne.CanvasObject {
	av := canvas.NewImageFromResource(fyne.NewStaticResource("icon.png", assets.Icon))
	av.FillMode = canvas.ImageFillContain
	av.SetMinSize(fyne.NewSize(48, 48))

	name := canvas.NewText("Kenshi AudioPrep", theme.Color(theme.ColorNameForeground))
	name.TextSize = theme.Size(theme.SizeNameHeadingText)
	name.TextStyle = fyne.TextStyle{Bold: true}

	sub := widget.NewLabel("Audio-first encoder for X and TikTok. Loudness-normalised high-bitrate AAC, cheap H.264, ready to upload.")
	sub.Wrapping = fyne.TextWrapWord
	sub.Importance = widget.LowImportance

	return container.NewBorder(nil, nil, container.NewPadded(av), nil,
		container.NewVBox(name, sub))
}

// setStatusDot colours the status-bar dot: green ready, gold busy, red failed.
func (a *App) setStatusDot(name fyne.ThemeColorName) {
	if a.statusDot == nil {
		return
	}
	a.statusDot.FillColor = theme.Color(name)
	a.statusDot.Refresh()
}

func (a *App) setStatus(s string) { a.status.SetText(s) }

// showError follows the spec: a human sentence first, the raw ffmpeg tail in
// a monospace box below.
func (a *App) showError(human string, err error) {
	raw := ""
	if err != nil {
		raw = err.Error()
	}
	msg := widget.NewLabel(human)
	msg.Wrapping = fyne.TextWrapWord

	var content fyne.CanvasObject = msg
	if raw != "" && raw != human {
		detail := widget.NewTextGrid()
		detail.SetText(raw)
		sc := container.NewScroll(detail)
		sc.SetMinSize(fyne.NewSize(560, 200))
		content = container.NewVBox(msg, sc)
	}
	d := dialog.NewCustom("Error", "Close", content, a.win)
	d.Show()
}

// humanError pulls the first line off a wrapped pipeline error, which is the
// stage name plus a short reason; the rest is ffmpeg's stderr tail.
func humanError(err error) string {
	first, _, _ := strings.Cut(err.Error(), "\n")
	first = strings.TrimSuffix(first, ":")
	return first
}
