// Package preset holds the encoding presets as plain data.
//
// Everything a preset controls lives in the Preset struct below. The pipeline
// package reads these values and turns them into ffmpeg arguments; there is no
// preset-specific branching anywhere else in the codebase. If you want a new
// preset, add a row to All and you are done.
package preset

// Preset is a complete set of encoding knobs. The "Custom" preset is just a
// Preset the user has edited in the Advanced panel, so every field here is
// user-adjustable in principle.
type Preset struct {
	ID          string // stable identifier, used in output filenames and config
	Name        string // shown in the UI
	Description string // one-liner shown next to the name

	// Video. MaxWidth/MaxHeight describe a bounding box in *landscape*
	// orientation; the pipeline swaps the box when the input is portrait.
	// Zero means "no video stream" (audio-only output).
	MaxWidth  int
	MaxHeight int
	FPSCap    int    // frames per second ceiling; input above this gets an fps filter
	CRF       int    // libx264 constant rate factor (lower = better/larger)
	MaxRate   string // ffmpeg rate string, e.g. "2500k"
	BufSize   string // VBV buffer, normally 2x MaxRate

	// Audio.
	AudioBitrateKbps int     // AAC target bitrate
	LoudnessI        float64 // integrated loudness target in LUFS (EBU R128)
	TruePeak         float64 // true-peak ceiling in dBTP
	LRA              float64 // loudness range target in LU
	Lowpass          bool    // apply a gentle low-pass before loudnorm
	LowpassHz        int     // low-pass cutoff frequency

	// Output.
	AudioOnly bool   // true = drop video, write .m4a
	OutputExt string // ".mp4" or ".m4a"

	// Platform hint. WarnMaxDuration > 0 means the UI should warn when the
	// input is longer than this many seconds. Zero disables the warning.
	WarnMaxDuration float64
}

// IDs. Exported so the UI and config can refer to presets without string
// literals sprinkled around.
const (
	IDXAudioFirst = "x-audio"
	IDXBalanced   = "x-balanced"
	IDTikTok      = "tiktok"
	IDAudioOnly   = "audio-only"
	IDCustom      = "custom"
)

// Shared audio defaults. Every built-in preset uses the same loudness targets
// because -14 LUFS / -1 dBTP is what X, TikTok and YouTube all normalise
// toward; feeding them audio already at that level means their own
// normaliser has nothing to do.
const (
	defaultAudioKbps = 256
	xAudioKbps       = 320 // X (audio-first) and Custom; see DECISIONS.md
	defaultI         = -14.0
	defaultTP        = -1.0
	defaultLRA       = 11.0
	defaultLowpassHz = 16000
)

// All is the preset table, in display order. The first entry is the default.
var All = []Preset{
	{
		// Video is deliberately cheap here: 24 fps, CRF 26, 1.8 Mb/s cap.
		// Audio gets 320k, the top of the AAC-LC ladder. X re-encodes both
		// anyway; this keeps the upload small and the audio input as clean
		// as the encoder can make it.
		ID:               IDXAudioFirst,
		Name:             "X (audio-first)",
		Description:      "720p at 24 fps with a small video budget, 320k AAC at -14 LUFS. Best for vocal covers.",
		MaxWidth:         1280,
		MaxHeight:        720,
		FPSCap:           24,
		CRF:              26,
		MaxRate:          "1800k",
		BufSize:          "3600k",
		AudioBitrateKbps: xAudioKbps,
		LoudnessI:        defaultI,
		TruePeak:         defaultTP,
		LRA:              defaultLRA,
		Lowpass:          true,
		LowpassHz:        defaultLowpassHz,
		OutputExt:        ".mp4",
		WarnMaxDuration:  140,
	},
	{
		ID:               IDXBalanced,
		Name:             "X (balanced)",
		Description:      "1080p with a roomier video budget. Same audio treatment, no low-pass.",
		MaxWidth:         1920,
		MaxHeight:        1080,
		FPSCap:           30,
		CRF:              23,
		MaxRate:          "6000k",
		BufSize:          "12000k",
		AudioBitrateKbps: defaultAudioKbps,
		LoudnessI:        defaultI,
		TruePeak:         defaultTP,
		LRA:              defaultLRA,
		Lowpass:          false,
		LowpassHz:        defaultLowpassHz,
		OutputExt:        ".mp4",
		WarnMaxDuration:  140,
	},
	{
		ID:               IDTikTok,
		Name:             "TikTok / Shorts",
		Description:      "Vertical 1080x1920, generous video budget, same loudness targets.",
		MaxWidth:         1920,
		MaxHeight:        1080,
		FPSCap:           30,
		CRF:              23,
		MaxRate:          "8000k",
		BufSize:          "16000k",
		AudioBitrateKbps: defaultAudioKbps,
		LoudnessI:        defaultI,
		TruePeak:         defaultTP,
		LRA:              defaultLRA,
		Lowpass:          false,
		LowpassHz:        defaultLowpassHz,
		OutputExt:        ".mp4",
	},
	{
		ID:               IDAudioOnly,
		Name:             "Audio only (M4A)",
		Description:      "Drop the video entirely. 256k AAC, loudness-normalised, in an .m4a.",
		AudioBitrateKbps: defaultAudioKbps,
		LoudnessI:        defaultI,
		TruePeak:         defaultTP,
		LRA:              defaultLRA,
		Lowpass:          false,
		LowpassHz:        defaultLowpassHz,
		AudioOnly:        true,
		OutputExt:        ".m4a",
	},
	{
		// Custom starts as a copy of the X audio-first values; the UI lets
		// the user change every field.
		ID:               IDCustom,
		Name:             "Custom",
		Description:      "Start from the X preset and change anything in Advanced.",
		MaxWidth:         1280,
		MaxHeight:        720,
		FPSCap:           24,
		CRF:              26,
		MaxRate:          "1800k",
		BufSize:          "3600k",
		AudioBitrateKbps: xAudioKbps,
		LoudnessI:        defaultI,
		TruePeak:         defaultTP,
		LRA:              defaultLRA,
		Lowpass:          true,
		LowpassHz:        defaultLowpassHz,
		OutputExt:        ".mp4",
	},
}

// Default returns the preset that should be selected on first launch.
func Default() Preset { return All[0] }

// ByID looks a preset up by its identifier. The second return value is false
// when the ID is unknown, in which case the caller should fall back to
// Default(). This "value, ok" pair is the idiomatic Go way to signal
// "not found" without using nil or a sentinel error.
func ByID(id string) (Preset, bool) {
	for _, p := range All {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// Names returns the display names in table order; handy for a dropdown.
func Names() []string {
	out := make([]string, 0, len(All))
	for _, p := range All {
		out = append(out, p.Name)
	}
	return out
}

// ByName is the inverse of Names for the dropdown.
func ByName(name string) (Preset, bool) {
	for _, p := range All {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}

// AsCustom returns a copy of p relabelled as the Custom preset. The UI calls
// this the moment the user edits any Advanced field so the output filename
// says "_custom" rather than pretending to be a stock preset.
func AsCustom(p Preset) Preset {
	p.ID = IDCustom
	p.Name = "Custom"
	p.Description = All[len(All)-1].Description
	return p
}
