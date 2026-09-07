//go:build !windows

package ui

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

func revealInFolder(path string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path).Start()
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}
