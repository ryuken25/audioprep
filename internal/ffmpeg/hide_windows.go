//go:build windows

package ffmpeg

import (
	"os/exec"
	"syscall"
)

// hideConsole stops ffmpeg from flashing a black console window every time
// we launch it. The app itself is built with -H windowsgui, but child
// processes get their own console unless we say otherwise.
//
// The //go:build line at the top is a build constraint: this file is only
// compiled on Windows. hide_other.go provides the no-op twin for everything
// else, so callers can use hideConsole() unconditionally.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
