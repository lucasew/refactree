package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/*.html static/*
var embedded embed.FS

// Server serves the code browser.
type Server struct {
	loader *Loader
	tmpl   *template.Template
	mux    *http.ServeMux
}

// Options configures the web server.
type Options struct {
	RootDir string
}

// New builds a Server rooted at opts.RootDir.
func New(opts Options) (*Server, error) {
	loader, err := NewLoader(opts.RootDir)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"safeURL": func(s string) template.URL { return template.URL(s) },
	}).ParseFS(embedded, "templates/*.html")
	if err != nil {
		return nil, err
	}

	s := &Server{loader: loader, tmpl: tmpl, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	staticFS, err := fs.Sub(embedded, "static")
	if err == nil {
		s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc(CodePathPrefix, s.handleCode)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	v := s.loader.LoadIndex()
	s.render(w, "index.html", v)
}

func (s *Server) handleCode(w http.ResponseWriter, r *http.Request) {
	ref, ok := DecodeCodePath(r.URL.Path)
	if !ok || ref == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Honour ?ref= as an alternate entry (useful for debugging).
	if q := r.URL.Query().Get("ref"); q != "" {
		ref = q
	}

	v := s.loader.LoadFileWithOptions(ref, LoadOptions{
		File: r.URL.Query().Get("file"),
	})
	s.render(w, "code.html", v)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ListenAndServe starts HTTP on addr (e.g. ":8080").
func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

// TrimTrailingSlash is a small helper for tests/templates.
func TrimTrailingSlash(p string) string {
	return strings.TrimRight(p, "/")
}
