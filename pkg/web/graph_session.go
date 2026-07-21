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
	Type       string               `json:"type"` // ready | focus | edge | edges | done | error | pong
	Node       *graphql.GraphNode   `json:"node,omitempty"`
	Edge       *graphql.GraphEdge   `json:"edge,omitempty"`  // legacy single
	Edges      []*graphql.GraphEdge `json:"edges,omitempty"` // batched (~1s flush)
	Incomplete *bool                `json:"incomplete,omitempty"`
	Message    string               `json:"message,omitempty"`
	VisitRef   string               `json:"visitRef,omitempty"`
}

// edgeWireBatcher accumulates edges and flushes on a timer (or size cap / Flush).
// Avoids one WebSocket frame per edge during crawl.
type edgeWireBatcher struct {
	ctx        context.Context
	outbox     chan<- graphSessionOut
	mu         sync.Mutex
	buf        []*graphql.GraphEdge
	timer      *time.Timer
	flushEvery time.Duration
	maxBuf     int
}

func newEdgeWireBatcher(ctx context.Context, outbox chan<- graphSessionOut) *edgeWireBatcher {
	return &edgeWireBatcher{
		ctx:        ctx,
		outbox:     outbox,
		flushEvery: time.Second,
		maxBuf:     64,
	}
}

func (b *edgeWireBatcher) Add(e *graphql.GraphEdge) bool {
	if e == nil {
		return true
	}
	if b.ctx.Err() != nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, e)
	if len(b.buf) >= b.maxBuf {
		return b.flushLocked()
	}
	if b.timer == nil {
		b.timer = time.AfterFunc(b.flushEvery, func() { b.Flush() })
	}
	return true
}

// Flush sends any buffered edges immediately (call before done).
func (b *edgeWireBatcher) Flush() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushLocked()
}

func (b *edgeWireBatcher) flushLocked() bool {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	if len(b.buf) == 0 {
		return true
	}
	edges := b.buf
	b.buf = nil
	inc := true
	// Unlock not held across send — we already hold lock; sendOut may block.
	// Copy done; release by sending under lock is OK (same as other sendOut paths).
	return sendOut(b.ctx, b.outbox, graphSessionOut{
		Type:       "edges",
		Edges:      edges,
		Incomplete: &inc,
	})
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
	// crawlDone: paths whose crawl batch was successfully materialized+emitted.
	// Re-walks skip these so visit/open does not rematerialize the whole tree.
	crawlDone map[string]bool

	crawlOn    atomic.Bool
	crawlPause atomic.Bool // visit: pump stops sending; current batch finishes
}

func newGraphExploreSession(root string, corpus *graphql.SessionCorpus) *graphExploreSession {
	if corpus == nil {
		corpus = graphql.NewSessionCorpus(root)
	}
	return &graphExploreSession{
		root:      root,
		corpus:    corpus,
		seen:      make(map[string]bool),
		crawlDone: make(map[string]bool),
	}
}

func (s *graphExploreSession) crawlAlreadyDone(key string) bool {
	if key == "" {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.crawlDone[key]
}

func (s *graphExploreSession) markCrawlDone(batch crawlBatch) {
	if len(batch) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range batch {
		if k != "" {
			s.crawlDone[k] = true
		}
	}
}

func (s *graphExploreSession) resetCrawlDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crawlDone = make(map[string]bool)
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
//	then visit runs. Pump pause-waits poll crawlPause (no full re-walk on visit).
//	Re-walks skip crawlDone paths so opening source does not rematerialize the tree.
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
	edgeBatch := newEdgeWireBatcher(ctx, outbox)
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
				key := extractKey(fe)
				if key == "" || sess.crawlAlreadyDone(key) {
					// Already materialized+emitted this session — skip.
					return true
				}
				stored := sess.corpus.Touch(fe)
				key = extractKey(stored)
				if key == "" || sess.crawlAlreadyDone(key) {
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

			// Walk finished. If a visit is still holding the pause, wait it out
			// (do not require a kick — visits no longer kick full re-walks).
			for sess.crawlPause.Load() && sess.crawlOn.Load() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(15 * time.Millisecond):
				}
			}

			// Full walk finished (or stopped). Flush edges then done once.
			if sess.crawlOn.Load() {
				_ = edgeBatch.Flush()
				inc := true
				_ = sendOut(ctx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: "project"})
			}

			// Idle until crawl is re-enabled (toggle off→on). Visits only pause/resume
			// mid-walk via crawlPause — they must not restart a full rematerialize.
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
		emit := sess.deltaEmitter(ctx, outbox, edgeBatch)

		for {
			// Prefer pending visits without blocking on crawl.
			select {
			case <-ctx.Done():
				_ = edgeBatch.Flush()
				return
			case ref := <-visitCh:
				sess.crawlPause.Store(true)
				// Do not drop the buffered batch — process it after the visit so
				// crawlDone stays accurate and edges are not lost.
				sess.handleVisitPriority(ctx, ref, outbox, edgeBatch)
				continue
			default:
			}

			select {
			case <-ctx.Done():
				_ = edgeBatch.Flush()
				return
			case ref := <-visitCh:
				sess.crawlPause.Store(true)
				sess.handleVisitPriority(ctx, ref, outbox, edgeBatch)
			case batch, ok := <-crawlCh:
				if !ok {
					_ = edgeBatch.Flush()
					return
				}
				// Finish this batch fully — preemption only stops the pump from
				// enqueueing the next batch (no mid-Materialize cancel).
				if !sess.corpus.EmitProjectBatch(ctx, batch, emit) {
					if ctx.Err() != nil {
						_ = edgeBatch.Flush()
						return
					}
					// emit cancelled mid-batch: do not mark done (retry next walk)
					continue
				}
				sess.markCrawlDone(batch)
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
				// Fresh enable: allow rematerialize of the tree (toggle off→on).
				if !was {
					sess.resetCrawlDone()
				}
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

func (s *graphExploreSession) handleVisitPriority(
	ctx context.Context,
	ref string,
	outbox chan<- graphSessionOut,
	edgeBatch *edgeWireBatcher,
) {
	// crawlPause already set by worker. Pump polls crawlPause every ~15ms while
	// mid-walk — do not kick a full re-walk here (that rematerialized the repo
	// on every go-to-def and pegged the CPU).
	emit := s.deltaEmitter(ctx, outbox, edgeBatch)
	_ = s.corpus.StreamVisit(ctx, ref, emit)
	_ = edgeBatch.Flush()
	inc := true
	_ = sendOut(ctx, outbox, graphSessionOut{Type: "done", Incomplete: &inc, VisitRef: ref})
	s.crawlPause.Store(false)
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

func (s *graphExploreSession) deltaEmitter(
	ctx context.Context,
	outbox chan<- graphSessionOut,
	edgeBatch *edgeWireBatcher,
) graphql.StreamEmitter {
	return func(ev graphql.StreamEvent) bool {
		if ctx.Err() != nil {
			return false
		}
		switch ev.Type {
		case "focus":
			// Flush edges so ordering stays focus-then-related edges when possible.
			if edgeBatch != nil {
				_ = edgeBatch.Flush()
			}
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
			if edgeBatch != nil {
				return edgeBatch.Add(ev.Edge)
			}
			return sendOut(ctx, outbox, graphSessionOut{Type: "edge", Edge: ev.Edge, Incomplete: ev.Incomplete})
		case "error":
			if edgeBatch != nil {
				_ = edgeBatch.Flush()
			}
			_ = sendOut(ctx, outbox, graphSessionOut{Type: "error", Message: ev.Message})
			return true
		case "done":
			return true
		default:
			return true
		}
	}
}
