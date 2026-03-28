//go:build !embedui

package main

import "io/fs"

func embeddedAssets() fs.FS {
	return nil
}
