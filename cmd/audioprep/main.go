// audioprep is a small desktop tool that prepares a video for upload to X or
// TikTok with the audio treated as the priority: loudness-normalised 256k AAC
// at 48 kHz, and a deliberately cheap H.264 video so the platform's own
// transcoder has an easy job.
//
// This file only wires things together. The interesting code is in
// internal/pipeline (what ffmpeg gets asked to do), internal/ffmpeg (how it
// gets asked) and internal/ui (the window).
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/ryuken25/audioprep/assets"
	"github.com/ryuken25/audioprep/internal/ui"
)

// version is overwritten at build time with:
//
//	go build -ldflags "-X main.version=0.1.0"
//
// so the status bar shows the real release number. "dev" is what you see
// from a plain `go run`.
var version = "dev"

func main() {
	// NewWithID gives Fyne a stable identifier for this app (used for
	// preferences and, on some platforms, window grouping).
	a := app.NewWithID("com.ryuken25.audioprep")
	a.SetIcon(fyne.NewStaticResource("icon.png", assets.Icon))

	ui.New(a, version).Run()
}
