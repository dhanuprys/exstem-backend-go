package fonts

import "embed"

// FS exposes the statically embedded TTF fonts mapped at compile-time.
//
//go:embed *.ttf
var FS embed.FS
