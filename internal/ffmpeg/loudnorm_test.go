package ffmpeg

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestParseLoudnormJSON_RealOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/loudnorm_pass1.txt")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseLoudnormJSON(string(data))
	if err != nil {
		t.Fatalf("ParseLoudnormJSON: %v", err)
	}
	approx := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.01 {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	approx("InputI", s.InputI, -21.81)
	approx("InputTP", s.InputTP, -17.69)
	approx("InputLRA", s.InputLRA, 0.10)
	approx("InputThresh", s.InputThresh, -31.81)
	approx("TargetOffset", s.TargetOffset, 0.01)
	if s.NormType != "dynamic" {
		t.Errorf("NormType = %q", s.NormType)
	}
	if s.IsSilent() {
		t.Error("real signal reported as silent")
	}
}

func TestParseLoudnormJSON_SilentInput(t *testing.T) {
	// loudnorm prints -inf for digital silence.
	txt := `[Parsed_loudnorm_0 @ 0x1]
{
	"input_i" : "-inf",
	"input_tp" : "-inf",
	"input_lra" : "0.00",
	"input_thresh" : "-inf",
	"output_i" : "-inf",
	"output_tp" : "-inf",
	"output_lra" : "0.00",
	"output_thresh" : "-inf",
	"normalization_type" : "dynamic",
	"target_offset" : "inf"
}
size=N/A time=00:00:05.00`
	s, err := ParseLoudnormJSON(txt)
	if err != nil {
		t.Fatal(err)
	}
	if !math.IsInf(s.InputI, -1) {
		t.Errorf("InputI = %v, want -Inf", s.InputI)
	}
	if !s.IsSilent() {
		t.Error("silence not detected")
	}
	// Silent input must fall back to a plain loudnorm (no measured_* keys).
	f := ApplyFilter(LoudnormTargets{I: -14, TP: -1, LRA: 11}, s)
	if strings.Contains(f, "measured_") || strings.Contains(f, "linear") {
		t.Errorf("silent fallback still uses linear mode: %s", f)
	}
}

func TestParseLoudnormJSON_Missing(t *testing.T) {
	if _, err := ParseLoudnormJSON("no json here at all"); err == nil {
		t.Error("expected an error for text without JSON")
	}
	// Braces present but not loudnorm output.
	if _, err := ParseLoudnormJSON(`[x @ 0] {"foo":"bar"}`); err == nil {
		t.Error("expected an error for unrelated JSON")
	}
}

func TestParseLoudnormJSON_PicksLastBlock(t *testing.T) {
	// ffmpeg can print other brace-y things earlier (e.g. side data dumps).
	txt := `{"unrelated":"1"}
[Parsed_loudnorm_0 @ 0x1]
{
	"input_i" : "-20.00",
	"input_tp" : "-3.00",
	"input_lra" : "5.00",
	"input_thresh" : "-30.00",
	"output_i" : "-14.00",
	"output_tp" : "-1.00",
	"output_lra" : "5.00",
	"output_thresh" : "-24.00",
	"normalization_type" : "linear",
	"target_offset" : "0.50"
}`
	s, err := ParseLoudnormJSON(txt)
	if err != nil {
		t.Fatal(err)
	}
	if s.InputI != -20 || s.TargetOffset != 0.5 {
		t.Errorf("got %+v", s)
	}
}

func TestFilters(t *testing.T) {
	tg := LoudnormTargets{I: -14, TP: -1, LRA: 11}
	if got, want := MeasureFilter(tg), "loudnorm=I=-14:TP=-1:LRA=11:print_format=json"; got != want {
		t.Errorf("MeasureFilter = %q, want %q", got, want)
	}
	m := LoudnormStats{InputI: -21.81, InputTP: -17.69, InputLRA: 0.1, InputThresh: -31.81, TargetOffset: 0.01}
	got := ApplyFilter(tg, m)
	want := "loudnorm=I=-14:TP=-1:LRA=11:measured_I=-21.81:measured_TP=-17.69:measured_LRA=0.1:measured_thresh=-31.81:offset=0.01:linear=true:print_format=summary"
	if got != want {
		t.Errorf("ApplyFilter =\n %q\nwant\n %q", got, want)
	}
}

func TestParseEBUR128Summary_RealOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/ebur128.txt")
	if err != nil {
		t.Fatal(err)
	}
	l, err := ParseEBUR128Summary(string(data))
	if err != nil {
		t.Fatalf("ParseEBUR128Summary: %v", err)
	}
	if math.Abs(l.Integrated-(-21.8)) > 0.05 {
		t.Errorf("Integrated = %v, want -21.8", l.Integrated)
	}
	if math.Abs(l.Range-0.0) > 0.05 {
		t.Errorf("Range = %v, want 0.0", l.Range)
	}
	if math.Abs(l.TruePeak-(-17.7)) > 0.05 {
		t.Errorf("TruePeak = %v, want -17.7", l.TruePeak)
	}
}

func TestParseEBUR128Summary_Missing(t *testing.T) {
	if _, err := ParseEBUR128Summary("nothing"); err == nil {
		t.Error("expected error")
	}
}
