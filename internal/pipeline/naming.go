package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OutputPath decides where the result goes: <base>_<presetID><ext> in
// outputDir (or next to the input when outputDir is empty). It never
// overwrites: if the name is taken it appends -1, -2, ... until free.
//
// exists is injected so tests can simulate collisions without touching the
// disk. Passing a function instead of calling os.Stat directly is a common
// Go trick for making filesystem code testable.
func OutputPath(inputPath, outputDir, presetID, ext string, exists func(string) bool) string {
	dir := outputDir
	if dir == "" {
		dir = filepath.Dir(inputPath)
	}
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	stem := fmt.Sprintf("%s_%s", base, presetID)

	candidate := filepath.Join(dir, stem+ext)
	for i := 1; exists(candidate); i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
	}
	return candidate
}

// FileExists is the production implementation of the exists callback.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
