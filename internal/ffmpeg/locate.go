package ffmpeg

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ErrNotFound is returned by Locate when neither binary can be found.
var ErrNotFound = errors.New("ffmpeg and ffprobe not found")

// exeName appends ".exe" on Windows so the same code works everywhere.
func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// AppDataBinDir returns the per-user folder where a downloaded ffmpeg lives:
// %APPDATA%\audioprep\bin on Windows, ~/.config/audioprep/bin elsewhere.
func AppDataBinDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "audioprep", "bin"), nil
}

// Locate finds ffmpeg and ffprobe, trying in order:
//  1. the directory containing the running executable
//  2. %APPDATA%\audioprep\bin (where Download puts them)
//  3. PATH
//
// It returns both paths, or ErrNotFound. The search order means a user can
// always override the downloaded copy by dropping their own ffmpeg.exe next
// to audioprep.exe.
func Locate() (ffmpegPath, ffprobePath string, err error) {
	var dirs []string
	if exe, e := os.Executable(); e == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if d, e := AppDataBinDir(); e == nil {
		dirs = append(dirs, d)
	}

	for _, d := range dirs {
		f := filepath.Join(d, exeName("ffmpeg"))
		p := filepath.Join(d, exeName("ffprobe"))
		if fileExists(f) && fileExists(p) {
			return f, p, nil
		}
	}

	// Fall back to PATH. exec.LookPath does the PATHEXT dance on Windows.
	f, e1 := exec.LookPath("ffmpeg")
	p, e2 := exec.LookPath("ffprobe")
	if e1 == nil && e2 == nil {
		return f, p, nil
	}
	return "", "", ErrNotFound
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
