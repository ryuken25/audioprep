// Package assets embeds static files into the binary.
//
// //go:embed is a compiler directive: at build time the named file is read
// and its bytes become the value of the variable below. The binary is then
// self-contained; there is no icon.png to ship next to the .exe. The
// directive can only reference files inside the package directory, which is
// why the icon lives here instead of being reached from cmd/audioprep.
package assets

import _ "embed"

//go:embed icon.png
var Icon []byte
