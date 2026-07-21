package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/lucasew/refactree/pkg/web/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

//go:embed templates/*.html static/*
var embedded embed.FS

// distFS is the Bun SPA build. Populated via //go:embed dist/* in spa_embed.go
// when dist assets exist.

// Server serves the code browser (SPA + GraphQL).
type Server struct {
	loader *Loader
	// corpus is shared across GraphQL code loads and all graph WS sessions so
	// file-browser opens and crawler visits hit the same extract cache.
	corpus *graphql.SessionCorpus
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

	s := &Server{
		loader: loader,
		corpus: graphql.NewSessionCorpus(opts.RootDir),
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return recoverHandler(s.mux)
}

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
	// Legacy static (CSS shared tokens) + SPA assets.
	if staticFS, err := fs.Sub(embedded, "static"); err == nil {
		s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}
	if spa, err := spaFileSystem(); err == nil {
		s.mux.Handle("/assets/", http.FileServer(http.FS(spa)))
	}

	gqlSrv := handler.New(graphql.NewExecutableSchema(graphql.Config{
		Resolvers: &graphql.Resolver{Store: &GraphStore{Loader: s.loader, Corpus: s.corpus}},
	}))
	gqlSrv.AddTransport(transport.Options{})
	gqlSrv.AddTransport(transport.GET{})
	gqlSrv.AddTransport(transport.POST{})
	gqlSrv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	gqlSrv.Use(extension.Introspection{})
	gqlSrv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	s.mux.Handle("/graphql", gqlSrv)
	s.mux.HandleFunc("/api/graph/session", s.handleGraphSession)
	s.mux.Handle("/playground", playground.Handler("refactree", "/graphql"))

	// SPA shell for / and /code/...
	s.mux.HandleFunc("/", s.handleSPA)
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	// Asset files under embed root (vite may emit at /assets/ already handled).
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		http.NotFound(w, r)
		return
	}
	data, err := spaIndexHTML()
	if err != nil {
		// Fallback message when dist not built.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>
<p>refactree SPA not built. Run <code>mise run frontend:build</code> (or <code>bun run build</code>) then rebuild rft.</p>
</body></html>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// ListenAndServe starts HTTP on addr (e.g. "127.0.0.1:8080").
func (s *Server) ListenAndServe(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return srv.ListenAndServe()
}

// TrimTrailingSlash is a small helper for tests/templates.
func TrimTrailingSlash(p string) string {
	return strings.TrimRight(p, "/")
}
