package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lucasew/refactree/pkg/web/graphql"
)

var graphUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Local code browser; loopback-first by default.
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}

// client → server
type graphSessionIn struct {
	Op  string `json:"op"`  // visit | project | ping
	Ref string `json:"ref"` // for visit
}

// server → client reuses graphql.StreamEvent JSON shape (+ op-level ready)
type graphSessionOut struct {
	Type       string             `json:"type"` // ready | focus | edge | done | error | pong
	Node       *graphql.GraphNode `json:"node,omitempty"`
	Edge       *graphql.GraphEdge `json:"edge,omitempty"`
	Incomplete *bool              `json:"incomplete,omitempty"`
	Message    string             `json:"message,omitempty"`
	// VisitRef is set on done so the client knows which visit finished.
	VisitRef string `json:"visitRef,omitempty"`
}

// graphExploreSession accumulates edges already pushed to one browser tab.
type graphExploreSession struct {
	root string
	mu   sync.Mutex
	seen map[string]bool // edge keys already sent
}

func newGraphExploreSession(root string) *graphExploreSession {
	return &graphExploreSession{
		root: root,
		seen: make(map[string]bool),
	}
}

func edgeSeenKey(e *graphql.GraphEdge) string {
	if e == nil {
		return ""
	}
	return string(e.Kind) + "\x00" + e.From + "\x00" + e.To
}

// handleGraphSession is a long-lived WebSocket: client visits nodes; server
// pushes only edges not yet seen in this session.
//
//	WS /api/graph/session
//	→ {"op":"visit","ref":"path:./foo.go::Bar"}
//	→ {"op":"project"}
//	← focus / edge* / done  (deltas only)
func (s *Server) handleGraphSession(w http.ResponseWriter, r *http.Request) {
	if s.loader == nil {
		http.Error(w, "loader not configured", http.StatusInternalServerError)
		return
	}
	conn, err := graphUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("graph session upgrade", "err", err)
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	})

	sess := newGraphExploreSession(s.loader.RootDir)
	writeMu := sync.Mutex{}
	writeJSON := func(out graphSessionOut) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		return conn.WriteJSON(out)
	}

	if err := writeJSON(graphSessionOut{Type: "ready"}); err != nil {
		return
	}

	// Heartbeat
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		t := time.NewTicker(25 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				writeMu.Lock()
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(10*time.Second))
				writeMu.Unlock()
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		var in graphSessionIn
		if err := json.Unmarshal(data, &in); err != nil {
			_ = writeJSON(graphSessionOut{Type: "error", Message: "invalid json"})
			continue
		}

		switch in.Op {
		case "ping":
			_ = writeJSON(graphSessionOut{Type: "pong"})
		case "visit":
			ref := in.Ref
			if ref == "" {
				ref = "path:./"
			}
			sess.handleVisit(ctx, ref, writeJSON)
		case "project":
			sess.handleProject(ctx, writeJSON)
		default:
			_ = writeJSON(graphSessionOut{Type: "error", Message: "unknown op: " + in.Op})
		}
	}
}

func (s *graphExploreSession) handleVisit(ctx context.Context, ref string, write func(graphSessionOut) error) {
	emit := s.deltaEmitter(write)
	_ = graphql.StreamNeighborhood(ctx, s.root, ref, emit)
	inc := true
	_ = write(graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: ref})
}

func (s *graphExploreSession) handleProject(ctx context.Context, write func(graphSessionOut) error) {
	emit := s.deltaEmitter(write)
	_ = graphql.StreamProjectGraph(ctx, s.root, emit)
	inc := true
	_ = write(graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: "project"})
}

// deltaEmitter wraps StreamEmitter: forwards focus always; edges only if new to session.
func (s *graphExploreSession) deltaEmitter(write func(graphSessionOut) error) graphql.StreamEmitter {
	return func(ev graphql.StreamEvent) bool {
		switch ev.Type {
		case "focus":
			out := graphSessionOut{Type: "focus", Node: ev.Node, Incomplete: ev.Incomplete}
			return write(out) == nil
		case "edge":
			if ev.Edge == nil {
				return true
			}
			k := edgeSeenKey(ev.Edge)
			s.mu.Lock()
			if s.seen[k] {
				s.mu.Unlock()
				return true // already pushed this session
			}
			s.seen[k] = true
			s.mu.Unlock()
			out := graphSessionOut{Type: "edge", Edge: ev.Edge, Incomplete: ev.Incomplete}
			return write(out) == nil
		case "error":
			_ = write(graphSessionOut{Type: "error", Message: ev.Message})
			return true
		case "done":
			// per-stream done suppressed; visit/project sends its own done after
			return true
		case "node":
			// nodes on demand — ignore bulk nodes if any
			return true
		default:
			return true
		}
	}
}
