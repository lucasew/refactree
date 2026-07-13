package web

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lucasew/refactree/pkg/ingest"
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
	return recoverHandler(s.mux)
}

// recoverHandler keeps the process alive if a request panics (e.g. third-party
// parse faults). Concurrent tree-sitter use is serialized separately in ingest.
func recoverHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("refactree serve: panic", "method", r.Method, "path", r.URL.Path, "panic", rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
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

	// Core canonicalization over the ingest graph (any provider); redirect if changed.
	if s.loader != nil {
		parsed := ingest.ParseReference(ref)
		canon := ingest.CanonicalizeReference(s.loader.RootDir, parsed).String()
		if canon != "" && canon != ref {
			http.Redirect(w, r, EncodeCodeURL(canon), http.StatusFound)
			return
		}
	}

	v := s.loader.LoadFile(ref)
	s.render(w, "code.html", v)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ListenAndServe starts HTTP on addr (e.g. "127.0.0.1:8080").
// Timeouts bound slowloris and hung connections; WriteTimeout is generous so
// large annotated pages still finish.
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

// TrimTrailingSlash is a small helper for tests/templates.
func TrimTrailingSlash(p string) string {
	return strings.TrimRight(p, "/")
}
