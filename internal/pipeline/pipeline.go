// Package pipeline turns "this input, this preset, these options" into the
// exact ffmpeg argument lists that produce the output, and runs them in
// order. The argument builders are pure functions so they can be golden
// tested; Run (in run.go) is the only thing that touches the runner.
package pipeline

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ryuken25/audioprep/internal/ffmpeg"
	"github.com/ryuken25/audioprep/internal/preset"
)

// Request is everything the pipeline needs to know about one job.
type Request struct {
	InputPath string
	Probe     *ffmpeg.Probe
	Preset    preset.Preset

	// StaticVideo replaces the video with a single frame taken at StaticAt
	// seconds, held for the whole duration at 1 fps. Ignored for AudioOnly.
	StaticVideo bool
	StaticAt    float64

	// OutputDir overrides "next to the input". Empty = next to input.
	OutputDir string

	// AudioEncoder is "aac" or "libfdk_aac"; the caller detects which the
	// ffmpeg build supports. Empty defaults to "aac".
	AudioEncoder string

	// CoverPath is a still picture to use as the video when the input has no
	// video stream and the preset is not audio-only. Empty means a black
	// frame at the preset's box size. Ignored for inputs that have video.
	CoverPath string

	// CoverWidth/CoverHeight are the picture's own pixel size, used to decide
	// the output orientation. Zero means "unknown, use the preset's box".
	CoverWidth, CoverHeight int
}

// IsCoverMode reports whether this request turns an audio file into a video
// with a single still picture (or a black frame).
func (r Request) IsCoverMode() bool {
	if r.Preset.AudioOnly {
		return false
	}
	return r.Probe != nil && !r.Probe.HasVideo()
}

// CoverBox returns the output frame for cover mode: the preset's box, turned
// to match the picture's orientation. The output is always exactly this size.
// The picture is scaled down to fit inside it and padded with black, so a
// short cover and a tall one both come out as one predictable frame; that is
// what the tile in the UI previews (it letterboxes on black too).
//
// A picture smaller than the box is not enlarged: the scale filter only
// decreases, so it simply sits in the middle with wider bars.
func (r Request) CoverBox() (w, h int) {
	bw, bh := r.Preset.Box()
	// Flip the box if the picture disagrees with it. A square picture keeps
	// the preset's own orientation.
	if r.CoverWidth > 0 && r.CoverHeight > 0 {
		if (r.CoverHeight > r.CoverWidth) != (bh > bw) && r.CoverHeight != r.CoverWidth {
			bw, bh = bh, bw
		}
	}
	return even(bw), even(bh)
}

// targets pulls the loudnorm numbers out of the preset.
func (r Request) targets() ffmpeg.LoudnormTargets {
	return ffmpeg.LoudnormTargets{I: r.Preset.LoudnessI, TP: r.Preset.TruePeak, LRA: r.Preset.LRA}
}

func (r Request) audioEncoder() string {
	if r.AudioEncoder == "" {
		return "aac"
	}
	return r.AudioEncoder
}

// audioFilterPrefix is the part of the audio filter chain that runs BEFORE
// loudnorm. It has to be identical in both passes: if the measurement pass
// saw the unfiltered signal and the apply pass filters it, the measured
// loudness is wrong and linear mode over- or under-shoots.
func (r Request) audioFilterPrefix() string {
	if r.Preset.Lowpass && r.Preset.LowpassHz > 0 {
		return fmt.Sprintf("lowpass=f=%d,", r.Preset.LowpassHz)
	}
	return ""
}

// MeasureArgs builds the loudnorm measurement pass (pass 1). It decodes the
// audio only, runs it through the same pre-filters as pass 2 plus loudnorm
// in JSON mode, and discards the output.
func MeasureArgs(r Request) []string {
	af := r.audioFilterPrefix() + ffmpeg.MeasureFilter(r.targets())
	return []string{
		"-i", r.InputPath,
		"-map", "0:a:0",
		"-vn",
		"-af", af,
		"-f", "null",
		nullDevice,
	}
}

