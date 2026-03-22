// Package ui serves minimal HTML (html/template) and embedded Tailwind-built CSS for taskapi.
// Register returns an error if embedded templates or static files cannot be loaded (build/packaging issue).
//
// After changing Tailwind classes in templates, run: npm run build:css (repo root).
//
// Tests: ui_test.go exercises GET / and GET /static/app.css on a mux.
package ui
