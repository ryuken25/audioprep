package ffmpeg

import (
	"strings"
	"testing"
	"time"
)

// Captured from a real `ffmpeg -progress pipe:1` run; note out_time_ms is
// microseconds despite its name.
const progressBlock = `frame=75
fps=0.00
stream_0_0_q=-1.0
bitrate= 651.2kbits/s
total_size=203509
out_time_us=2500000
out_time_ms=2500000
out_time=00:00:02.500000
dup_frames=0
drop_frames=0
speed=39.3x
progress=continue
frame=150
fps=30.00
out_time_us=5000000
out_time_ms=5000000
out_time=00:00:05.000000
speed=40.0x
progress=end
`

func feedAll(pp *progressParser, text string) []Progress {
	var out []Progress
	for _, line := range strings.Split(text, "\n") {
		if p, ok := pp.Feed(line); ok {
			out = append(out, p)
		}
	}
	return out
}

func TestProgressParser_RealBlocks(t *testing.T) {
	pp := &progressParser{total: 5 * time.Second}
	got := feedAll(pp, progressBlock)
	if len(got) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(got))
	}
	first, last := got[0], got[1]

	if first.OutTime != 2500*time.Millisecond {
		t.Errorf("first.OutTime = %v, want 2.5s", first.OutTime)
	}
	if first.Percent < 49.9 || first.Percent > 50.1 {
		t.Errorf("first.Percent = %v, want 50", first.Percent)
	}
	if first.Speed != 39.3 || first.Frame != 75 || first.Done {
		t.Errorf("first = %+v", first)
	}

	if !last.Done || last.Percent != 100 || last.OutTime != 5*time.Second || last.FPS != 30 {
		t.Errorf("last = %+v", last)
	}
}

func TestProgressParser_OutTimeMsIsMicroseconds(t *testing.T) {
	// Builds without out_time_us still print out_time_ms in microseconds.
	pp := &progressParser{total: 10 * time.Second}
	got := feedAll(pp, "out_time_ms=5000000\nprogress=continue\n")
	if len(got) != 1 || got[0].OutTime != 5*time.Second {
		t.Fatalf("got %+v, want OutTime 5s", got)
	}
}

func TestProgressParser_OutTimeFallback(t *testing.T) {
	pp := &progressParser{total: 100 * time.Second}
	got := feedAll(pp, "out_time=00:01:23.500000\nprogress=continue\n")
	if len(got) != 1 || got[0].OutTime != 83500*time.Millisecond {
		t.Fatalf("got %+v, want 1m23.5s", got)
	}
	// Negative start times happen briefly; must clamp to 0, not error.
	got = feedAll(&progressParser{}, "out_time=-00:00:00.020000\nprogress=continue\n")
	if len(got) != 1 || got[0].OutTime != 0 {
		t.Fatalf("negative out_time: got %+v", got)
	}
}

func TestProgressParser_UnknownTotal(t *testing.T) {
	pp := &progressParser{} // total 0
	got := feedAll(pp, "out_time_us=1000000\nprogress=continue\n")
	if got[0].Percent != -1 {
		t.Errorf("Percent = %v, want -1 for unknown total", got[0].Percent)
	}
	if got[0].ETA() != 0 {
		t.Errorf("ETA = %v, want 0", got[0].ETA())
	}
}

func TestProgressParser_NeverHits100BeforeEnd(t *testing.T) {
	pp := &progressParser{total: time.Second}
	got := feedAll(pp, "out_time_us=1200000\nprogress=continue\n")
	if got[0].Percent >= 100 {
		t.Errorf("Percent = %v before end", got[0].Percent)
	}
}

func TestProgress_ETA(t *testing.T) {
	p := Progress{OutTime: 10 * time.Second, Total: 40 * time.Second, Speed: 2}
	if eta := p.ETA(); eta != 15*time.Second {
		t.Errorf("ETA = %v, want 15s", eta)
	}
	p.Speed = 0
	if eta := p.ETA(); eta != 0 {
		t.Errorf("ETA with zero speed = %v", eta)
	}
}

func TestProgressParser_SpeedNA(t *testing.T) {
	pp := &progressParser{total: time.Second}
	got := feedAll(pp, "speed=N/A\nout_time_us=0\nprogress=continue\n")
	if got[0].Speed != 0 {
		t.Errorf("Speed = %v for N/A", got[0].Speed)
	}
}