// StillFrameArgs extracts one PNG at StaticAt seconds for static-video mode.
// The -ss before -i seeks fast (keyframe-accurate then decodes to the exact
// time), and -update 1 tells the image muxer we want exactly one file.
func StillFrameArgs(r Request, framePath string) []string {
	at := r.StaticAt
	// Clamp so a bad timestamp does not seek past the end and produce nothing.
	if r.Probe != nil && r.Probe.Duration > 0 && at >= r.Probe.Duration {
		at = r.Probe.Duration / 2
	}
	if at < 0 {
		at = 0
	}
	return []string{
		"-ss", ftoa(at),
		"-i", r.InputPath,
		"-map", "0:v:0",
		"-frames:v", "1",
		"-update", "1",
		framePath,
	}
}

// EncodeArgs builds the main encode (pass 2). stats comes from MeasureArgs'
// stderr via ffmpeg.ParseLoudnormJSON. framePath is only used when
// StaticVideo is set and points at the PNG from StillFrameArgs.
func EncodeArgs(r Request, stats ffmpeg.LoudnormStats, framePath, outputPath string) []string {
	p := r.Preset
	args := make([]string, 0, 48)

	// ---- inputs -----------------------------------------------------------
	cover := r.IsCoverMode()
	static := r.StaticVideo && !p.AudioOnly && !cover
	if static {
		// Input 0: the still image looped at 1 fps. Input 1: the original,
		// for its audio. -shortest plus an explicit -t keeps the loop from
		// running forever if -shortest misbehaves (it has, historically).
		args = append(args,
			"-framerate", "1", "-loop", "1", "-i", framePath,
			"-i", r.InputPath,
			"-map", "0:v:0", "-map", "1:a:0",
			"-shortest",
		)
		if r.Probe != nil && r.Probe.Duration > 0 {
			args = append(args, "-t", ftoa(r.Probe.Duration))
		}
	} else if cover {
		// An audio file becomes a video: input 0 is the picture (or a black
		// canvas), input 1 is the audio. -shortest plus an explicit -t keeps
		// the looped image from running past the track.
		cw, ch := r.CoverBox()
		if r.CoverPath != "" {
			args = append(args, "-framerate", "1", "-loop", "1", "-i", r.CoverPath)
		} else {
			args = append(args, "-f", "lavfi", "-i",
				fmt.Sprintf("color=c=black:s=%dx%d:r=1", cw, ch))
		}
		args = append(args,
			"-i", r.InputPath,
			"-map", "0:v:0", "-map", "1:a:0",
			"-shortest",
		)
		if r.Probe != nil && r.Probe.Duration > 0 {
			args = append(args, "-t", ftoa(r.Probe.Duration))
		}
	} else {
		args = append(args, "-i", r.InputPath)
		if p.AudioOnly {
			args = append(args, "-map", "0:a:0", "-vn")
		} else {
			args = append(args, "-map", "0:v:0", "-map", "0:a:0")
		}
	}

	// ---- video ------------------------------------------------------------
	if !p.AudioOnly {
		args = append(args,
			"-c:v", "libx264",
			"-profile:v", "high",
			"-level", "4.1",
			"-pix_fmt", "yuv420p",
			"-crf", strconv.Itoa(p.CRF),
		)
		if p.MaxRate != "" {
			args = append(args, "-maxrate", p.MaxRate)
		}
		if p.BufSize != "" {
			args = append(args, "-bufsize", p.BufSize)
		}

		var vf []string
		if static || cover {
			// One frame per second; stillimage tuning spends bits on
			// detail instead of motion it will never see.
			args = append(args, "-tune", "stillimage", "-r", "1")
		} else if r.Probe != nil && r.Probe.Video != nil && p.FPSCap > 0 && r.Probe.Video.FPS > float64(p.FPSCap)+0.01 {
			vf = append(vf, fmt.Sprintf("fps=%d", p.FPSCap))
		}
		if cover {
			// Fit the picture inside the box without stretching, then pad to
			// the exact box so the output is one clean size.
			cw, ch := r.CoverBox()
			if r.CoverPath != "" {
				vf = append(vf, fmt.Sprintf(
					"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black",
					cw, ch, cw, ch))
			}
		} else if r.Probe != nil && r.Probe.Video != nil {
			d := FitDimensions(r.Probe.Video.Width, r.Probe.Video.Height, r.Probe.Video.Rotation, p.MaxWidth, p.MaxHeight)
			if d.Scaled {
				vf = append(vf, fmt.Sprintf("scale=%d:%d", d.Width, d.Height))
			}
		}
		if len(vf) > 0 {
			args = append(args, "-vf", strings.Join(vf, ","))
		}
	}

	// ---- audio ------------------------------------------------------------
	af := r.audioFilterPrefix() + ffmpeg.ApplyFilter(r.targets(), stats) + ",aresample=48000"
	args = append(args,
		"-af", af,
		"-c:a", r.audioEncoder(),
		"-b:a", fmt.Sprintf("%dk", p.AudioBitrateKbps),
		"-ar", "48000",
		"-ac", "2",
	)
	if r.audioEncoder() == "libfdk_aac" {
		// fdk's default cutoff at 256k is fine; just make sure we are in
		// AAC-LC and not HE-AAC.
		args = append(args, "-profile:a", "aac_low")
	}

	// ---- container --------------------------------------------------------
	args = append(args, "-movflags", "+faststart", outputPath)
	return args
}

