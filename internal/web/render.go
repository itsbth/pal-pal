package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"time"
)

//go:embed templates static
var assets embed.FS

type renderer struct {
	pages      map[string]*template.Template
	components map[string]*template.Template
	static     fs.FS
}

func newRenderer() (*renderer, error) {
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"formatTime": func(value time.Time) string {
			if value.IsZero() {
				return "never"
			}
			return value.Format("15:04:05")
		},
		"formatDateTime": func(value time.Time) string {
			return value.Format("02 Jan 15:04")
		},
		"formatDuration": func(seconds int64) string {
			duration := time.Duration(seconds) * time.Second
			days := duration / (24 * time.Hour)
			if days > 0 {
				return fmt.Sprintf("%dd %dh", days, (duration%(24*time.Hour))/time.Hour)
			}
			hours := duration / time.Hour
			if hours > 0 {
				return fmt.Sprintf("%dh %dm", hours, (duration%time.Hour)/time.Minute)
			}
			return fmt.Sprintf("%dm", duration/time.Minute)
		},
	}

	result := &renderer{
		pages:      make(map[string]*template.Template),
		components: make(map[string]*template.Template),
	}

	pageEntries, err := fs.Glob(assets, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("list page templates: %w", err)
	}
	for _, entry := range pageEntries {
		name := entry[len("templates/pages/") : len(entry)-len(".html")]
		parsed, err := template.New("layout").Funcs(funcs).ParseFS(assets, "templates/layout.html", entry)
		if err != nil {
			return nil, fmt.Errorf("parse page %s: %w", name, err)
		}
		result.pages[name] = parsed
	}

	componentEntries, err := fs.Glob(assets, "templates/components/*.html")
	if err != nil {
		return nil, fmt.Errorf("list component templates: %w", err)
	}
	for _, entry := range componentEntries {
		name := entry[len("templates/components/") : len(entry)-len(".html")]
		parsed, err := template.New(name).Funcs(funcs).ParseFS(assets, entry)
		if err != nil {
			return nil, fmt.Errorf("parse component %s: %w", name, err)
		}
		result.components[name] = parsed
	}

	result.static, err = fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("open static assets: %w", err)
	}
	return result, nil
}

func (r *renderer) page(w io.Writer, name string, data any) error {
	tmpl, ok := r.pages[name]
	if !ok {
		return fmt.Errorf("unknown page template %q", name)
	}
	return tmpl.ExecuteTemplate(w, "layout", data)
}

func (r *renderer) component(w io.Writer, name string, data any) error {
	tmpl, ok := r.components[name]
	if !ok {
		return fmt.Errorf("unknown component template %q", name)
	}
	return tmpl.ExecuteTemplate(w, name+".html", data)
}
