//go:build windows

package pipeline

// nullDevice is where "-f null" output goes. ffmpeg accepts "-" everywhere,
// but on Windows using NUL avoids any chance of a console handle mix-up when
// the app is built without a console.
const nullDevice = "NUL"
