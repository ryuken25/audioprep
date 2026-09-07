package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// DropZone is the big "drop a video here" panel. Drag-and-drop itself is
// handled at the window level (Window.SetOnDropped); this widget only has to
// look inviting and react to a click.
//
// This is a custom Fyne widget: a struct embedding widget.BaseWidget plus a
// CreateRenderer method. The renderer owns the actual canvas objects.
type DropZone struct {
	widget.BaseWidget
	OnTapped func()

	title    *canvas.Text
	subtitle *canvas.Text
	border   *canvas.Rectangle
	hovering bool
}

func NewDropZone(onTapped func()) *DropZone {
	d := &DropZone{OnTapped: onTapped}
	d.ExtendBaseWidget(d) // required so Fyne calls our CreateRenderer
	return d
}

// SetText swaps the two lines, e.g. after a file is loaded.
func (d *DropZone) SetText(title, subtitle string) {
	d.title.Text = title
	d.subtitle.Text = subtitle
	d.title.Refresh()
	d.subtitle.Refresh()
}

// Tapped satisfies fyne.Tappable so a click opens the file dialog.
func (d *DropZone) Tapped(_ *fyne.PointEvent) {
	if d.OnTapped != nil {
		d.OnTapped()
	}
}

// MouseIn/MouseOut satisfy desktop.Hoverable for a highlight; they are
// declared with the desktop event type in hover.go to keep this file
// platform-neutral.

func (d *DropZone) setHover(on bool) {
	d.hovering = on
	d.applyBorder()
}

func (d *DropZone) applyBorder() {
	if d.border == nil {
		return
	}
	if d.hovering {
		d.border.StrokeColor = mauve
		d.border.FillColor = color.NRGBA{R: 0x8a, G: 0x61, B: 0x68, A: 0x2e}
	} else {
		d.border.StrokeColor = theme.Color(theme.ColorNameInputBorder)
		d.border.FillColor = color.Transparent
	}
	d.border.Refresh()
}

func (d *DropZone) CreateRenderer() fyne.WidgetRenderer {
	d.border = canvas.NewRectangle(color.Transparent)
	d.border.StrokeWidth = 2
	d.border.CornerRadius = 12
	d.applyBorder()

	d.title = canvas.NewText("Drop a video here or click to browse", theme.Color(theme.ColorNameForeground))
	d.title.TextSize = 20
	d.title.TextStyle = fyne.TextStyle{Bold: true}
	d.title.Alignment = fyne.TextAlignCenter

	d.subtitle = canvas.NewText("mp4  mov  mkv  webm  avi  ·  m4a  wav  mp3", theme.Color(theme.ColorNamePlaceHolder))
	d.subtitle.TextSize = 13
	d.subtitle.Alignment = fyne.TextAlignCenter

	icon := widget.NewIcon(theme.UploadIcon())

	text := container.NewVBox(
		container.NewCenter(icon),
		d.title,
		d.subtitle,
	)
	content := container.NewStack(d.border, container.NewCenter(text))
	return widget.NewSimpleRenderer(content)
}

// MinSize gives the zone a comfortable, obviously-droppable height.
func (d *DropZone) MinSize() fyne.Size {
	return fyne.NewSize(480, 150)
}
