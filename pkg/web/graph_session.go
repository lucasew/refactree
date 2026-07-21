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

// sessionJob is work for the explore worker (inbox).
type sessionJob struct {
	op  string // visit | project
	ref string // visit ref; empty for project
}

// graphExploreSession holds per-tab edge deltas + extract corpus (no re-read).
//
// Pipeline (non-blocking for the WebSocket read loop):
//
//	read loop  → inbox  → explore worker (StreamVisit / StreamProject)
//	                     → outbox → write loop → WebSocket
//
// Heavy package/file work never runs on the socket read or write goroutines.
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

// sendOut pushes a client message. Returns false if the session is shutting down
// (caller should stop emitting / exploring).
func sendOut(ctx context.Context, outbox chan<- graphSessionOut, msg graphSessionOut) bool {
	select {
	case <-ctx.Done():
		return false
	case outbox <- msg:
		return true
	}
}

// handleGraphSession is a long-lived WebSocket explore session.
//
//	WS /api/graph/session
//	→ {"op":"visit","ref":"…"}  /  {"op":"project"}
//	← focus / edge* (deltas) / done
//
// Architecture:
//
//	• read loop: parse JSON, enqueue visit/project on inbox (never explores)
//	• worker: one explore at a time from inbox; emits to outbox
//	• write loop: sole owner of conn writes (JSON + pings)
//
// FileExtracts stay in SessionCorpus for the life of the socket.
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// inbox: pending package/ref explore jobs (buffered so read loop never blocks on work)
	// size 16: client may click several nodes while one MaterializeVisit runs
	inbox := make(chan sessionJob, 16)
	// outbox: unbuffered — backpressure if the socket is slow (worker waits for writer)
	outbox := make(chan graphSessionOut)

	sess := newGraphExploreSession(s.loader.RootDir)

	var wg sync.WaitGroup

	// --- write loop: only goroutine that writes to conn ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ping.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case msg, ok := <-outbox:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				if err := conn.WriteJSON(msg); err != nil {
					return
				}
			}
		}
	}()

	// --- explore worker: packages/refs from inbox → events on outbox ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case job, ok := <-inbox:
				if !ok {
					return
				}
				switch job.op {
				case "visit":
					ref := job.ref
					if ref == "" {
						ref = "path:./"
					}
					sess.runVisit(ctx, ref, outbox)
				case "project":
					sess.runProject(ctx, outbox)
				}
			}
		}
	}()

	// ready (after pipelines exist)
	if !sendOut(ctx, outbox, graphSessionOut{Type: "ready"}) {
		cancel()
		wg.Wait()
		return
	}

	// --- read loop: enqueue only (never Materialize / discover) ---
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		var in graphSessionIn
		if err := json.Unmarshal(data, &in); err != nil {
			_ = sendOut(ctx, outbox, graphSessionOut{Type: "error", Message: "invalid json"})
			continue
		}

		switch in.Op {
		case "ping":
			_ = sendOut(ctx, outbox, graphSessionOut{Type: "pong"})
		case "visit":
			ref := in.Ref
			if ref == "" {
				ref = "path:./"
			}
			job := sessionJob{op: "visit", ref: ref}
			select {
			case <-ctx.Done():
				// writer/worker already exiting
			case inbox <- job:
				// queued for explore worker
			default:
				// inbox full: never block the read loop
				_ = sendOut(ctx, outbox, graphSessionOut{
					Type:     "error",
					Message:  "explore inbox full; try again",
					VisitRef: ref,
				})
			}
		case "project":
			job := sessionJob{op: "project"}
			select {
			case <-ctx.Done():
			case inbox <- job:
			default:
				_ = sendOut(ctx, outbox, graphSessionOut{
					Type:     "error",
					Message:  "explore inbox full; try again",
					VisitRef: "project",
				})
			}
		default:
			_ = sendOut(ctx, outbox, graphSessionOut{Type: "error", Message: "unknown op: " + in.Op})
		}
	}

	cancel()
	wg.Wait()
}

func (s *graphExploreSession) runVisit(ctx context.Context, ref string, outbox chan<- graphSessionOut) {
	emit := s.deltaEmitter(ctx, outbox)
	_ = s.corpus.StreamVisit(ctx, ref, emit)
	inc := true
	_ = sendOut(ctx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: ref})
}

func (s *graphExploreSession) runProject(ctx context.Context, outbox chan<- graphSessionOut) {
	emit := s.deltaEmitter(ctx, outbox)
	_ = s.corpus.StreamProject(ctx, emit)
	inc := true
	_ = sendOut(ctx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: "project"})
}

func (s *graphExploreSession) deltaEmitter(ctx context.Context, outbox chan<- graphSessionOut) graphql.StreamEmitter {
	return func(ev graphql.StreamEvent) bool {
		if ctx.Err() != nil {
			return false
		}
		switch ev.Type {
		case "focus":
			return sendOut(ctx, outbox, graphSessionOut{Type: "focus", Node: ev.Node, Incomplete: ev.Incomplete})
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
			return sendOut(ctx, outbox, graphSessionOut{Type: "edge", Edge: ev.Edge, Incomplete: ev.Incomplete})
		case "error":
			_ = sendOut(ctx, outbox, graphSessionOut{Type: "error", Message: ev.Message})
			return true
		case "done":
			return true
		default:
			return true
		}
	}
}
