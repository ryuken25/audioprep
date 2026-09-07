package pipeline

import (
	"path/filepath"
	"testing"
)

func TestOutputPath(t *testing.T) {
	never := func(string) bool { return false }

	got := OutputPath(filepath.Join("C:", "vids", "take 1.MOV"), "", "x-audio", ".mp4", never)
	want := filepath.Join("C:", "vids", "take 1_x-audio.mp4")
	if got != want {
		t.Errorf("next-to-input: got %q want %q", got, want)
	}

	got = OutputPath(filepath.Join("C:", "vids", "take.mp4"), filepath.Join("D:", "out"), "tiktok", ".mp4", never)
	want = filepath.Join("D:", "out", "take_tiktok.mp4")
	if got != want {
		t.Errorf("output dir: got %q want %q", got, want)
	}

	got = OutputPath("song.wav", "", "audio-only", ".m4a", never)
	if filepath.Base(got) != "song_audio-only.m4a" {
		t.Errorf("m4a: got %q", got)
	}
}

func TestOutputPath_Collisions(t *testing.T) {
	// Simulate: base name and -1 exist, -2 is free.
	taken := map[string]bool{
		filepath.Join("d", "clip_x-audio.mp4"):   true,
		filepath.Join("d", "clip_x-audio-1.mp4"): true,
	}
	exists := func(p string) bool { return taken[p] }

	got := OutputPath(filepath.Join("d", "clip.mp4"), "", "x-audio", ".mp4", exists)
	want := filepath.Join("d", "clip_x-audio-2.mp4")
	if got != want {
		t.Errorf("collision: got %q want %q", got, want)
	}
}

func TestOutputPath_NoExtensionInput(t *testing.T) {
	got := OutputPath(filepath.Join("d", "clip"), "", "x-audio", ".mp4", func(string) bool { return false })
	if filepath.Base(got) != "clip_x-audio.mp4" {
		t.Errorf("got %q", got)
	}
}
