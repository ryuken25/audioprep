package ffmpeg

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LoudnormTargets are the three numbers the EBU R128 normaliser aims for.
type LoudnormTargets struct {
	I   float64 // integrated loudness, LUFS
	TP  float64 // true peak ceiling, dBTP
	LRA float64 // loudness range, LU
}

// LoudnormStats is what loudnorm's first (measurement) pass reports. The
// "Input*" values describe the source; "TargetOffset" is the gain loudnorm
// wants to apply. All of these get fed back into the second pass so it can
// run in linear mode (a single clean gain change) instead of dynamic mode
// (a limiter that pumps).
type LoudnormStats struct {
	InputI       float64
	InputTP      float64
	InputLRA     float64
	InputThresh  float64
	OutputI      float64
	OutputTP     float64
	OutputLRA    float64
	OutputThresh float64
	TargetOffset float64
	NormType     string
}

// loudnorm prints its JSON with every number quoted, e.g. "input_i": "-23.51",
// and uses "-inf" for digital silence. We decode into strings and convert.
type rawLoudnorm struct {
	InputI       string `json:"input_i"`
	InputTP      string `json:"input_tp"`
	InputLRA     string `json:"input_lra"`
	InputThresh  string `json:"input_thresh"`
	OutputI      string `json:"output_i"`
	OutputTP     string `json:"output_tp"`
	OutputLRA    string `json:"output_lra"`
	OutputThresh string `json:"output_thresh"`
	NormType     string `json:"normalization_type"`
	TargetOffset string `json:"target_offset"`
}

// ErrNoLoudnormJSON is returned when the stderr text contains no JSON block.
var ErrNoLoudnormJSON = errors.New("no loudnorm JSON block found in ffmpeg output")

// ParseLoudnormJSON finds the JSON object loudnorm prints at the end of the
// measurement pass and converts it. It scans for the *last* {...} block so any
// unrelated braces earlier in the log are ignored.
func ParseLoudnormJSON(stderr string) (LoudnormStats, error) {
	start := strings.LastIndex(stderr, "{")
	end := strings.LastIndex(stderr, "}")
	if start < 0 || end < start {
		return LoudnormStats{}, ErrNoLoudnormJSON
	}
	block := stderr[start : end+1]

	var raw rawLoudnorm
	if err := json.Unmarshal([]byte(block), &raw); err != nil {
		return LoudnormStats{}, fmt.Errorf("decode loudnorm json: %w", err)
	}
	if raw.InputI == "" {
		return LoudnormStats{}, ErrNoLoudnormJSON
	}

	return LoudnormStats{
		InputI:       parseLoudnormNum(raw.InputI),
		InputTP:      parseLoudnormNum(raw.InputTP),
		InputLRA:     parseLoudnormNum(raw.InputLRA),
		InputThresh:  parseLoudnormNum(raw.InputThresh),
		OutputI:      parseLoudnormNum(raw.OutputI),
		OutputTP:     parseLoudnormNum(raw.OutputTP),
		OutputLRA:    parseLoudnormNum(raw.OutputLRA),
		OutputThresh: parseLoudnormNum(raw.OutputThresh),
		TargetOffset: parseLoudnormNum(raw.TargetOffset),
		NormType:     raw.NormType,
	}, nil
}

// parseLoudnormNum handles the "-inf" / "inf" spellings loudnorm uses for
// silent input. Go's ParseFloat already accepts those, so this mostly exists
// to make the intent explicit and to swallow the error into 0.
func parseLoudnormNum(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// IsSilent reports whether the measurement pass saw effectively no signal.
// loudnorm's linear mode cannot work with -inf inputs, so the caller should
// fall back to a plain single-pass loudnorm in that case.
func (s LoudnormStats) IsSilent() bool {
	return s.InputI < -70 || s.InputI != s.InputI // NaN check without importing math
}

// MeasureFilter builds the loudnorm filter string for pass one. It asks for
// JSON so ParseLoudnormJSON can read the result back.
func MeasureFilter(t LoudnormTargets) string {
	return fmt.Sprintf("loudnorm=I=%s:TP=%s:LRA=%s:print_format=json",
		ftoa(t.I), ftoa(t.TP), ftoa(t.LRA))
}

// ApplyFilter builds the loudnorm filter string for pass two, feeding the
// measured values back in and asking for linear mode. loudnorm resamples to
// 192 kHz internally, so callers must append an aresample afterwards.
func ApplyFilter(t LoudnormTargets, m LoudnormStats) string {
	if m.IsSilent() {
		// Nothing to measure; let loudnorm do its dynamic thing.
		return fmt.Sprintf("loudnorm=I=%s:TP=%s:LRA=%s", ftoa(t.I), ftoa(t.TP), ftoa(t.LRA))
	}
	return fmt.Sprintf(
		"loudnorm=I=%s:TP=%s:LRA=%s:measured_I=%s:measured_TP=%s:measured_LRA=%s:measured_thresh=%s:offset=%s:linear=true:print_format=summary",
		ftoa(t.I), ftoa(t.TP), ftoa(t.LRA),
		ftoa(m.InputI), ftoa(m.InputTP), ftoa(m.InputLRA), ftoa(m.InputThresh), ftoa(m.TargetOffset),
	)
}

// ftoa prints a float compactly ("-14", "-1", "-23.51") so filter strings
// stay readable in the log.
func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// --- ebur128 post-check ----------------------------------------------------

// Loudness is the result of measuring a finished file with the ebur128
// filter. It is what the "after" column of the result table shows.
type Loudness struct {
	Integrated float64 // LUFS
	Range      float64 // LU
	TruePeak   float64 // dBTP (dBFS in ebur128's wording)
}

var (
	reEbuI    = regexp.MustCompile(`(?m)^\s*I:\s*(-?[\d.]+|-?inf)\s*LUFS`)
	reEbuLRA  = regexp.MustCompile(`(?m)^\s*LRA:\s*(-?[\d.]+|-?inf)\s*LU`)
	reEbuPeak = regexp.MustCompile(`(?m)^\s*Peak:\s*(-?[\d.]+|-?inf)\s*dBFS`)
)

// ParseEBUR128Summary reads the summary block that `-af ebur128=peak=true`
// prints when the stream ends.
func ParseEBUR128Summary(stderr string) (Loudness, error) {
	// The summary is the last thing printed; only look at the tail so the
	// per-frame "M: -20.1 S: -19.8" lines do not confuse the regexes.
	idx := strings.LastIndex(stderr, "Summary:")
	if idx < 0 {
		return Loudness{}, errors.New("no ebur128 summary found")
	}
	tailText := stderr[idx:]

	var l Loudness
	if m := reEbuI.FindStringSubmatch(tailText); m != nil {
		l.Integrated = parseLoudnormNum(m[1])
	} else {
		return Loudness{}, errors.New("ebur128 summary missing integrated loudness")
	}
	if m := reEbuLRA.FindStringSubmatch(tailText); m != nil {
		l.Range = parseLoudnormNum(m[1])
	}
	if m := reEbuPeak.FindStringSubmatch(tailText); m != nil {
		l.TruePeak = parseLoudnormNum(m[1])
	}
	return l, nil
}
