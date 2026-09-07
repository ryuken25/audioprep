package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// Probe is the subset of ffprobe output the app cares about, already
// converted from ffprobe's stringly-typed JSON into real Go numbers.
type Probe struct {
	Path       string
	FormatName string
	Duration   float64 // seconds
	Size       int64   // bytes
	BitRate    int64   // overall container bitrate, bits/s (0 if unknown)
	Video      *VideoStream
	Audio      *AudioStream
}

// VideoStream describes the first video stream. Width/Height are the *stored*
// dimensions; Rotation tells you how the player is expected to turn them.
// A phone video shot upright is usually stored as 1920x1080 with Rotation=90.
type VideoStream struct {
	Codec    string
	Width    int
	Height   int
	Rotation int // normalised to 0, 90, 180 or 270
	FPS      float64
	BitRate  int64
	PixFmt   string
}

// AudioStream describes the first audio stream.
type AudioStream struct {
	Codec      string
	SampleRate int
	Channels   int
	BitRate    int64
}

// DisplayWidth and DisplayHeight return the dimensions as the viewer sees
// them, i.e. after applying the rotation metadata.
func (v *VideoStream) DisplayWidth() int {
	if v.Rotation == 90 || v.Rotation == 270 {
		return v.Height
	}
	return v.Width
}

func (v *VideoStream) DisplayHeight() int {
	if v.Rotation == 90 || v.Rotation == 270 {
		return v.Width
	}
	return v.Height
}

// HasVideo reports whether a real video stream was found. Cover art in an MP3
// shows up as a video stream too, so we also require a sane frame rate.
func (p *Probe) HasVideo() bool {
	return p.Video != nil && p.Video.Width > 0 && p.Video.Height > 0
}

// --- raw JSON shapes -------------------------------------------------------
//
// ffprobe prints most numbers as JSON strings ("duration": "12.345"), so we
// decode into string fields first and convert afterwards. Keeping these raw
// types unexported keeps the messy part contained in this file.

type rawProbe struct {
	Streams []rawStream `json:"streams"`
	Format  rawFormat   `json:"format"`
}

type rawFormat struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
	BitRate    string `json:"bit_rate"`
}

type rawStream struct {
	CodecType    string            `json:"codec_type"`
	CodecName    string            `json:"codec_name"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	PixFmt       string            `json:"pix_fmt"`
	RFrameRate   string            `json:"r_frame_rate"`
	AvgFrameRate string            `json:"avg_frame_rate"`
	SampleRate   string            `json:"sample_rate"`
	Channels     int               `json:"channels"`
	BitRate      string            `json:"bit_rate"`
	Disposition  map[string]int    `json:"disposition"`
	Tags         map[string]string `json:"tags"`
	SideData     []rawSideData     `json:"side_data_list"`
}

type rawSideData struct {
	Type     string  `json:"side_data_type"`
	Rotation float64 `json:"rotation"`
}

// ProbeFile runs ffprobe on path and returns the parsed result.
func (r *Runner) ProbeFile(ctx context.Context, path string) (*Probe, error) {
	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	}
	cmd := exec.CommandContext(ctx, r.FFprobe, args...)
	hideConsole(cmd)
	r.logf("$ ffprobe %s", strings.Join(quoteArgs(args), " "))

	out, err := cmd.Output()
	if err != nil {
		// exec.ExitError carries stderr; surface it so a corrupt file gives a
		// useful message instead of just "exit status 1".
		var ee *exec.ExitError
		if asExitError(err, &ee) {
			return nil, fmt.Errorf("ffprobe failed: %w\n%s", err, tail(string(ee.Stderr), 10))
		}
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}
	p, err := ParseProbeJSON(out)
	if err != nil {
		return nil, err
	}
	p.Path = path
	return p, nil
}

// ParseProbeJSON converts raw ffprobe JSON into a Probe. It is a pure function
// so the unit tests can feed it captured ffprobe output without running
// anything.
func ParseProbeJSON(data []byte) (*Probe, error) {
	var raw rawProbe
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse ffprobe json: %w", err)
	}

	p := &Probe{
		FormatName: raw.Format.FormatName,
		Duration:   atof(raw.Format.Duration),
		Size:       atoi64(raw.Format.Size),
		BitRate:    atoi64(raw.Format.BitRate),
	}

	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			// Skip attached pictures (album art) so an MP3 with cover art
			// does not look like a video file.
			if s.Disposition["attached_pic"] == 1 || p.Video != nil {
				continue
			}
			fps := parseRate(s.AvgFrameRate)
			if fps <= 0 {
				fps = parseRate(s.RFrameRate)
			}
			p.Video = &VideoStream{
				Codec:    s.CodecName,
				Width:    s.Width,
				Height:   s.Height,
				Rotation: parseRotation(s),
				FPS:      fps,
				BitRate:  atoi64(s.BitRate),
				PixFmt:   s.PixFmt,
			}
		case "audio":
			if p.Audio != nil {
				continue
			}
			p.Audio = &AudioStream{
				Codec:      s.CodecName,
				SampleRate: int(atoi64(s.SampleRate)),
				Channels:   s.Channels,
				BitRate:    atoi64(s.BitRate),
			}
		}
	}

	// Some containers (e.g. raw AAC) do not report a duration on the format;
	// that is fine, the pipeline treats 0 as "unknown".
	return p, nil
}

// parseRotation reads rotation from either the modern side_data_list
// (ffmpeg >= 5) or the legacy "rotate" tag, and normalises it to 0/90/180/270.
func parseRotation(s rawStream) int {
	var deg float64
	found := false
	for _, sd := range s.SideData {
		if sd.Type == "Display Matrix" {
			deg = sd.Rotation
			found = true
			break
		}
	}
	if !found {
		if v, ok := s.Tags["rotate"]; ok {
			deg = atof(v)
			found = true
		}
	}
	if !found {
		return 0
	}
	// ffmpeg reports the display matrix as a counter-clockwise angle, so a
	// portrait phone clip commonly shows rotation=-90. We only care about the
	// axis swap, so fold everything into [0, 360).
	r := int(math.Round(deg))
	r = ((r % 360) + 360) % 360
	// Snap to the nearest right angle; real files are always multiples of 90.
	switch {
	case r >= 45 && r < 135:
		return 90
	case r >= 135 && r < 225:
		return 180
	case r >= 225 && r < 315:
		return 270
	default:
		return 0
	}
}

// parseRate turns "30000/1001" or "30" into a float.
func parseRate(s string) float64 {
	if s == "" || s == "0/0" {
		return 0
	}
	if num, den, ok := strings.Cut(s, "/"); ok {
		n, d := atof(num), atof(den)
		if d == 0 {
			return 0
		}
		return n / d
	}
	return atof(s)
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// Summary returns a compact one-line description, used in logs.
func (p *Probe) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%.2fs", p.Duration)
	if p.Video != nil {
		fmt.Fprintf(&b, " video=%s %dx%d", p.Video.Codec, p.Video.DisplayWidth(), p.Video.DisplayHeight())
		if p.Video.Rotation != 0 {
			fmt.Fprintf(&b, " (stored %dx%d rot %d)", p.Video.Width, p.Video.Height, p.Video.Rotation)
		}
		fmt.Fprintf(&b, " %.3gfps", p.Video.FPS)
	}
	if p.Audio != nil {
		fmt.Fprintf(&b, " audio=%s %dHz ch%d", p.Audio.Codec, p.Audio.SampleRate, p.Audio.Channels)
	}
	return b.String()
}
