package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/app.css
var staticFS embed.FS

const logCmd = "taskapi"

// Register adds HTML routes on mux: GET / (home) and GET /static/* (compiled Tailwind CSS).
// Register before mounting the JSON API with Handle("/", api) so GET / and /static/ win.
func Register(mux *http.ServeMux) error {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "home.html", nil); err != nil {
			slog.Error("template render failed", "cmd", logCmd, "operation", "ui.home", "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	})

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("static embed: %w", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	return nil
}
