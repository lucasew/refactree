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
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
}

type graphSessionIn struct {
	Op  string `json:"op"`  // visit | project | ping
	Ref string `json:"ref"` // for visit
}

type graphSessionOut struct {
	Type       string             `json:"type"` // ready | focus | edge | done | error | pong
	Node       *graphql.GraphNode `json:"node,omitempty"`
	Edge       *graphql.GraphEdge `json:"edge,omitempty"`
	Incomplete *bool              `json:"incomplete,omitempty"`
	Message    string             `json:"message,omitempty"`
	VisitRef   string             `json:"visitRef,omitempty"`
}

// graphExploreSession holds per-tab edge deltas + extract corpus (no re-read).
//
// Core explore loop (see SessionCorpus.StreamVisit):
//
//	discover visit closure (one multi-seed BFS) → Touch extracts once
//	MaterializeVisit(closure) → stream edges → session seen dedupes wire traffic
type graphExploreSession struct {
	root   string
	corpus *graphql.SessionCorpus
	mu     sync.Mutex
	seen   map[string]bool // edge keys already sent
}

func newGraphExploreSession(root string) *graphExploreSession {
	return &graphExploreSession{
		root:   root,
		corpus: graphql.NewSessionCorpus(root),
		seen:   make(map[string]bool),
	}
}

func edgeSeenKey(e *graphql.GraphEdge) string {
	if e == nil {
		return ""
	}
	return string(e.Kind) + "\x00" + e.From + "\x00" + e.To
}

// handleGraphSession is a long-lived WebSocket explore session.
//
//	WS /api/graph/session
//	→ {"op":"visit","ref":"…"}  /  {"op":"project"}
//	← focus / edge* (deltas) / done
//
// FileExtracts are kept in SessionCorpus for the life of the socket so each
// project file is parsed at most once per session (mtime cache still applies).
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
	_ = s.corpus.StreamVisit(ctx, ref, emit)
	inc := true
	_ = write(graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: ref})
}

func (s *graphExploreSession) handleProject(ctx context.Context, write func(graphSessionOut) error) {
	emit := s.deltaEmitter(write)
	_ = s.corpus.StreamProject(ctx, emit)
	inc := true
	_ = write(graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: "project"})
}

func (s *graphExploreSession) deltaEmitter(write func(graphSessionOut) error) graphql.StreamEmitter {
	return func(ev graphql.StreamEvent) bool {
		switch ev.Type {
		case "focus":
			return write(graphSessionOut{Type: "focus", Node: ev.Node, Incomplete: ev.Incomplete}) == nil
		case "edge":
			if ev.Edge == nil {
				return true
			}
			k := edgeSeenKey(ev.Edge)
			s.mu.Lock()
			if s.seen[k] {
				s.mu.Unlock()
				return true
			}
			s.seen[k] = true
			s.mu.Unlock()
			return write(graphSessionOut{Type: "edge", Edge: ev.Edge, Incomplete: ev.Incomplete}) == nil
		case "error":
			_ = write(graphSessionOut{Type: "error", Message: ev.Message})
			return true
		case "done":
			return true
		default:
			return true
		}
	}
}
