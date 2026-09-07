package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
)

// Stage names, in the order they run. The UI shows these above the bar.
const (
	StageMeasure = "Measuring loudness (pass 1 of 2)"
	StageStill   = "Extracting still frame"
	StageEncode  = "Encoding (pass 2 of 2)"
	StageCheck   = "Checking output"
)

// Hooks are optional callbacks Run fires as it goes. All may be nil. They
// are called from the pipeline goroutine, never the UI thread, so the UI
// must marshal them across (fyne.Do).
type Hooks struct {
	OnStage    func(name string)
	OnProgress func(p ffmpeg.Progress)
}

// Result is what Run hands back on success.
type Result struct {
	OutputPath string
	Before     *ffmpeg.Probe
	After      *ffmpeg.Probe
	// BeforeLoudness comes free from the loudnorm measurement pass, so we do
	// not run a separate ebur128 on the input.
	BeforeLoudness ffmpeg.Loudness
	AfterLoudness  ffmpeg.Loudness
	Stats          ffmpeg.LoudnormStats
	Warnings       []string
}

// Run executes the whole pipeline: measure, (still frame), encode, check.
// It is synchronous; callers run it in a goroutine and cancel via ctx.
//
// On cancellation or failure the partially written output is deleted so the
// user never finds a half-encoded file next to their video.
func Run(ctx context.Context, rn *ffmpeg.Runner, req Request, h Hooks) (*Result, error) {
	if req.Probe == nil {
		return nil, errors.New("pipeline: request has no probe")
	}
	if req.Probe.Audio == nil {
		return nil, errors.New("the input has no audio stream; nothing to normalise")
	}
	// An input without video is fine: cover mode turns it into a video with a
	// still picture (or a black frame). Only an audio-only preset skips that.

	outputPath := OutputPath(req.InputPath, req.OutputDir, req.Preset.ID, req.Preset.OutputExt, FileExists)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output folder: %w", err)
	}

	total := durationOf(req.Probe)
	progress := make(chan ffmpeg.Progress, 8)
	// A goroutine forwards progress snapshots to the hook. It exits when the
	// channel is closed at the end of Run. Using a channel here (rather than
	// calling the hook from inside the runner) keeps ffmpeg's reader goroutine
	// from ever blocking on UI code.
	go func() {
		for p := range progress {
			if h.OnProgress != nil {
				h.OnProgress(p)
			}
		}
	}()
	defer close(progress)

	stage := func(name string) {
		if h.OnStage != nil {
			h.OnStage(name)
		}
	}

	cleanupOnFail := func(err error) (*Result, error) {
		os.Remove(outputPath)
		return nil, err
	}

	// ---- pass 1: measure ---------------------------------------------------
	stage(StageMeasure)
	stderr, err := rn.Run(ctx, MeasureArgs(req), total, progress)
	if err != nil {
		return cleanupOnFail(wrapStage(StageMeasure, err))
	}
	stats, err := ffmpeg.ParseLoudnormJSON(stderr)
	if err != nil {
		return cleanupOnFail(fmt.Errorf("could not read loudness measurement: %w", err))
	}

	// ---- optional: still frame --------------------------------------------
	var framePath string
	if req.StaticVideo && !req.Preset.AudioOnly {
		stage(StageStill)
		tmp, err := os.MkdirTemp("", "audioprep-")
		if err != nil {
			return cleanupOnFail(fmt.Errorf("create temp folder: %w", err))
		}
		defer os.RemoveAll(tmp)
		framePath = filepath.Join(tmp, "frame.png")
		if _, err := rn.Run(ctx, StillFrameArgs(req, framePath), 0, nil); err != nil {
			return cleanupOnFail(wrapStage(StageStill, err))
		}
	}

	// ---- pass 2: encode ----------------------------------------------------
	stage(StageEncode)
	if _, err := rn.Run(ctx, EncodeArgs(req, stats, framePath, outputPath), total, progress); err != nil {
		return cleanupOnFail(wrapStage(StageEncode, err))
	}

	// ---- post-check --------------------------------------------------------
	stage(StageCheck)
	after, err := rn.ProbeFile(ctx, outputPath)
	if err != nil {
		return cleanupOnFail(fmt.Errorf("output written but could not be probed: %w", err))
	}
	checkErr, err := rn.Run(ctx, CheckArgs(outputPath), total, nil)
	afterLoud := ffmpeg.Loudness{}
	if err == nil {
		afterLoud, _ = ffmpeg.ParseEBUR128Summary(checkErr)
	}
	// A failed check is not fatal: the file is fine, we just cannot show
	// numbers for it. ctx cancellation during the check is the exception.
	if err != nil && ctx.Err() != nil {
		return cleanupOnFail(wrapStage(StageCheck, err))
	}

	res := &Result{
		OutputPath: outputPath,
		Before:     req.Probe,
		After:      after,
		BeforeLoudness: ffmpeg.Loudness{
			Integrated: stats.InputI,
			Range:      stats.InputLRA,
			TruePeak:   stats.InputTP,
		},
		AfterLoudness: afterLoud,
		Stats:         stats,
		Warnings:      append(Warnings(req.Probe, req.Preset), PostWarnings(after)...),
	}
	return res, nil
}

// wrapStage prefixes an error with the stage name unless it was a cancel,
// which the UI reports as a plain "Cancelled".
func wrapStage(stage string, err error) error {
	if errors.Is(err, context.Canceled) {
		return err
	}
	var re *ffmpeg.RunError
	if errors.As(err, &re) && re.Killed {
		return context.Canceled
	}
	return fmt.Errorf("%s: %w", stage, err)
}
