package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns the embedded SPA filesystem rooted at the build output directory.
func Dist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// can only happen if the embed directive is broken at build time.
		panic(err)
	}
	return sub
}