// CheckArgs measures a finished file with ebur128 for the "after" column.
func CheckArgs(outputPath string) []string {
	return []string{
		"-i", outputPath,
		"-map", "0:a:0",
		"-vn",
		"-af", "ebur128=peak=true",
		"-f", "null",
		nullDevice,
	}
}

// ScaleFor is a convenience for the UI preview ("will encode at 1280x720").
func ScaleFor(p *ffmpeg.Probe, pr preset.Preset) Dimensions {
	if p == nil || p.Video == nil {
		return Dimensions{}
	}
	return FitDimensions(p.Video.Width, p.Video.Height, p.Video.Rotation, pr.MaxWidth, pr.MaxHeight)
}

// Warnings lists platform-limit problems for a probe under a preset. These
// are advisory: the encode still runs.
func Warnings(p *ffmpeg.Probe, pr preset.Preset) []string {
	if p == nil {
		return nil
	}
	var w []string
	if pr.WarnMaxDuration > 0 && p.Duration > pr.WarnMaxDuration {
		w = append(w, fmt.Sprintf("Input is %.0f s; X rejects videos longer than %.0f s.", p.Duration, pr.WarnMaxDuration))
	}
	return w
}

// PostWarnings checks the finished output against X's hard limits.
func PostWarnings(out *ffmpeg.Probe) []string {
	if out == nil {
		return nil
	}
	var w []string
	const maxBytes = 512 * 1024 * 1024
	if out.Size > maxBytes {
		w = append(w, fmt.Sprintf("Output is %.0f MB; X's limit is 512 MB.", float64(out.Size)/1024/1024))
	}
	if out.Duration > 140.05 {
		w = append(w, fmt.Sprintf("Output is %.1f s; X's limit is 140 s.", out.Duration))
	}
	if out.Video != nil {
		if out.Video.DisplayWidth() > 1920 || out.Video.DisplayHeight() > 1200 {
			w = append(w, fmt.Sprintf("Output is %dx%d; X's limit is 1920x1200.", out.Video.DisplayWidth(), out.Video.DisplayHeight()))
		}
		if out.Video.FPS > 60.05 {
			w = append(w, fmt.Sprintf("Output is %.0f fps; X's limit is 60 fps.", out.Video.FPS))
		}
	}
	return w
}

// durationOf is a small helper for progress totals.
func durationOf(p *ffmpeg.Probe) time.Duration {
	if p == nil || p.Duration <= 0 {
		return 0
	}
	return time.Duration(p.Duration * float64(time.Second))
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
