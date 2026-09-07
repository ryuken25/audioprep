package pipeline

import (
	"strings"
	"testing"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
	"github.com/ryuken25/audioprep/internal/preset"
)

// A typical phone vocal cover: stored 1920x1080 at 29.97 fps, rotated to
// portrait, 48 kHz stereo AAC, 42.5 s.
func phoneProbe() *ffmpeg.Probe {
	return &ffmpeg.Probe{
		Path:     "in.mp4",
		Duration: 42.5,
		Video:    &ffmpeg.VideoStream{Codec: "hevc", Width: 1920, Height: 1080, Rotation: 90, FPS: 29.97},
		Audio:    &ffmpeg.AudioStream{Codec: "aac", SampleRate: 48000, Channels: 2},
	}
}

var measured = ffmpeg.LoudnormStats{InputI: -21.81, InputTP: -17.69, InputLRA: 0.1, InputThresh: -31.81, TargetOffset: 0.01}

func mustPreset(t *testing.T, id string) preset.Preset {
	t.Helper()
	p, ok := preset.ByID(id)
	if !ok {
		t.Fatalf("unknown preset %q", id)
	}
	return p
}

// join makes the golden strings readable in one line.
func join(args []string) string { return strings.Join(args, " ") }

func TestMeasureArgs_XAudioFirst(t *testing.T) {
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDXAudioFirst)}
	got := join(MeasureArgs(req))
	want := "-i in.mp4 -map 0:a:0 -vn -af lowpass=f=16000,loudnorm=I=-14:TP=-1:LRA=11:print_format=json -f null " + nullDevice
	if got != want {
		t.Errorf("MeasureArgs\n got: %s\nwant: %s", got, want)
	}
}

func TestMeasureArgs_NoLowpass(t *testing.T) {
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDXBalanced)}
	got := join(MeasureArgs(req))
	if strings.Contains(got, "lowpass") {
		t.Errorf("balanced preset must not low-pass: %s", got)
	}
}

func TestEncodeArgs_XAudioFirst_Golden(t *testing.T) {
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDXAudioFirst)}
	got := join(EncodeArgs(req, measured, "", "out.mp4"))
	want := strings.Join([]string{
		"-i in.mp4 -map 0:v:0 -map 0:a:0",
		"-c:v libx264 -profile:v high -level 4.1 -pix_fmt yuv420p -crf 23 -maxrate 2500k -bufsize 5000k",
		// 29.97 fps is under the 30 cap, so no fps filter; portrait phone clip scales to 720x1280.
		"-vf scale=720:1280",
		"-af lowpass=f=16000,loudnorm=I=-14:TP=-1:LRA=11:measured_I=-21.81:measured_TP=-17.69:measured_LRA=0.1:measured_thresh=-31.81:offset=0.01:linear=true:print_format=summary,aresample=48000",
		"-c:a aac -b:a 256k -ar 48000 -ac 2",
		"-movflags +faststart out.mp4",
	}, " ")
	if got != want {
		t.Errorf("EncodeArgs X audio-first\n got: %s\nwant: %s", got, want)
	}
}

func TestEncodeArgs_FPSCapApplied(t *testing.T) {
	pr := phoneProbe()
	pr.Video.FPS = 60
	req := Request{InputPath: "in.mp4", Probe: pr, Preset: mustPreset(t, preset.IDXAudioFirst)}
	got := join(EncodeArgs(req, measured, "", "out.mp4"))
	if !strings.Contains(got, "-vf fps=30,scale=720:1280") {
		t.Errorf("60 fps input should get fps=30 before scale: %s", got)
	}
}

func TestEncodeArgs_NoScaleWhenFits(t *testing.T) {
	pr := phoneProbe()
	pr.Video.Width, pr.Video.Height, pr.Video.Rotation = 1280, 720, 0
	req := Request{InputPath: "in.mp4", Probe: pr, Preset: mustPreset(t, preset.IDXAudioFirst)}
	got := join(EncodeArgs(req, measured, "", "out.mp4"))
	if strings.Contains(got, "-vf") {
		t.Errorf("720p input must not get a -vf: %s", got)
	}
}

func TestEncodeArgs_Balanced(t *testing.T) {
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDXBalanced)}
	got := join(EncodeArgs(req, measured, "", "out.mp4"))
	for _, want := range []string{"-maxrate 6000k", "-bufsize 12000k", "-b:a 256k"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	if strings.Contains(got, "lowpass") || strings.Contains(got, "scale=") {
		t.Errorf("balanced 1080p portrait: no lowpass, no scale expected: %s", got)
	}
}

func TestEncodeArgs_TikTok(t *testing.T) {
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDTikTok)}
	got := join(EncodeArgs(req, measured, "", "out.mp4"))
	if !strings.Contains(got, "-maxrate 8000k -bufsize 16000k") {
		t.Errorf("tiktok rates: %s", got)
	}
	if strings.Contains(got, "scale=") {
		t.Errorf("1080x1920 portrait already fits the tiktok box: %s", got)
	}
}

