package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFiles embed.FS

// GetDistFS returns the embedded dist filesystem
func GetDistFS() (fs.FS, error) {
	return fs.Sub(distFiles, "dist")
}
