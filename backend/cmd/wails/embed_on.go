//go:build embedui

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDist embed.FS

func embeddedAssets() fs.FS {
	return embeddedDist
}