func TestEncodeArgs_AudioOnly_Golden(t *testing.T) {
	req := Request{InputPath: "song.wav", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDAudioOnly)}
	got := join(EncodeArgs(req, measured, "", "out.m4a"))
	want := "-i song.wav -map 0:a:0 -vn " +
		"-af loudnorm=I=-14:TP=-1:LRA=11:measured_I=-21.81:measured_TP=-17.69:measured_LRA=0.1:measured_thresh=-31.81:offset=0.01:linear=true:print_format=summary,aresample=48000 " +
		"-c:a aac -b:a 256k -ar 48000 -ac 2 -movflags +faststart out.m4a"
	if got != want {
		t.Errorf("EncodeArgs audio-only\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "libx264") {
		t.Error("audio-only must not configure a video encoder")
	}
}

func TestEncodeArgs_StaticVideo_Golden(t *testing.T) {
	req := Request{
		InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDXAudioFirst),
		StaticVideo: true, StaticAt: 1,
	}
	got := join(EncodeArgs(req, measured, "frame.png", "out.mp4"))
	want := strings.Join([]string{
		"-framerate 1 -loop 1 -i frame.png -i in.mp4 -map 0:v:0 -map 1:a:0 -shortest -t 42.5",
		"-c:v libx264 -profile:v high -level 4.1 -pix_fmt yuv420p -crf 23 -maxrate 2500k -bufsize 5000k",
		"-tune stillimage -r 1",
		"-vf scale=720:1280",
		"-af lowpass=f=16000,loudnorm=I=-14:TP=-1:LRA=11:measured_I=-21.81:measured_TP=-17.69:measured_LRA=0.1:measured_thresh=-31.81:offset=0.01:linear=true:print_format=summary,aresample=48000",
		"-c:a aac -b:a 256k -ar 48000 -ac 2",
		"-movflags +faststart out.mp4",
	}, " ")
	if got != want {
		t.Errorf("EncodeArgs static\n got: %s\nwant: %s", got, want)
	}
}

func TestStillFrameArgs(t *testing.T) {
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), StaticAt: 1}
	got := join(StillFrameArgs(req, "f.png"))
	want := "-ss 1 -i in.mp4 -map 0:v:0 -frames:v 1 -update 1 f.png"
	if got != want {
		t.Errorf("StillFrameArgs\n got: %s\nwant: %s", got, want)
	}
	// Timestamp beyond the end falls back to the middle.
	req.StaticAt = 999
	if got := join(StillFrameArgs(req, "f.png")); !strings.HasPrefix(got, "-ss 21.25 ") {
		t.Errorf("out-of-range timestamp: %s", got)
	}
}

func TestEncodeArgs_LibfdkAAC(t *testing.T) {
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDXAudioFirst), AudioEncoder: "libfdk_aac"}
	got := join(EncodeArgs(req, measured, "", "out.mp4"))
	if !strings.Contains(got, "-c:a libfdk_aac -b:a 256k -ar 48000 -ac 2 -profile:a aac_low") {
		t.Errorf("libfdk args: %s", got)
	}
}

func TestEncodeArgs_SilentInputFallsBackToDynamic(t *testing.T) {
	silent := ffmpeg.LoudnormStats{InputI: -99}
	req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: mustPreset(t, preset.IDXAudioFirst)}
	got := join(EncodeArgs(req, silent, "", "out.mp4"))
	if strings.Contains(got, "linear=true") {
		t.Errorf("silent input must not use linear mode: %s", got)
	}
}

func TestWarnings(t *testing.T) {
	pr := phoneProbe()
	pr.Duration = 150
	w := Warnings(pr, mustPreset(t, preset.IDXAudioFirst))
	if len(w) != 1 || !strings.Contains(w[0], "140") {
		t.Errorf("expected a 140 s warning, got %v", w)
	}
	if w := Warnings(pr, mustPreset(t, preset.IDTikTok)); len(w) != 0 {
		t.Errorf("tiktok has no duration warning, got %v", w)
	}
}

func TestPostWarnings(t *testing.T) {
	out := &ffmpeg.Probe{
		Duration: 141, Size: 600 * 1024 * 1024,
		Video: &ffmpeg.VideoStream{Width: 2560, Height: 1440, FPS: 120},
	}
	w := PostWarnings(out)
	if len(w) != 4 {
		t.Errorf("expected 4 warnings (size, duration, dims, fps), got %d: %v", len(w), w)
	}
	if w := PostWarnings(&ffmpeg.Probe{Duration: 30, Size: 1000, Video: &ffmpeg.VideoStream{Width: 1280, Height: 720, FPS: 30}}); len(w) != 0 {
		t.Errorf("clean output warned: %v", w)
	}
}

// Every built-in preset must produce args that at least mention the right
// encoder and a non-empty filter chain. Cheap guard against a typo in the
// preset table.
func TestAllPresetsProduceArgs(t *testing.T) {
	for _, p := range preset.All {
		req := Request{InputPath: "in.mp4", Probe: phoneProbe(), Preset: p}
		args := EncodeArgs(req, measured, "frame.png", "out"+p.OutputExt)
		s := join(args)
		if !strings.Contains(s, "-c:a aac") || !strings.Contains(s, "loudnorm=") {
			t.Errorf("preset %s: audio args missing: %s", p.ID, s)
		}
		if p.AudioOnly != !strings.Contains(s, "libx264") {
			t.Errorf("preset %s: AudioOnly=%v but libx264 presence mismatched: %s", p.ID, p.AudioOnly, s)
		}
		if len(MeasureArgs(req)) == 0 {
			t.Errorf("preset %s: empty measure args", p.ID)
		}
	}
}
