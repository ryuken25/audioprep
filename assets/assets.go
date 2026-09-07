// Package assets embeds static files into the binary.
//
// //go:embed is a compiler directive: at build time the named file is read
// and its bytes become the value of the variable below. The binary is then
// self-contained; there is no icon.png or world.jpg to ship next to the .exe.
// The directive can only reference files inside the package directory, which
// is why they live here instead of being reached from cmd/audioprep.
package assets

import _ "embed"

// Icon is the app icon: the owner's avatar in a rounded square.
//
//go:embed icon.png
var Icon []byte

// World is the VRChat-skin backdrop, pre-cropped to the window aspect.
//
//go:embed world.jpg
var World []byte
