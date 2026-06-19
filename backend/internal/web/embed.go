package web

import (
	"embed"
	"io/fs"
)

// dist holds the compiled Vue frontend, baked into the binary at build time.
//
//go:embed all:dist
var dist embed.FS

// FS returns the frontend file system rooted at the dist directory.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
