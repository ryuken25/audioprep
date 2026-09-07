//go:build !windows

package ffmpeg

import (
	"os"
	"os/exec"
)

// hideConsole is a no-op outside Windows; see proc_windows.go.
func hideConsole(cmd *exec.Cmd) {}

// killTree on Unix just kills the child. The app targets Windows; a process
// group kill (Setpgid) could be added here if that ever changes.
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	return p.Kill()
}
