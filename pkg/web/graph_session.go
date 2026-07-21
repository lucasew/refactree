package web

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lucasew/refactree/pkg/ingest"
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
	Op      string `json:"op"`                // visit | project | crawl | ping
	Ref     string `json:"ref"`               // for visit
	Enabled *bool  `json:"enabled,omitempty"` // for crawl
}

type graphSessionOut struct {
	Type       string             `json:"type"` // ready | focus | edge | done | error | pong
	Node       *graphql.GraphNode `json:"node,omitempty"`
	Edge       *graphql.GraphEdge `json:"edge,omitempty"`
	Incomplete *bool              `json:"incomplete,omitempty"`
	Message    string             `json:"message,omitempty"`
	VisitRef   string             `json:"visitRef,omitempty"`
}

// crawlBatch is one unit of project crawl work for the crawl worker.
type crawlBatch map[string]*ingest.FileExtract

// crawlEnd marks the end of one full tree walk (so the client can get done).
type crawlEnd struct{}

type graphExploreSession struct {
	root   string
	corpus *graphql.SessionCorpus
	mu     sync.Mutex
	seen   map[string]bool // session-wide edge wire dedupe

	crawlOn    atomic.Bool
	crawlPause atomic.Bool // visit: pump stops sending; current batch finishes
}

