package web

import (
	"embed"
	"io/fs"
)

// Assets contains the frontend files served by the management API.
//
//go:embed dist
var builtAssets embed.FS

var Assets fs.FS

func init() {
	var err error
	Assets, err = fs.Sub(builtAssets, "dist")
	if err != nil {
		panic(err)
	}
}
