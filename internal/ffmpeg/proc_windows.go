//go:build windows

package ffmpeg

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// hideConsole stops ffmpeg from flashing a black console window every time
// we launch it. The app itself is built with -H windowsgui, but child
// processes get their own console unless we say otherwise.
//
// The //go:build line at the top is a build constraint: this file is only
// compiled on Windows. proc_other.go provides the twins for everything else,
// so callers can use these functions unconditionally.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// killTree terminates p and every process it started.
//
// exec.CommandContext only kills the direct child. If "ffmpeg" on PATH is a
// wrapper (Chocolatey and Scoop both install shims; a .cmd file is another
// common case), killing the wrapper leaves the real ffmpeg running, still
// holding our pipes and still writing the output file. Cancel would then
// appear to hang. taskkill /T walks the tree and /F terminates each member.
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	tk := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid))
	tk.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = tk.Run() // exits non-zero if the tree is already gone; that is fine
	// Belt and braces in case taskkill is unavailable or refused.
	_ = p.Kill()
	return nil
}
