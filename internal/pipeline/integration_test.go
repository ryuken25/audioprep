package pipeline

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
	"github.com/ryuken25/audioprep/internal/preset"
)

// TestIntegration_XPreset is the headless smoke test from the spec: generate
// a 5 s test pattern with a 440 Hz tone, run it through the X preset, and
// check the output probes as expected. It needs a real ffmpeg on PATH and is
// skipped otherwise, so `go test ./...` still passes on a bare CI box.
//
// It is also skipped under -short. Run it explicitly with:
//
//	go test ./internal/pipeline -run Integration -v
func TestIntegration_XPreset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}

	dir := t.TempDir()
	sample := filepath.Join(dir, "sample.mp4")

	// Make the sample 1080p @ 60 fps so the X preset has to both scale and
	// cap the frame rate, and use a quiet tone so loudnorm has real work.
	gen := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=5:size=1920x1080:rate=60",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5",
		"-af", "volume=-12dB",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-shortest", sample)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v\n%s", err, out)
	}

	rn := &ffmpeg.Runner{FFmpeg: ffmpegPath, FFprobe: ffprobePath, Log: func(s string) { t.Log(s) }}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	probe, err := rn.ProbeFile(ctx, sample)
	if err != nil {
		t.Fatal(err)
	}
	if !probe.HasVideo() || probe.Audio == nil {
		t.Fatalf("sample probe missing streams: %+v", probe)
	}

	var stages []string
	var lastPct float64
	res, err := Run(ctx, rn, Request{
		InputPath: sample,
		Probe:     probe,
		Preset:    preset.Default(),
		OutputDir: dir,
	}, Hooks{
		OnStage:    func(s string) { stages = append(stages, s) },
		OnProgress: func(p ffmpeg.Progress) { lastPct = p.Percent },
	})
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}

	// ---- assertions on the result -----------------------------------------
	if filepath.Base(res.OutputPath) != "sample_x-audio.mp4" {
		t.Errorf("output name = %s", filepath.Base(res.OutputPath))
	}
	if _, err := os.Stat(res.OutputPath); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	if len(stages) < 3 || stages[0] != StageMeasure || stages[len(stages)-1] != StageCheck {
		t.Errorf("stages = %v", stages)
	}
	if lastPct < 99 {
		t.Errorf("last progress = %.1f%%, expected ~100", lastPct)
	}

	out := res.After
	if out.Video == nil || out.Audio == nil {
		t.Fatalf("output probe missing streams: %+v", out)
	}
	if out.Video.Codec != "h264" || out.Video.Width != 1280 || out.Video.Height != 720 {
		t.Errorf("video = %s %dx%d, want h264 1280x720", out.Video.Codec, out.Video.Width, out.Video.Height)
	}
	if out.Video.FPS > 24.5 || out.Video.FPS < 23.5 {
		t.Errorf("fps = %.2f, want 24 (X audio-first cap)", out.Video.FPS)
	}
	if out.Video.PixFmt != "yuv420p" {
		t.Errorf("pix_fmt = %s", out.Video.PixFmt)
	}
	if out.Audio.Codec != "aac" || out.Audio.SampleRate != 48000 || out.Audio.Channels != 2 {
		t.Errorf("audio = %s %d Hz ch%d, want aac 48000 ch2", out.Audio.Codec, out.Audio.SampleRate, out.Audio.Channels)
	}
	// The native aac encoder lands under the requested 320k on a pure sine
	// (nothing to spend bits on); anything above 200k proves the flag took.
	if out.Audio.BitRate < 200_000 {
		t.Errorf("audio bitrate = %d, want >= 200k", out.Audio.BitRate)
	}
	if math.Abs(out.Duration-probe.Duration) > 0.3 {
		t.Errorf("duration drifted: in %.2f out %.2f", probe.Duration, out.Duration)
	}

	// Loudness: target -14 LUFS, tolerate 1 LU (a 5 s sine is a hard case
	// for the gating algorithm). True peak must be at or under -1 dBTP.
	if math.Abs(res.AfterLoudness.Integrated-(-14)) > 1.0 {
		t.Errorf("integrated loudness = %.2f LUFS, want -14 +/- 1", res.AfterLoudness.Integrated)
	}
	if res.AfterLoudness.TruePeak > -0.9 {
		t.Errorf("true peak = %.2f dBTP, want <= -1", res.AfterLoudness.TruePeak)
	}
	// And it must actually have changed something: the input was at -12 dB.
	if math.Abs(res.BeforeLoudness.Integrated-res.AfterLoudness.Integrated) < 1 {
		t.Errorf("before %.2f / after %.2f: loudnorm did not move the level", res.BeforeLoudness.Integrated, res.AfterLoudness.Integrated)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
}

// TestIntegration_Cancel proves the Cancel button really kills ffmpeg: start
// an encode, cancel the context almost immediately, and check we get back
// context.Canceled quickly with no output file left behind.
func TestIntegration_Cancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobePath, _ := exec.LookPath("ffprobe")

	dir := t.TempDir()
	sample := filepath.Join(dir, "long.mp4")
	// 60 s of 1080p is enough that the encode cannot finish before cancel.
	gen := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=60:size=1920x1080:rate=30",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=60",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", sample)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v\n%s", err, out)
	}

	rn := &ffmpeg.Runner{FFmpeg: ffmpegPath, FFprobe: ffprobePath}
	probe, err := rn.ProbeFile(context.Background(), sample)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	go func() {
		// Cancel as soon as the encode stage starts.
		<-started
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	t0 := time.Now()
	_, err = Run(ctx, rn, Request{InputPath: sample, Probe: probe, Preset: preset.Default(), OutputDir: dir},
		Hooks{OnStage: func(s string) {
			if s == StageEncode {
				select {
				case started <- struct{}{}:
				default:
				}
			}
		}})
	elapsed := time.Since(t0)

	if err == nil {
		t.Fatal("expected an error after cancel")
	}
	if ctx.Err() == nil {
		t.Fatal("context was not cancelled")
	}
	if elapsed > 15*time.Second {
		t.Errorf("cancel took %v; ffmpeg was not killed promptly", elapsed)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "long_x-audio.mp4")); statErr == nil {
		t.Error("partial output was left behind after cancel")
	}
}
