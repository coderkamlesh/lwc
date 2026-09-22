package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// assets are compiled into the binary so Lambda does not need a filesystem
// volume and local development has the same asset behavior as production.
//
//go:embed templates static
var assets embed.FS

type Renderer struct {
	pages map[string]*template.Template
}

func NewRenderer() (*Renderer, error) {
	pageFiles, err := fs.Glob(assets, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("find page templates: %w", err)
	}
	if len(pageFiles) == 0 {
		return nil, fmt.Errorf("no page templates found")
	}

	pages := make(map[string]*template.Template, len(pageFiles))
	for _, pageFile := range pageFiles {
		name := strings.TrimSuffix(path.Base(pageFile), path.Ext(pageFile))
		parsed, err := template.New(name).Funcs(template.FuncMap{
			"formatDate": func(value string) string {
				parsedTime, err := time.Parse(time.RFC3339, value)
				if err != nil {
					return value
				}
				return parsedTime.Format("02 Jan 2006")
			},
		}).ParseFS(
			assets,
			"templates/layouts/base.html",
			pageFile,
			"templates/partials/*.html",
		)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pageFile, err)
		}
		pages[name] = parsed
	}

	return &Renderer{pages: pages}, nil
}

func StaticFS() fs.FS {
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		panic(fmt.Sprintf("sub static assets: %v", err))
	}
	return staticFS
}

func (r *Renderer) Render(w http.ResponseWriter, page string, data any) error {
	return r.execute(w, page, "base", data)
}

func (r *Renderer) RenderPartial(w http.ResponseWriter, page, partial string, data any) error {
	return r.execute(w, page, partial, data)
}

func (r *Renderer) execute(w http.ResponseWriter, page, name string, data any) error {
	parsed, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("page template %q not found", page)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return parsed.ExecuteTemplate(w, name, data)
}
