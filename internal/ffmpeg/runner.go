// Package ffmpeg wraps the ffmpeg and ffprobe command-line tools.
//
// Nothing in here knows about presets or the UI. It knows how to find the
// binaries, run them with a cancellable context, stream progress back, and
// turn their text output into Go values.
package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Runner executes ffmpeg/ffprobe. Create one at startup after Locate() has
// found the binaries and pass it around; it holds no per-job state, so a
// single Runner can be shared by every job.
type Runner struct {
	FFmpeg  string // absolute path to ffmpeg(.exe)
	FFprobe string // absolute path to ffprobe(.exe)

	// Log, when set, receives every line ffmpeg writes to stderr plus the
	// command lines themselves. The UI wires this to its log panel.
	// It may be called from any goroutine.
	Log func(line string)
}

// maxStderrBytes caps how much stderr we keep in memory per run. loudnorm's
// JSON and ebur128's summary are both at the very end, and error messages are
// too, so keeping the tail is enough.
const maxStderrBytes = 512 * 1024

// RunError is returned when ffmpeg exits non-zero. It carries the last lines
// of stderr so the UI can show "human sentence first, raw ffmpeg tail below".
type RunError struct {
	Err    error  // the underlying exec error (exit status, killed, ...)
	Tail   string // last ~20 lines of stderr
	Killed bool   // true when the context was cancelled
}

func (e *RunError) Error() string {
	if e.Killed {
		return "ffmpeg was cancelled"
	}
	return fmt.Sprintf("ffmpeg failed: %v\n%s", e.Err, e.Tail)
}

// Unwrap lets errors.Is(err, context.Canceled) work through RunError.
// This is Go's error-wrapping convention: any type with an Unwrap() method
// participates in errors.Is / errors.As chains.
func (e *RunError) Unwrap() error { return e.Err }

// Run executes ffmpeg with args, streaming Progress values to progress (if
// non-nil) until the process exits. total is the expected output duration and
// is only used to compute percentages; pass 0 when unknown.
//
// The full stderr text (bounded) is returned so callers can parse loudnorm
// JSON or ebur128 summaries out of it.
//
// Cancelling ctx kills the ffmpeg process. On Windows that is a hard
// TerminateProcess, which is exactly what we want for a Cancel button.
func (r *Runner) Run(ctx context.Context, args []string, total time.Duration, progress chan<- Progress) (string, error) {
	full := make([]string, 0, len(args)+6)
	// -hide_banner: skip the build-config wall of text.
	// -nostdin:     never wait on the keyboard (we have no console anyway).
	// -nostats:     suppress the "\r frame=..." ticker; we use -progress.
	// -progress pipe:1: machine-readable progress on stdout.
	full = append(full, "-hide_banner", "-nostdin", "-nostats", "-progress", "pipe:1", "-y")
	full = append(full, args...)

	// exec.CommandContext ties the process lifetime to ctx: when ctx is
	// cancelled the process is killed. We never build a shell string; args
	// go straight to CreateProcess, so filenames with spaces or quotes are
	// safe.
	cmd := exec.CommandContext(ctx, r.FFmpeg, full...)
	hideConsole(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	r.logf("$ ffmpeg %s", strings.Join(quoteArgs(full), " "))

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start ffmpeg: %w", err)
	}

	// Both pipes must be drained concurrently. If we read stderr to the end
	// before touching stdout, ffmpeg blocks once the stdout pipe buffer fills
	// and the whole thing deadlocks. Two goroutines and a WaitGroup solve it:
	// a goroutine is a cheap concurrent function call, and the WaitGroup lets
	// us block until both readers have finished.
	var wg sync.WaitGroup
	var errText strings.Builder
	var errMu sync.Mutex // protects errText, which both the reader and the caller touch

	wg.Add(2)
	go func() {
		defer wg.Done()
		pp := &progressParser{total: total}
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if snap, ok := pp.Feed(sc.Text()); ok && progress != nil {
				// Non-blocking send: if the UI is slow to consume, drop the
				// snapshot rather than stall ffmpeg. The next block arrives
				// in half a second anyway.
				select {
				case progress <- snap:
				default:
				}
			}
		}
	}()
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderr)
		// loudnorm's JSON lines are short, but some ffmpeg warnings are long;
		// give the scanner a generous buffer.
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			r.logf("%s", line)
			errMu.Lock()
			appendBounded(&errText, line)
			errMu.Unlock()
		}
	}()

	wg.Wait()
	waitErr := cmd.Wait()

	errMu.Lock()
	text := errText.String()
	errMu.Unlock()

	if waitErr != nil {
		killed := ctx.Err() != nil
		return text, &RunError{Err: waitErr, Tail: tail(text, 20), Killed: killed}
	}
	return text, nil
}

// appendBounded keeps the builder under maxStderrBytes by dropping the
// oldest half when it grows too large. Cheap and good enough for logs.
func appendBounded(b *strings.Builder, line string) {
	if b.Len()+len(line)+1 > maxStderrBytes {
		s := b.String()
		b.Reset()
		b.WriteString(s[len(s)/2:])
	}
	b.WriteString(line)
	b.WriteByte('\n')
}

// tail returns the last n lines of s.
func tail(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func (r *Runner) logf(format string, a ...any) {
	if r.Log != nil {
		r.Log(fmt.Sprintf(format, a...))
	}
}

// quoteArgs makes a copy-pasteable command line for the log. It is purely
// cosmetic; the real process never sees this string.
func quoteArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\"'") {
			out[i] = `"` + strings.ReplaceAll(a, `"`, `\"`) + `"`
		} else {
			out[i] = a
		}
	}
	return out
}

// asExitError is a tiny helper around errors.As so probe.go reads cleanly.
func asExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}

// Version returns the first line of `ffmpeg -version`, e.g.
// "ffmpeg version N-118000-g... Copyright ...". Used for the status bar.
func Version(ctx context.Context, ffmpegPath string) (string, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, "-version")
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(out), "\n")
	line = strings.TrimSpace(line)
	// Trim the copyright clause; the UI only has one line of room.
	if i := strings.Index(line, " Copyright"); i > 0 {
		line = line[:i]
	}
	return line, nil
}

// HasEncoder reports whether the ffmpeg build lists the named encoder in
// `ffmpeg -encoders`. Used to pick libfdk_aac over the native aac encoder
// when it is available.
func HasEncoder(ctx context.Context, ffmpegPath, name string) bool {
	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-encoders")
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return hasEncoderIn(string(out), name)
}

// hasEncoderIn is the pure part of HasEncoder, for tests. Encoder lines look
// like " A....D aac                  AAC (Advanced Audio Coding)".
func hasEncoderIn(listing, name string) bool {
	sc := bufio.NewScanner(strings.NewReader(listing))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[1] == name {
			return true
		}
	}
	return false
}
