package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lucasew/refactree/pkg/web/graphql"
)

// handleGraphStream serves progressive graph updates as Server-Sent Events.
//
//	GET /api/graph/stream?ref=<reference>     focus neighborhood
//	GET /api/graph/stream?mode=project        project import map
//
// Events: focus | node | edge | done | error (JSON data payloads).
func (s *Server) handleGraphStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.loader == nil {
		http.Error(w, "loader not configured", http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Allow long streams on local tool (ResponseController clears write deadline).
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{}) // no deadline
	}

	ctx := r.Context()
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))

	emit := func(ev graphql.StreamEvent) bool {
		if ctx.Err() != nil {
			return false
		}
		payload, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	var err error
	switch mode {
	case "project", "projectGraph":
		err = graphql.StreamProjectGraph(ctx, s.loader.RootDir, emit)
	default:
		if ref == "" {
			ref = "path:./"
		}
		err = graphql.StreamNeighborhood(ctx, s.loader.RootDir, ref, emit)
	}
	if err != nil && ctx.Err() == nil {
		_ = emit(graphql.StreamEvent{Type: "error", Message: err.Error()})
	}
}
