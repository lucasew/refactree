package lsp

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/projectfs"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const debounceDelay = 300 * time.Millisecond

// Snapshot is a last-good project graph for intelligence.
type Snapshot struct {
	Root   string
	Result *ingest.Result
}

// Session holds overlays, open docs, and the last-good snapshot.
type Session struct {
	mu sync.RWMutex

	overlay  *projectfs.Overlay
	openDocs map[string]int32 // abs path -> version
	// fastExtract: abs path -> last successful FileExtract for document symbols
	fastExtract map[string]*ingest.FileExtract

	snap *Snapshot
	root string

	client protocol.Client

	dirty     bool
	timer     *time.Timer
	cancelBld context.CancelFunc
	gen       uint64
}

func newSession() *Session {
	return &Session{
		overlay:     projectfs.NewOverlay(nil),
		openDocs:    map[string]int32{},
		fastExtract: map[string]*ingest.FileExtract{},
		snap:        &Snapshot{},
	}
}

func (s *Session) setClient(c protocol.Client) {
	s.mu.Lock()
	s.client = c
	s.mu.Unlock()
}

func (s *Session) ensureRoot(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root != "" {
		return
	}
	dir := path
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		dir = filepath.Dir(path)
	}
	if root, ok := discoverRoot(dir); ok {
		s.root = root
	}
}

// discoverRoot: prefer existing path if it is a dir; else walk for .git.
func discoverRoot(start string) (string, bool) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	// If caller already passed a project dir, use it when it looks like a root.
	if hasGit(abs) {
		return abs, true
	}
	dir := abs
	for {
		if hasGit(dir) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// fallback: start directory itself
			return abs, true
		}
		dir = parent
	}
}

func hasGit(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && st.IsDir()
}

func (s *Session) markDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty = true
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(debounceDelay, func() {
		s.scheduleRebuild()
	})
}

func (s *Session) scheduleRebuild() {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return
	}
	s.dirty = false
	if s.cancelBld != nil {
		s.cancelBld()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelBld = cancel
	s.gen++
	gen := s.gen
	client := s.client
	s.mu.Unlock()
	go s.rebuild(ctx, gen, client)
}

func (s *Session) rebuild(ctx context.Context, gen uint64, client protocol.Client) {
	s.mu.RLock()
	root := s.root
	s.mu.RUnlock()
	if root == "" {
		s.mu.RLock()
		for p := range s.openDocs {
			s.mu.RUnlock()
			s.ensureRoot(p)
			s.mu.RLock()
			root = s.root
			break
		}
		s.mu.RUnlock()
	}
	if root == "" {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	// Prefer Seed from open docs when small; full Dir when we need workspace graph.
	// Dogfood: always full Dir for correct refs/completion; cancelable.
	result, err := ingest.ProjectResultFS(root, s.overlay)
	if err != nil {
		slog.Debug("lsp rebuild: project", "err", err)
		if client != nil {
			_ = client.LogMessage(ctx, &protocol.LogMessageParams{
				Type:    protocol.MessageTypeWarning,
				Message: "rft: " + err.Error(),
			})
		}
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	s.mu.Lock()
	if gen != s.gen {
		s.mu.Unlock()
		return
	}
	s.snap = &Snapshot{Root: root, Result: result}
	s.mu.Unlock()
}

func (s *Session) snapshot() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

func (s *Session) docText(path string) (string, bool) {
	return s.overlay.GetString(path)
}

func (s *Session) fastParse(path, text string) {
	root := s.root
	if root == "" {
		s.ensureRoot(path)
		s.mu.RLock()
		root = s.root
		s.mu.RUnlock()
	}
	if root == "" {
		return
	}
	// Temporarily set overlay (already set by caller) and hop-parse this file.
	extracts, err := ingest.CollectExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractHop,
		Root:  root,
		Paths: []string{path},
		FS:    s.overlay,
	})
	if err != nil || len(extracts) == 0 {
		return
	}
	s.mu.Lock()
	s.fastExtract[path] = extracts[0]
	s.mu.Unlock()
}

func pathToURI(path string) uri.URI {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return uri.File(abs)
}

func uriToPath(u uri.URI) string {
	s := string(u)
	if strings.HasPrefix(s, "file://") {
		p := strings.TrimPrefix(s, "file://")
		if strings.HasPrefix(p, "/") {
			return p
		}
		return p
	}
	return s
}

func (s *Session) relPath(abs string) string {
	s.mu.RLock()
	root := s.root
	if s.snap != nil && s.snap.Root != "" {
		root = s.snap.Root
	}
	s.mu.RUnlock()
	if root == "" {
		return filepath.Base(abs)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.Base(abs)
	}
	return filepath.ToSlash(rel)
}
