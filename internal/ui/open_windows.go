//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// revealInFolder opens Explorer with the file selected.
func revealInFolder(path string) error {
	cmd := exec.Command("explorer.exe", "/select,", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	// Explorer returns a non-zero exit code even on success; ignore it.
	_ = cmd.Start()
	return nil
}
