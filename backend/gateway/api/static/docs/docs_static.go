package docs_static

import (
	"embed"
)

//go:embed scalar.js powerlab.svg
var EmbeddedAssets embed.FS

const (
	ScalarJSName    = "scalar.js"
	PowerLabLogoSVG = "powerlab.svg"
)
