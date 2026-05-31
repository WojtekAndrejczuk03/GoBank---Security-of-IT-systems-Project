package web

import "embed"

// Templates holds all HTML template files embedded into the binary.
//go:embed templates
var Templates embed.FS
