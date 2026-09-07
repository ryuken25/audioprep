package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
	"github.com/ryuken25/audioprep/internal/pipeline"
)

// ResultPanel shows the before/after table and the three action buttons.
type ResultPanel struct {
	card     *widget.Card
	table    *fyne.Container
	path     *widget.Label
	warnings *fyne.Container
	openBtn  *widget.Button
	copyBtn  *widget.Button
	againBtn *widget.Button
}

func NewResultPanel(onOpen, onCopy, onAgain func()) *ResultPanel {
	r := &ResultPanel{}
	r.table = container.NewGridWithColumns(3)
	r.path = widget.NewLabel("")
	r.path.Wrapping = fyne.TextWrapBreak
	r.path.TextStyle = fyne.TextStyle{Monospace: true}
	r.warnings = container.NewVBox()

	r.openBtn = widget.NewButtonWithIcon("Open folder", theme.FolderOpenIcon(), onOpen)
	r.copyBtn = widget.NewButtonWithIcon("Copy path", theme.ContentCopyIcon(), onCopy)
	r.againBtn = widget.NewButtonWithIcon("Process another", theme.MediaReplayIcon(), onAgain)
	r.againBtn.Importance = widget.HighImportance

	body := container.NewVBox(
		r.table,
		widget.NewSeparator(),
		r.path,
		r.warnings,
		container.NewHBox(r.openBtn, r.copyBtn, r.againBtn),
	)
	r.card = widget.NewCard("Done", "", body)
	r.card.Hide()
	return r
}

func (r *ResultPanel) Widget() fyne.CanvasObject { return r.card }

func (r *ResultPanel) Hide() { r.card.Hide() }

func (r *ResultPanel) Show(res *pipeline.Result) {
	r.table.Objects = nil
	head := func(s string) fyne.CanvasObject {
		return widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	r.table.Add(head(""))
	r.table.Add(head("Before"))
	r.table.Add(head("After"))

	row := func(name, before, after string) {
		r.table.Add(widget.NewLabel(name))
		r.table.Add(widget.NewLabel(before))
		r.table.Add(widget.NewLabel(after))
	}

	b, a := res.Before, res.After
	row("Size", fmtBytes(b.Size), fmtBytes(a.Size))
	row("Duration", fmtDuration(b.Duration), fmtDuration(a.Duration))
	row("Overall bitrate", fmtBitrate(b.BitRate), fmtBitrate(a.BitRate))
	row("Video", videoDesc(b.Video), videoDesc(a.Video))
	row("Video bitrate", vbr(b.Video), vbr(a.Video))
	row("Audio", audioDesc(b.Audio), audioDesc(a.Audio))
	row("Audio bitrate", abr(b.Audio), abr(a.Audio))
	row("Loudness", lufs(res.BeforeLoudness.Integrated), lufs(res.AfterLoudness.Integrated))
	row("True peak", dbtp(res.BeforeLoudness.TruePeak), dbtp(res.AfterLoudness.TruePeak))
	row("Loudness range", lu(res.BeforeLoudness.Range), lu(res.AfterLoudness.Range))

	r.path.SetText(res.OutputPath)

	r.warnings.Objects = nil
	for _, w := range res.Warnings {
		t := canvas.NewText("⚠  "+w, color.NRGBA{R: 0xf5, G: 0x9e, B: 0x0b, A: 0xff})
		t.TextSize = 13
		r.warnings.Add(t)
	}
	r.warnings.Refresh()
	r.table.Refresh()
	r.card.Show()
}

func videoDesc(v *ffmpeg.VideoStream) string {
	if v == nil {
		return "none"
	}
	return fmt.Sprintf("%s %dx%d @ %.3g fps", strings.ToUpper(v.Codec), v.DisplayWidth(), v.DisplayHeight(), v.FPS)
}

func vbr(v *ffmpeg.VideoStream) string {
	if v == nil {
		return "—"
	}
	return fmtBitrate(v.BitRate)
}

func audioDesc(a *ffmpeg.AudioStream) string {
	if a == nil {
		return "none"
	}
	return fmt.Sprintf("%s %d Hz %s", strings.ToUpper(a.Codec), a.SampleRate, channels(a.Channels))
}

func abr(a *ffmpeg.AudioStream) string {
	if a == nil {
		return "—"
	}
	return fmtBitrate(a.BitRate)
}

func lufs(v float64) string {
	if v == 0 || v < -70 {
		return "—"
	}
	return fmt.Sprintf("%.1f LUFS", v)
}

func dbtp(v float64) string {
	if v == 0 || v < -70 {
		return "—"
	}
	return fmt.Sprintf("%.1f dBTP", v)
}

func lu(v float64) string {
	if v < 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f LU", v)
}
