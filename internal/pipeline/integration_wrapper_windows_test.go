//go:build windows

package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
	"github.com/ryuken25/audioprep/internal/preset"
)

// TestIntegration_CancelThroughWrapper reproduces the Chocolatey/Scoop shim
// situation: ffmpeg is reached through a .cmd wrapper, so the process we
// start is cmd.exe and the real ffmpeg is its child. Cancel must still stop
// the real encoder, promptly, and leave no partial file.
func TestIntegration_CancelThroughWrapper(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped in -short mode")
	}
	realFFmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	realFFprobe, _ := exec.LookPath("ffprobe")

	dir := t.TempDir()
	wrapper := filepath.Join(dir, "ffmpeg.cmd")
	if err := os.WriteFile(wrapper, []byte("@echo off\r\n\""+realFFmpeg+"\" %*\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	sample := filepath.Join(dir, "long.mp4")
	gen := exec.Command(realFFmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=60:size=1920x1080:rate=30",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=60",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", sample)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v\n%s", err, out)
	}

	rn := &ffmpeg.Runner{FFmpeg: wrapper, FFprobe: realFFprobe}
	probe, err := rn.ProbeFile(context.Background(), sample)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	go func() {
		<-started
		time.Sleep(500 * time.Millisecond)
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
	if elapsed > 15*time.Second {
		t.Errorf("cancel through wrapper took %v; the real ffmpeg was not killed", elapsed)
	}
	// If the grandchild survived it is still writing this file (and holding
	// it open, so the cleanup Remove would have failed on Windows).
	out := filepath.Join(dir, "long_x-audio.mp4")
	time.Sleep(500 * time.Millisecond)
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("partial output still present after cancel: %s", out)
	}
}
