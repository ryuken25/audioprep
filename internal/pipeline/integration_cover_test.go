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

// TestIntegration_CoverMode runs the real thing an audio-only upload needs: an
// m4a plus a portrait picture must come out as a portrait MP4 whose audio is
// normalised, with the picture letterboxed on black.
func TestIntegration_CoverMode(t *testing.T) {
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

	// A quiet 6 s tone in an m4a: audio only, no video stream at all.
	song := filepath.Join(dir, "song.m4a")
	gen := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=6",
		"-af", "volume=-10dB", "-c:a", "aac", "-b:a", "128k", song)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate song: %v\n%s", err, out)
	}

	// A portrait 900x1400 picture, so the X box must flip to 720x1280.
	art := filepath.Join(dir, "art.png")
	genArt := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=900x1400:duration=1:rate=1", "-frames:v", "1", art)
	if out, err := genArt.CombinedOutput(); err != nil {
		t.Fatalf("generate art: %v\n%s", err, out)
	}

	rn := &ffmpeg.Runner{FFmpeg: ffmpegPath, FFprobe: ffprobePath, Log: func(s string) { t.Log(s) }}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	probe, err := rn.ProbeFile(ctx, song)
	if err != nil {
		t.Fatal(err)
	}
	if probe.HasVideo() {
		t.Fatalf("the fixture should have no video stream: %+v", probe.Video)
	}

	req := Request{
		InputPath: song, Probe: probe, Preset: preset.Default(), OutputDir: dir,
		CoverPath: art, CoverWidth: 900, CoverHeight: 1400,
	}
	if !req.IsCoverMode() {
		t.Fatal("expected cover mode for an audio input on a video preset")
	}

	res, err := Run(ctx, rn, req, Hooks{})
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}

	out := res.After
	if out.Video == nil {
		t.Fatal("cover mode must produce a video stream")
	}
	if out.Video.Width != 720 || out.Video.Height != 1280 {
		t.Errorf("output = %dx%d, want 720x1280 (portrait art flips the box)", out.Video.Width, out.Video.Height)
	}
	if out.Video.Codec != "h264" || out.Video.PixFmt != "yuv420p" {
		t.Errorf("video = %s %s", out.Video.Codec, out.Video.PixFmt)
	}
	if out.Audio == nil || out.Audio.SampleRate != 48000 || out.Audio.Channels != 2 {
		t.Errorf("audio = %+v, want 48 kHz stereo", out.Audio)
	}
	if math.Abs(out.Duration-probe.Duration) > 0.5 {
		t.Errorf("duration drifted: in %.2f out %.2f", probe.Duration, out.Duration)
	}
	if math.Abs(res.AfterLoudness.Integrated-(-14)) > 1.2 {
		t.Errorf("loudness = %.2f LUFS, want -14 +/- 1.2", res.AfterLoudness.Integrated)
	}
	// A still picture at 1 fps must be far cheaper than the audio.
	if out.Video.BitRate > 0 && out.Audio.BitRate > 0 && out.Video.BitRate > out.Audio.BitRate {
		t.Errorf("video %d b/s should be well under audio %d b/s for a still frame", out.Video.BitRate, out.Audio.BitRate)
	}
	if _, err := os.Stat(res.OutputPath); err != nil {
		t.Fatalf("output missing: %v", err)
	}
	t.Logf("cover output: %s %dx%d, %.1f LUFS, %.0f KB",
		filepath.Base(res.OutputPath), out.Video.Width, out.Video.Height,
		res.AfterLoudness.Integrated, float64(out.Size)/1024)
}

// TestIntegration_CoverModeBlackFrame is the same path with no picture at all.
func TestIntegration_CoverModeBlackFrame(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobePath, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()

	song := filepath.Join(dir, "song.mp3")
	gen := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=330:duration=5", "-c:a", "libmp3lame", song)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot build an mp3 fixture with this ffmpeg: %v\n%s", err, out)
	}

	rn := &ffmpeg.Runner{FFmpeg: ffmpegPath, FFprobe: ffprobePath}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	probe, err := rn.ProbeFile(ctx, song)
	if err != nil {
		t.Fatal(err)
	}

	res, err := Run(ctx, rn, Request{
		InputPath: song, Probe: probe, Preset: preset.Default(), OutputDir: dir,
	}, Hooks{})
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}
	if res.After.Video == nil || res.After.Video.Width != 1280 || res.After.Video.Height != 720 {
		t.Errorf("black-frame output = %+v, want 1280x720", res.After.Video)
	}
}
