package ui

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/app.css
var staticFS embed.FS

// Register adds HTML routes on mux: GET / (home) and GET /static/* (compiled Tailwind CSS).
// Register before mounting the JSON API with Handle("/", api) so GET / and /static/ win.
func Register(mux *http.ServeMux) {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "home.html", nil); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	})

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("ui: static embed: " + err.Error())
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
}
