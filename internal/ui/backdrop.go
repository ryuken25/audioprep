package ui

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg" // registers the JPEG decoder for image.Decode

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/ryuken25/audioprep/assets"
)

// newBackdrop builds the VRChat skin's background: the owner's picture, dimmed
// by a flat overlay, with the app's content stacked on top.
//
// The web version blurs the picture with a CSS backdrop-filter. Fyne has no
// blur, so the picture ships pre-softened and the translucent surfaces in
// theme.go do the rest; over a calm picture the alpha alone reads as glass.
//
// Returns the stack and the two layers, so ToggleBackdrop can hide them
// without rebuilding the window.
type backdrop struct {
	stack   *fyne.Container
	picture *canvas.Image
	veil    *canvas.Rectangle
	on      bool
}

func newBackdrop(content fyne.CanvasObject, dark, on bool) *backdrop {
	b := &backdrop{on: on}

	img, _, err := image.Decode(bytes.NewReader(assets.World))
	if err == nil {
		b.picture = canvas.NewImageFromImage(img)
	} else {
		// A missing or corrupt picture must never stop the app from starting.
		b.picture = canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	}
	// The file is pre-cropped to the window aspect, so stretching it fills the
	// window without distortion and costs no per-frame scaling maths.
	b.picture.FillMode = canvas.ImageFillStretch
	b.picture.Translucency = 0.2

	b.veil = canvas.NewRectangle(backdropVeil(dark))

	b.stack = container.NewStack(b.picture, b.veil, content)
	b.apply(dark)
	return b
}

// backdropVeil is the flat overlay between the picture and the UI. It is the
// average of the web version's gradient; a gradient would need a custom
// renderer for very little gain.
func backdropVeil(dark bool) color.Color {
	if dark {
		return color.NRGBA{R: 0x0b, G: 0x0b, B: 0x0f, A: 0x9e} // rgba(11,11,15,.62)
	}
	return color.NRGBA{R: 0xe6, G: 0xe6, B: 0xf0, A: 0xb8} // rgba(230,230,240,.72)
}

// SetDark restyles the veil when the theme flips.
func (b *backdrop) SetDark(dark bool) {
	b.veil.FillColor = backdropVeil(dark)
	b.apply(dark)
}

// SetOn shows or hides the picture (the "plain" background choice).
func (b *backdrop) SetOn(on, dark bool) {
	b.on = on
	b.apply(dark)
}

func (b *backdrop) apply(dark bool) {
	if b.on {
		b.picture.Show()
		b.veil.Show()
		b.picture.Translucency = 0.2
		if !dark {
			b.picture.Translucency = 0
		}
	} else {
		b.picture.Hide()
		b.veil.Hide()
	}
	b.picture.Refresh()
	b.veil.Refresh()
}

func (b *backdrop) Widget() fyne.CanvasObject { return b.stack }
