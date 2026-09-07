package ui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // decoders for the formats the picker accepts
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// coverExts is what the picture picker accepts. WebP is deliberately absent:
// Go's standard library cannot decode it, so the preview tile would be blank
// even though ffmpeg would handle the file fine.
var coverExts = []string{".jpg", ".jpeg", ".png"}

// CoverCard is the "Picture" card. It appears only when the loaded input has
// no video stream and the preset still wants a video: an audio file plus a
// still picture becomes an MP4. With no picture the video is a black frame at
// the preset's box size, which X and TikTok both accept.
type CoverCard struct {
	card *widget.Card

	tileBG  *canvas.Rectangle
	tileImg *canvas.Image
	tileBox *fyne.Container

	line   *widget.Label
	hint   *widget.Label
	choose *widget.Button
	clear  *widget.Button

	path          string
	width, height int
	// boxW/boxH is the output frame the tile previews, from the preset.
	boxW, boxH int
}

// NewCoverCard builds the card. onPick opens the file dialog; onChanged is
// called after the picture is set or removed so the App can refresh the
// preset plan and the Process button.
func NewCoverCard(onPick func(), onChanged func()) *CoverCard {
	c := &CoverCard{boxW: 1280, boxH: 720}

	c.tileBG = canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 0xff})
	c.tileBG.StrokeColor = theme.Color(theme.ColorNameInputBorder)
	c.tileBG.StrokeWidth = 1
	c.tileBG.CornerRadius = 6

	c.tileImg = canvas.NewImageFromImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	// Contain, never stretch: the tile letterboxes on black exactly the way
	// the encode pads the picture into the output frame.
	c.tileImg.FillMode = canvas.ImageFillContain
	c.tileImg.Hide()

	c.tileBox = container.NewStack(c.tileBG, c.tileImg)

	c.line = widget.NewLabel("")
	c.line.Wrapping = fyne.TextWrapWord
	c.hint = widget.NewLabel("JPG or PNG. Scaled to fit the preset box, never stretched. Shown for the whole track.")
	c.hint.Wrapping = fyne.TextWrapWord
	c.hint.Importance = widget.LowImportance

	c.choose = widget.NewButtonWithIcon("Choose picture…", theme.FileImageIcon(), onPick)
	c.clear = widget.NewButtonWithIcon("Remove", theme.ContentClearIcon(), func() {
		c.Clear()
		if onChanged != nil {
			onChanged()
		}
	})
	c.clear.Hide()

	right := container.NewVBox(c.line, c.hint, container.NewHBox(c.choose, c.clear))
	body := container.NewBorder(nil, nil, container.NewPadded(c.tileBox), nil, right)

	c.card = widget.NewCard("Picture", "optional", body)
	c.card.Hide()
	c.refresh()
	return c
}

func (c *CoverCard) Widget() fyne.CanvasObject { return c.card }

// SetBox tells the card the output frame the current preset will produce, so
// the tile can preview the real shape. Call it whenever the preset changes.
func (c *CoverCard) SetBox(w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	c.boxW, c.boxH = w, h
	c.refresh()
}

// Show/Hide control visibility from the App.
func (c *CoverCard) Show() { c.card.Show() }
func (c *CoverCard) Hide() { c.card.Hide() }

// Path, Width and Height feed pipeline.Request.
func (c *CoverCard) Path() string     { return c.path }
func (c *CoverCard) Size() (int, int) { return c.width, c.height }
func (c *CoverCard) HasPicture() bool { return c.path != "" }

// Clear drops the picture and goes back to the black-frame state.
func (c *CoverCard) Clear() {
	c.path, c.width, c.height = "", 0, 0
	c.tileImg.Image = image.NewNRGBA(image.Rect(0, 0, 1, 1))
	c.tileImg.Hide()
	c.clear.Hide()
	c.choose.SetText("Choose picture…")
	c.refresh()
}

// Load reads a picture, shows it in the tile and records its size. A file the
// standard library cannot decode is rejected with a readable message rather
// than being handed to ffmpeg and failing much later.
func (c *CoverCard) Load(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	ok := false
	for _, e := range coverExts {
		if ext == e {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("%s is not a JPG or PNG", filepath.Base(path))
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("could not read %s as an image: %w", filepath.Base(path), err)
	}

	b := img.Bounds()
	c.path, c.width, c.height = path, b.Dx(), b.Dy()
	c.tileImg.Image = img
	c.tileImg.Show()
	c.tileImg.Refresh()
	c.clear.Show()
	c.choose.SetText("Change picture…")
	c.refresh()
	return nil
}

// refresh redraws the tile at the output aspect and rewrites the description.
func (c *CoverCard) refresh() {
	// Keep the tile about 120 px on its long side, in the output's shape.
	const long = 120
	w, h := float32(long), float32(long)
	if c.boxW >= c.boxH && c.boxH > 0 {
		h = long * float32(c.boxH) / float32(c.boxW)
	} else if c.boxW > 0 {
		w = long * float32(c.boxW) / float32(c.boxH)
	}
	c.tileBox.Resize(fyne.NewSize(w, h))
	c.tileBG.SetMinSize(fyne.NewSize(w, h))
	c.tileImg.SetMinSize(fyne.NewSize(w, h))

	if c.path == "" {
		c.line.SetText(fmt.Sprintf(
			"No picture yet. The video will be a black %dx%d frame, which X and TikTok accept fine.",
			c.boxW, c.boxH))
		return
	}
	c.line.SetText(fmt.Sprintf("%s · %dx%d, shown inside a %dx%d frame.",
		filepath.Base(c.path), c.width, c.height, c.boxW, c.boxH))
}
