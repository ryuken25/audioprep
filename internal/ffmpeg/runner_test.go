package ffmpeg

import (
	"strings"
	"testing"
)

const encoderListing = `Encoders:
 V..... = Video
 A..... = Audio
 ------
 V....D libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
 A....D aac                  AAC (Advanced Audio Coding)
 A....D libopus              libopus Opus (codec opus)
`

func TestHasEncoderIn(t *testing.T) {
	if !hasEncoderIn(encoderListing, "aac") {
		t.Error("aac should be found")
	}
	if !hasEncoderIn(encoderListing, "libx264") {
		t.Error("libx264 should be found")
	}
	if hasEncoderIn(encoderListing, "libfdk_aac") {
		t.Error("libfdk_aac should NOT be found in a gpl build listing")
	}
	// Must match whole tokens: "aac" must not match "libfdk_aac" and vice versa.
	if hasEncoderIn(" A....D libfdk_aac  Fraunhofer", "aac") {
		t.Error("substring match is wrong")
	}
}

func TestTail(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	text := strings.Join(lines, "\n") + "\n"
	if got := tail(text, 2); got != "d\ne" {
		t.Errorf("tail(2) = %q", got)
	}
	if got := tail(text, 10); got != "a\nb\nc\nd\ne" {
		t.Errorf("tail(10) = %q", got)
	}
	if got := tail("", 3); got != "" {
		t.Errorf("tail(empty) = %q", got)
	}
}

func TestQuoteArgs(t *testing.T) {
	got := quoteArgs([]string{"-i", `C:\My Videos\take 1.mp4`, "-crf", "23"})
	want := []string{"-i", `"C:\My Videos\take 1.mp4"`, "-crf", "23"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("quoteArgs = %v, want %v", got, want)
	}
}

func TestAppendBounded(t *testing.T) {
	var b strings.Builder
	big := strings.Repeat("x", 1000)
	for i := 0; i < 2000; i++ { // ~2 MB of input
		appendBounded(&b, big)
	}
	if b.Len() > maxStderrBytes+len(big)+1 {
		t.Errorf("builder grew to %d bytes, cap is %d", b.Len(), maxStderrBytes)
	}
	if !strings.HasSuffix(b.String(), big+"\n") {
		t.Error("last line was lost")
	}
}

func TestRunError_Unwrap(t *testing.T) {
	inner := &RunError{Err: errSentinel, Tail: "boom", Killed: false}
	if !strings.Contains(inner.Error(), "boom") {
		t.Error("tail missing from message")
	}
	if inner.Unwrap() != errSentinel {
		t.Error("Unwrap did not return inner error")
	}
	killed := &RunError{Err: errSentinel, Killed: true}
	if !strings.Contains(killed.Error(), "cancel") {
		t.Errorf("killed message = %q", killed.Error())
	}
}

type sentinel struct{}

func (sentinel) Error() string { return "sentinel" }

var errSentinel error = sentinel{}
