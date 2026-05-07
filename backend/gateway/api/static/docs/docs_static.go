package docs_static

import (
	"embed"
)

//go:embed scalar.js
var EmbeddedAssets embed.FS

const (
	ScalarJSName = "scalar.js"
)
