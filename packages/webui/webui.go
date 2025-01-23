package webui

import (
	"embed"
	"io/fs"
	"path"
)

//go:embed dist/*
var distDir embed.FS

var DistDir = DirFS{content: distDir}

type DirFS struct {
	content embed.FS
}

func (c DirFS) Open(name string) (fs.File, error) {
	return c.content.Open(path.Join("dist", name))
}
