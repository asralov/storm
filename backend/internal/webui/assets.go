//go:build production

package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var bundle embed.FS

func Assets() (fs.FS, error) { return fs.Sub(bundle, "dist") }