func newGraphExploreSession(root string, corpus *graphql.SessionCorpus) *graphExploreSession {
	if corpus == nil {
		corpus = graphql.NewSessionCorpus(root)
	}
	return &graphExploreSession{
		root:   root,
		corpus: corpus,
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

func enqueueStr(ch chan string, ref string) {
	for {
		select {
		case ch <- ref:
			return
		case <-ch:
		}
	}
}

func extractKey(fe *ingest.FileExtract) string {
	if fe == nil {
		return ""
	}
	return strings.TrimPrefix(filepath.ToSlash(fe.Path), "./")
}

// handleGraphSession is a long-lived WebSocket explore session.
//
// Cooperative crawl preemption (go-to-def does not hard-cancel Materialize):
//
//	crawl pump  --batches-->  crawlCh  --worker--> outbox --> WS
//	visitCh (priority) pauses the pump (stops pumping); current batch finishes,
//	then visit runs, then pump resumes if crawl is still on.
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

	outbox := make(chan graphSessionOut)
	visitCh := make(chan string, 1)
	// Buffer 1: pump can leave one batch without blocking if the worker is on a visit.
	// Preemption = crawlPause (stop pumping more); current/buffered batch is drained or run.
	crawlCh := make(chan crawlBatch, 1)
	// Wake pump when crawl turns on or visit unpauses.
	crawlKick := make(chan struct{}, 1)

	sess := newGraphExploreSession(s.loader.RootDir, s.corpus)
	kick := func() {
		select {
		case crawlKick <- struct{}{}:
		default:
		}
	}

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

	// --- crawl pump: walks tree and pumps batches; pause = stop pumping ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// Idle until crawl enabled.
			for !sess.crawlOn.Load() {
				select {
				case <-ctx.Done():
					return
				case <-crawlKick:
				}
			}

			inc := true
			if !sendOut(ctx, outbox, graphSessionOut{
				Type:       "focus",
				Node:       graphql.ProjectFocusNode(sess.root),
				Incomplete: &inc,
				VisitRef:   "project",
			}) {
				return
			}

			batch := make(crawlBatch, graphql.ProjectBatchSize)
			pumpBatch := func() bool {
				if len(batch) == 0 {
					return true
				}
				// Do not pump while visit is holding the pause flag.
				for sess.crawlPause.Load() {
					if !sess.crawlOn.Load() {
						return false
					}
					select {
					case <-ctx.Done():
						return false
					case <-crawlKick:
					case <-time.After(15 * time.Millisecond):
					}
				}
				if !sess.crawlOn.Load() {
					return false
				}
				select {
				case <-ctx.Done():
					return false
				case crawlCh <- batch:
					batch = make(crawlBatch, graphql.ProjectBatchSize)
					return true
				}
			}

			err := ingest.WalkExtracts(ingest.ExtractSource{
				Kind:      ingest.ExtractDir,
				Root:      sess.root,
				Recursive: true,
			}, func(fe *ingest.FileExtract) bool {
				if fe == nil {
					return true
				}
				if !sess.crawlOn.Load() {
					return false
				}
				// Cooperative pause: stop pumping (do not send). Worker may still
				// finish the batch already in flight.
				for sess.crawlPause.Load() {
					if !sess.crawlOn.Load() {
						return false
					}
					select {
					case <-ctx.Done():
						return false
					case <-crawlKick:
					case <-time.After(15 * time.Millisecond):
					}
				}
				if err := ctx.Err(); err != nil {
					return false
				}
				stored := sess.corpus.Touch(fe)
				key := extractKey(stored)
				if key == "" {
					return true
				}
				batch[key] = stored
				if len(batch) < graphql.ProjectBatchSize {
					return true
				}
				return pumpBatch()
			})
			if err != nil && ctx.Err() == nil && sess.crawlOn.Load() {
				_ = sendOut(ctx, outbox, graphSessionOut{Type: "error", Message: err.Error(), VisitRef: "project"})
			}
			_ = pumpBatch()

			// Full walk finished (or stopped). Signal done if still crawling.
			if sess.crawlOn.Load() && !sess.crawlPause.Load() {
				inc := true
				_ = sendOut(ctx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: "project"})
			}

			// Wait for another kick (re-enable, or after visit) before next full walk.
			if !sess.crawlOn.Load() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case <-crawlKick:
			}
		}
	}()

	// --- worker: prefers visits; otherwise consumes crawl batches ---
	wg.Add(1)
	go func() {
		defer wg.Done()
		emit := sess.deltaEmitter(ctx, outbox)

		drainCrawl := func() {
			for {
				select {
				case <-crawlCh:
					// Drop buffered batch; pump will resend after unpause/kick.
				default:
					return
				}
			}
		}

		for {
			// Prefer pending visits without blocking on crawl.
			select {
			case <-ctx.Done():
				return
			case ref := <-visitCh:
				sess.crawlPause.Store(true)
				drainCrawl() // unblock pump if it was sending into a full buffer
				sess.handleVisitPriority(ctx, ref, outbox, kick)
				continue
			default:
			}

			select {
			case <-ctx.Done():
				return
			case ref := <-visitCh:
				sess.crawlPause.Store(true)
				drainCrawl()
				sess.handleVisitPriority(ctx, ref, outbox, kick)
			case batch, ok := <-crawlCh:
				if !ok {
					return
				}
				// Finish this batch fully — preemption only stops the pump from
				// enqueueing the next batch (no mid-Materialize cancel).
				if !sess.corpus.EmitProjectBatch(ctx, batch, emit) {
					if ctx.Err() != nil {
						return
					}
				}
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
			// Pause crawl pump (stops pumping); do not cancel in-flight batch Materialize.
			sess.crawlPause.Store(true)
			sess.emitPackageFocus(ctx, ref, outbox)
			enqueueStr(visitCh, ref)
		case "project":
			// One-shot: enable a single walk cycle via kick (crawlOn may stay false).
			// Treat as crawl kick with temporary on.
			if !sess.crawlOn.Load() {
				// run one walk: set on, kick, pump will done and wait; then leave on false?
				// Simpler: just set crawl on for one shot from client via crawl op.
				// Keep project as: kick a walk if crawl on, else enable briefly.
				sess.crawlOn.Store(true)
				kick()
			} else {
				kick()
			}
		case "crawl":
			en := in.Enabled != nil && *in.Enabled
			was := sess.crawlOn.Swap(en)
			if en {
				sess.crawlPause.Store(false)
				kick()
			} else {
				// Stop pumping; worker finishes current batch only.
				sess.crawlPause.Store(false)
				if was {
					kick() // wake pump out of walk wait loops so it exits walk
				}
			}
		default:
			_ = trySendOut(ctx, outbox, graphSessionOut{Type: "error", Message: "unknown op: " + in.Op})
		}
	}

	cancel()
	wg.Wait()
}

func (s *graphExploreSession) handleVisitPriority(ctx context.Context, ref string, outbox chan<- graphSessionOut, kick func()) {
	// crawlPause already set by worker before drain.
	emit := s.deltaEmitter(ctx, outbox)
	_ = s.corpus.StreamVisit(ctx, ref, emit)
	inc := true
	_ = sendOut(ctx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: ref})
	s.crawlPause.Store(false)
	if s.crawlOn.Load() {
		kick() // resume pumping
	}
}

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
