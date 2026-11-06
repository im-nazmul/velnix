// Package dashboardassets contains dashboard assets generated at build time.
// Run sh scripts/build.sh before compiling the Go application.
package dashboardassets

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var embedded embed.FS

func Files() fs.FS {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return assets
}
