// Package ui owns shared host templates and static browser assets.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// StaticHandler serves the embedded host browser assets.
func StaticHandler() http.Handler {
	files, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("ui: embedded static directory is missing")
	}
	return http.FileServerFS(files)
}
