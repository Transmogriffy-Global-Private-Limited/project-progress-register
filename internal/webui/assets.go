package webui

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
)

//go:embed templates/*.html static/*.css
var assets embed.FS

// Templates parses all trusted server-rendered templates.
func Templates() (*template.Template, error) {
	templates, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse web templates: %w", err)
	}
	return templates, nil
}

// Static returns the embedded static asset filesystem rooted at static/.
func Static() (fs.FS, error) {
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("open static assets: %w", err)
	}
	return static, nil
}
