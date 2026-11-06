// Package dashboardassets contains reproducibly generated, tracked dashboard assets.
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
