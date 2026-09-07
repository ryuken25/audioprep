package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/ryuken25/audioprep/internal/preset"
)

// Advanced is the collapsible panel of knobs. It reads and writes a
// preset.Preset; the App owns the "current" preset and calls Load/Read.
//
// OnChanged fires whenever the user edits anything, so the App can flip the
// preset selector to "Custom". A guard flag suppresses it during Load so
// programmatic fills do not look like user edits.
type Advanced struct {
	OnChanged func()

	loudI, truePeak, lra *widget.Entry
	audioKbps            *widget.Entry
	lowpass              *widget.Check
	lowpassHz            *widget.Entry
	crf, maxrate, fpsCap *widget.Entry
	staticVideo          *widget.Check
	staticAt             *widget.Entry
	outputDir            *widget.Entry
	outputDirBtn         *widget.Button
	videoRows            []fyne.CanvasObject
	accordion            *widget.Accordion
	loading              bool
}

func NewAdvanced(onPickFolder func()) *Advanced {
	a := &Advanced{}

	num := func(placeholder string) *widget.Entry {
		e := widget.NewEntry()
		e.SetPlaceHolder(placeholder)
		e.OnChanged = func(string) { a.changed() }
		return e
	}
	a.loudI = num("-14")
	a.truePeak = num("-1.0")
	a.lra = num("11")
	a.audioKbps = num("256")
	a.lowpassHz = num("16000")
	a.crf = num("23")
	a.maxrate = num("2500k")
	a.fpsCap = num("30")
	a.staticAt = num("1.0")

	a.lowpass = widget.NewCheck("Low-pass before normalising", func(bool) { a.changed() })
	a.staticVideo = widget.NewCheck("Static video (single frame, 1 fps)", func(bool) { a.changed() })

	a.outputDir = widget.NewEntry()
	a.outputDir.SetPlaceHolder("Same folder as the input")
	a.outputDir.OnChanged = func(string) { a.changed() }
	a.outputDirBtn = widget.NewButton("Browse…", onPickFolder)

	row := func(label string, w fyne.CanvasObject) fyne.CanvasObject {
		return container.NewBorder(nil, nil, widget.NewLabel(label), nil, w)
	}
	unit := func(e *widget.Entry, u string) fyne.CanvasObject {
		return container.NewBorder(nil, nil, nil, widget.NewLabel(u), e)
	}

	audio := container.NewVBox(
		widget.NewLabelWithStyle("Audio", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(3,
			row("Loudness", unit(a.loudI, "LUFS")),
			row("True peak", unit(a.truePeak, "dBTP")),
			row("Range", unit(a.lra, "LU")),
		),
		container.NewGridWithColumns(3,
			row("AAC bitrate", unit(a.audioKbps, "kb/s")),
			a.lowpass,
			row("Cutoff", unit(a.lowpassHz, "Hz")),
		),
	)

	videoHeader := widget.NewLabelWithStyle("Video", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	videoGrid := container.NewGridWithColumns(3,
		row("CRF", a.crf),
		row("Max rate", a.maxrate),
		row("FPS cap", a.fpsCap),
	)
	staticRow := container.NewGridWithColumns(2,
		a.staticVideo,
		row("Frame at", unit(a.staticAt, "s")),
	)
	a.videoRows = []fyne.CanvasObject{videoHeader, videoGrid, staticRow}

	output := container.NewVBox(
		widget.NewLabelWithStyle("Output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, widget.NewLabel("Folder"), a.outputDirBtn, a.outputDir),
	)

	body := container.NewVBox(audio, widget.NewSeparator(), videoHeader, videoGrid, staticRow, widget.NewSeparator(), output)
	a.accordion = widget.NewAccordion(widget.NewAccordionItem("Advanced", body))
	return a
}

func (a *Advanced) Widget() fyne.CanvasObject { return a.accordion }

func (a *Advanced) changed() {
	if a.loading || a.OnChanged == nil {
		return
	}
	a.OnChanged()
}

// Load fills the form from a preset without firing OnChanged.
func (a *Advanced) Load(p preset.Preset) {
	a.loading = true
	defer func() { a.loading = false }()

	a.loudI.SetText(ftoa(p.LoudnessI))
	a.truePeak.SetText(ftoa(p.TruePeak))
	a.lra.SetText(ftoa(p.LRA))
	a.audioKbps.SetText(strconv.Itoa(p.AudioBitrateKbps))
	a.lowpass.SetChecked(p.Lowpass)
	a.lowpassHz.SetText(strconv.Itoa(p.LowpassHz))
	a.crf.SetText(strconv.Itoa(p.CRF))
	a.maxrate.SetText(p.MaxRate)
	a.fpsCap.SetText(strconv.Itoa(p.FPSCap))

	// Video knobs make no sense for audio-only output; grey them out.
	for _, obj := range a.videoRows {
		if p.AudioOnly {
			obj.Hide()
		} else {
			obj.Show()
		}
	}
}

// Read applies the form back onto a copy of base and returns it. Unparseable
// fields keep base's value, so a half-typed number never breaks the encode.
func (a *Advanced) Read(base preset.Preset) preset.Preset {
	p := base
	if v, ok := parseF(a.loudI.Text); ok {
		p.LoudnessI = v
	}
	if v, ok := parseF(a.truePeak.Text); ok {
		p.TruePeak = v
	}
	if v, ok := parseF(a.lra.Text); ok {
		p.LRA = v
	}
	if v, ok := parseI(a.audioKbps.Text); ok && v > 0 {
		p.AudioBitrateKbps = v
	}
	p.Lowpass = a.lowpass.Checked
	if v, ok := parseI(a.lowpassHz.Text); ok && v > 0 {
		p.LowpassHz = v
	}
	if v, ok := parseI(a.crf.Text); ok && v >= 0 && v <= 51 {
		p.CRF = v
	}
	if s := strings.TrimSpace(a.maxrate.Text); s != "" {
		p.MaxRate = s
		p.BufSize = doubleRate(s)
	}
	if v, ok := parseI(a.fpsCap.Text); ok && v > 0 {
		p.FPSCap = v
	}
	return p
}

// Static returns the static-video toggle and timestamp.
func (a *Advanced) Static() (bool, float64) {
	at := 1.0
	if v, ok := parseF(a.staticAt.Text); ok && v >= 0 {
		at = v
	}
	return a.staticVideo.Checked, at
}

func (a *Advanced) OutputDir() string { return strings.TrimSpace(a.outputDir.Text) }

func (a *Advanced) SetOutputDir(dir string) {
	a.loading = true
	a.outputDir.SetText(dir)
	a.loading = false
}

// SetStatic is used when restoring config.
func (a *Advanced) SetStatic(on bool, at float64) {
	a.loading = true
	a.staticVideo.SetChecked(on)
	a.staticAt.SetText(ftoa(at))
	a.loading = false
}

func (a *Advanced) Enable(on bool) {
	for _, e := range []*widget.Entry{a.loudI, a.truePeak, a.lra, a.audioKbps, a.lowpassHz, a.crf, a.maxrate, a.fpsCap, a.staticAt, a.outputDir} {
		if on {
			e.Enable()
		} else {
			e.Disable()
		}
	}
	for _, c := range []*widget.Check{a.lowpass, a.staticVideo} {
		if on {
			c.Enable()
		} else {
			c.Disable()
		}
	}
	if on {
		a.outputDirBtn.Enable()
	} else {
		a.outputDirBtn.Disable()
	}
}

// doubleRate turns "2500k" into "5000k" for the VBV buffer.
func doubleRate(rate string) string {
	s := strings.ToLower(strings.TrimSpace(rate))
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "k"):
		mult, s = 1000, strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "m"):
		mult, s = 1_000_000, strings.TrimSuffix(s, "m")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return rate
	}
	bits := v * mult * 2
	if bits >= 1_000_000 && int(bits)%1_000_000 == 0 {
		return strconv.Itoa(int(bits/1_000_000)) + "M"
	}
	return strconv.Itoa(int(bits/1000)) + "k"
}

func parseF(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil
}

func parseI(s string) (int, bool) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	return v, err == nil
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
