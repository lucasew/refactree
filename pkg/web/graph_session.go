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

// sessionJob is work for the explore worker.
type sessionJob struct {
	op  string // visit | project
	ref string // visit ref; empty for project
}

// jobSlot tracks the in-flight explore so a new file-browser click can preempt it.
type jobSlot struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (j *jobSlot) preempt() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cancel != nil {
		j.cancel()
		j.cancel = nil
	}
}

func (j *jobSlot) bind(parent context.Context) (context.Context, context.CancelFunc) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cancel != nil {
		j.cancel()
	}
	ctx, cancel := context.WithCancel(parent)
	j.cancel = cancel
	return ctx, cancel
}

func (j *jobSlot) clear(cancel context.CancelFunc) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cancel != nil {
		// only clear if still this job
		j.cancel = nil
	}
	cancel()
}

// graphExploreSession holds per-tab edge deltas + extract corpus (no re-read).
//
// Pipeline:
//
//	read → emit package focus (LookupNode, cheap) → inbox (latest wins) → worker
//	                                                         ↓
//	                                                      outbox → write → WS
//
// Package nodes from the file browser appear immediately even while the worker
// is still materializing another package. New visits preempt the job context so
// edge emission for the abandoned package stops after the current Materialize.
type graphExploreSession struct {
	root   string
	corpus *graphql.SessionCorpus
	mu     sync.Mutex
	seen   map[string]bool
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

func sendOut(ctx context.Context, outbox chan<- graphSessionOut, msg graphSessionOut) bool {
	select {
	case <-ctx.Done():
		return false
	case outbox <- msg:
		return true
	}
}

// trySendOut avoids stalling the read loop when the writer is mid-frame.
func trySendOut(ctx context.Context, outbox chan<- graphSessionOut, msg graphSessionOut) bool {
	select {
	case <-ctx.Done():
		return false
	case outbox <- msg:
		return true
	default:
		t := time.NewTimer(100 * time.Millisecond)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return false
		case outbox <- msg:
			return true
		case <-t.C:
			return false
		}
	}
}

// enqueueLatest keeps only the newest job (file-browser spam coalesces).
func enqueueLatest(ch chan sessionJob, job sessionJob) {
	for {
		select {
		case ch <- job:
			return
		case <-ch:
		}
	}
}

// handleGraphSession is a long-lived WebSocket explore session.
//
//	WS /api/graph/session
//	→ {"op":"visit","ref":"…"}  /  {"op":"project"}
//	← focus (immediate on visit) / edge* / done
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

	inbox := make(chan sessionJob, 1)
	outbox := make(chan graphSessionOut)
	sess := newGraphExploreSession(s.loader.RootDir)
	var jobs jobSlot

	var wg sync.WaitGroup

	// --- write loop ---
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

	// --- explore worker ---
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
				jobCtx, jcancel := jobs.bind(ctx)
				switch job.op {
				case "visit":
					ref := job.ref
					if ref == "" {
						ref = "path:./"
					}
					// jobCtx cancels mid-explore; session ctx carries "done" so the client unblocks.
					sess.runVisit(ctx, jobCtx, ref, outbox)
				case "project":
					sess.runProject(ctx, jobCtx, outbox)
				}
				jobs.clear(jcancel)
			}
		}
	}()

	if !sendOut(ctx, outbox, graphSessionOut{Type: "ready"}) {
		cancel()
		wg.Wait()
		return
	}

	// --- read loop ---
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		var in graphSessionIn
		if err := json.Unmarshal(data, &in); err != nil {
			_ = trySendOut(ctx, outbox, graphSessionOut{Type: "error", Message: "invalid json"})
			continue
		}

		switch in.Op {
		case "ping":
			_ = trySendOut(ctx, outbox, graphSessionOut{Type: "pong"})
		case "visit":
			ref := in.Ref
			if ref == "" {
				ref = "path:./"
			}
			// Immediate package/module node for file-browser navigation.
			sess.emitPackageFocus(ctx, ref, outbox)
			// Stop emitting edges for the package currently materializing.
			jobs.preempt()
			// Explore this package next (coalesce older pending visits).
			enqueueLatest(inbox, sessionJob{op: "visit", ref: ref})
		case "project":
			jobs.preempt()
			enqueueLatest(inbox, sessionJob{op: "project"})
		default:
			_ = trySendOut(ctx, outbox, graphSessionOut{Type: "error", Message: "unknown op: " + in.Op})
		}
	}

	jobs.preempt()
	cancel()
	wg.Wait()
}

// emitPackageFocus is cheap (no WalkExtracts / Materialize): paints the package
// on the client as soon as the file browser navigates there.
func (s *graphExploreSession) emitPackageFocus(ctx context.Context, ref string, outbox chan<- graphSessionOut) {
	n := graphql.LookupNode(s.root, ref)
	if n == nil {
		return
	}
	inc := true
	_ = trySendOut(ctx, outbox, graphSessionOut{
		Type:       "focus",
		Node:       n,
		Incomplete: &inc,
		VisitRef:   ref,
	})
}

// runVisit explores ref under jobCtx; done is always sent on sessCtx.
func (s *graphExploreSession) runVisit(sessCtx, jobCtx context.Context, ref string, outbox chan<- graphSessionOut) {
	emit := s.deltaEmitter(jobCtx, outbox)
	_ = s.corpus.StreamVisit(jobCtx, ref, emit)
	inc := true
	_ = sendOut(sessCtx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: ref})
}

func (s *graphExploreSession) runProject(sessCtx, jobCtx context.Context, outbox chan<- graphSessionOut) {
	emit := s.deltaEmitter(jobCtx, outbox)
	_ = s.corpus.StreamProject(jobCtx, emit)
	inc := true
	_ = sendOut(sessCtx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: "project"})
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
