package ffmpeg

import (
	"strconv"
	"strings"
	"time"
)

// Progress is one snapshot of an ffmpeg encode, emitted every time ffmpeg
// flushes a `-progress` block (roughly twice a second).
type Progress struct {
	OutTime time.Duration // how much of the output has been written
	Total   time.Duration // expected total, 0 when unknown
	Percent float64       // 0..100, or -1 when Total is unknown
	Speed   float64       // encode speed relative to realtime, e.g. 2.5 = 2.5x
	FPS     float64
	Frame   int64
	Done    bool // true on the final "progress=end" block
}

// ETA estimates the remaining wall-clock time from the current speed.
// Returns 0 when it cannot tell.
func (p Progress) ETA() time.Duration {
	if p.Total <= 0 || p.Speed <= 0 || p.OutTime >= p.Total {
		return 0
	}
	remaining := p.Total - p.OutTime
	return time.Duration(float64(remaining) / p.Speed)
}

// progressParser accumulates key=value lines from `ffmpeg -progress pipe:1`
// until it sees the `progress=` line that closes a block, then emits one
// Progress. Keeping the state in a struct makes this trivially unit-testable.
type progressParser struct {
	total time.Duration
	cur   Progress
	// out_time can arrive as out_time_us, out_time_ms and out_time; track
	// which one we already used so the more precise form wins.
	haveUS bool
}

// Feed consumes one line. It returns (snapshot, true) when a block completes.
func (pp *progressParser) Feed(line string) (Progress, bool) {
	key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
	if !ok {
		return Progress{}, false
	}
	val = strings.TrimSpace(val)

	switch key {
	case "out_time_us":
		// Microseconds. The most precise form; prefer it.
		if n, err := strconv.ParseInt(val, 10, 64); err == nil && n >= 0 {
			pp.cur.OutTime = time.Duration(n) * time.Microsecond
			pp.haveUS = true
		}
	case "out_time_ms":
		// Despite the name, ffmpeg prints MICROseconds here (a long-standing
		// quirk). Only use it if out_time_us was not seen in this block.
		if !pp.haveUS {
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n >= 0 {
				pp.cur.OutTime = time.Duration(n) * time.Microsecond
			}
		}
	case "out_time":
		// "00:01:23.456000" as a fallback for very old builds.
		if !pp.haveUS {
			if d, ok := parseClock(val); ok {
				pp.cur.OutTime = d
			}
		}
	case "speed":
		// "2.53x" or "N/A"
		s := strings.TrimSuffix(val, "x")
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			pp.cur.Speed = f
		}
	case "fps":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			pp.cur.FPS = f
		}
	case "frame":
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			pp.cur.Frame = n
		}
	case "progress":
		snap := pp.cur
		snap.Total = pp.total
		snap.Done = val == "end"
		snap.Percent = percent(snap.OutTime, pp.total, snap.Done)
		// Reset per-block flags but keep cumulative values so a block that
		// omits a key still reports the last known number.
		pp.haveUS = false
		return snap, true
	}
	return Progress{}, false
}

func percent(out, total time.Duration, done bool) float64 {
	if done {
		return 100
	}
	if total <= 0 {
		return -1
	}
	p := float64(out) / float64(total) * 100
	if p < 0 {
		p = 0
	}
	if p > 99.9 {
		p = 99.9 // never claim 100 until ffmpeg says end
	}
	return p
}

// parseClock parses "HH:MM:SS.ffffff". Negative values appear briefly at the
// start of some encodes; treat them as zero.
func parseClock(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		return 0, true
	}
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	sec, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	total := float64(h)*3600 + float64(m)*60 + sec
	return time.Duration(total * float64(time.Second)), true
}
