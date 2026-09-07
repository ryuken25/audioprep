package ui

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
)

// InfoCard shows what ffprobe found. It is a plain two-column grid of
// label/value pairs wrapped in a Card.
type InfoCard struct {
	card   *widget.Card
	grid   *fyne.Container
	values map[string]*widget.Label
}

var infoRows = []string{"File", "Duration", "Video", "Frame rate", "Video bitrate", "Audio", "Audio bitrate", "Size"}

func NewInfoCard() *InfoCard {
	ic := &InfoCard{values: map[string]*widget.Label{}}
	ic.grid = container.NewGridWithColumns(2)
	for _, k := range infoRows {
		key := widget.NewLabelWithStyle(k, fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
		val := widget.NewLabel("—")
		val.Wrapping = fyne.TextWrapBreak
		ic.values[k] = val
		ic.grid.Add(key)
		ic.grid.Add(val)
	}
	ic.card = widget.NewCard("", "", ic.grid)
	ic.card.Hide()
	return ic
}

func (ic *InfoCard) Widget() fyne.CanvasObject { return ic.card }

func (ic *InfoCard) Clear() {
	for _, v := range ic.values {
		v.SetText("—")
	}
	ic.card.Hide()
}

func (ic *InfoCard) Set(p *ffmpeg.Probe) {
	set := func(k, v string) { ic.values[k].SetText(v) }

	set("File", filepath.Base(p.Path))
	set("Duration", fmtDuration(p.Duration))
	set("Size", fmtBytes(p.Size))

	if p.HasVideo() {
		v := p.Video
		dims := fmt.Sprintf("%s  %dx%d", v.Codec, v.DisplayWidth(), v.DisplayHeight())
		if v.Rotation != 0 {
			dims += fmt.Sprintf("  (stored %dx%d, rotated %d°)", v.Width, v.Height, v.Rotation)
		}
		set("Video", dims)
		set("Frame rate", fmt.Sprintf("%.3g fps", v.FPS))
		set("Video bitrate", fmtBitrate(v.BitRate))
	} else {
		set("Video", "none")
		set("Frame rate", "—")
		set("Video bitrate", "—")
	}

	if p.Audio != nil {
		a := p.Audio
		set("Audio", fmt.Sprintf("%s  %d Hz  %s", a.Codec, a.SampleRate, channels(a.Channels)))
		set("Audio bitrate", fmtBitrate(a.BitRate))
	} else {
		set("Audio", "none")
		set("Audio bitrate", "—")
	}
	ic.card.Show()
}

func channels(n int) string {
	switch n {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	default:
		return fmt.Sprintf("%d ch", n)
	}
}

func fmtDuration(sec float64) string {
	if sec <= 0 {
		return "unknown"
	}
	m := int(sec) / 60
	s := sec - float64(m*60)
	if m > 0 {
		return fmt.Sprintf("%d:%05.2f  (%.1f s)", m, s, sec)
	}
	return fmt.Sprintf("%.2f s", sec)
}

func fmtBytes(b int64) string {
	switch {
	case b <= 0:
		return "—"
	case b < 1024*1024:
		return fmt.Sprintf("%.0f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(b)/1024/1024)
	}
}

func fmtBitrate(bps int64) string {
	if bps <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f kb/s", float64(bps)/1000)
}
