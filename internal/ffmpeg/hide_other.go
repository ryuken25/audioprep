//go:build !windows

package ffmpeg

import "os/exec"

// hideConsole is a no-op outside Windows; see hide_windows.go.
func hideConsole(cmd *exec.Cmd) {}
