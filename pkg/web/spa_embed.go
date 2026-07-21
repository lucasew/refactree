package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:dist
var spaDist embed.FS

func spaFileSystem() (fs.FS, error) {
	sub, err := fs.Sub(spaDist, "dist")
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func spaIndexHTML() ([]byte, error) {
	b, err := spaDist.ReadFile("dist/index.html")
	if err != nil {
		return nil, fmt.Errorf("spa index: %w (run mise run frontend:build)", err)
	}
	return b, nil
}
