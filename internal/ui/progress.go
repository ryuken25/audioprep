package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
)

// ProgressPanel is the stage label, the bar, the ETA and the Cancel button.
type ProgressPanel struct {
	box    *fyne.Container
	stage  *widget.Label
	bar    *widget.ProgressBar
	spin   *widget.ProgressBarInfinite
	detail *widget.Label
	cancel *widget.Button
}

func NewProgressPanel(onCancel func()) *ProgressPanel {
	p := &ProgressPanel{}
	p.stage = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	p.bar = widget.NewProgressBar()
	p.bar.TextFormatter = func() string { return fmt.Sprintf("%.0f%%", p.bar.Value*100) }
	p.spin = widget.NewProgressBarInfinite()
	p.spin.Hide()
	p.detail = widget.NewLabel("")
	p.cancel = widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), onCancel)

	bars := container.NewStack(p.bar, p.spin)
	p.box = container.NewVBox(
		container.NewBorder(nil, nil, p.stage, p.cancel),
		bars,
		p.detail,
	)
	p.box.Hide()
	return p
}

func (p *ProgressPanel) Widget() fyne.CanvasObject { return p.box }

// Start resets and shows the panel for a new run.
func (p *ProgressPanel) Start() {
	p.bar.SetValue(0)
	p.detail.SetText("")
	p.stage.SetText("Starting…")
	p.cancel.Enable()
	p.spin.Hide()
	p.bar.Show()
	p.box.Show()
}

func (p *ProgressPanel) Stop() {
	p.spin.Stop()
	p.box.Hide()
}

// SetStage switches to an indeterminate spinner for stages without a
// duration (still-frame extraction, post-check) and back to the bar for the
// two loudnorm passes.
func (p *ProgressPanel) SetStage(name string, determinate bool) {
	p.stage.SetText(name)
	p.bar.SetValue(0)
	p.detail.SetText("")
	if determinate {
		p.spin.Stop()
		p.spin.Hide()
		p.bar.Show()
	} else {
		p.bar.Hide()
		p.spin.Show()
		p.spin.Start()
	}
}

func (p *ProgressPanel) Update(pr ffmpeg.Progress) {
	if pr.Percent >= 0 {
		p.bar.SetValue(pr.Percent / 100)
	}
	var parts []string
	if pr.Speed > 0 {
		parts = append(parts, fmt.Sprintf("%.1fx realtime", pr.Speed))
	}
	if eta := pr.ETA(); eta > 0 {
		parts = append(parts, "ETA "+fmtETA(eta))
	}
	if pr.FPS > 0 {
		parts = append(parts, fmt.Sprintf("%.0f fps", pr.FPS))
	}
	text := ""
	for i, s := range parts {
		if i > 0 {
			text += "   ·   "
		}
		text += s
	}
	p.detail.SetText(text)
}

func (p *ProgressPanel) Cancelling() {
	p.stage.SetText("Cancelling…")
	p.cancel.Disable()
}

func fmtETA(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
}
