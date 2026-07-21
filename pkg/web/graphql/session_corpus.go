package graphql

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lucasew/refactree/pkg/ingest"
)

// SessionCorpus holds FileExtracts for one graph explore session.
// Each relative path is absorbed at most once; Materialize of the full set
// runs only when the set grows. Parse uses ingest.parseFileCached (mtime).
type SessionCorpus struct {
	root string

	mu     sync.Mutex
	byPath map[string]*ingest.FileExtract // key: ToSlash rel path, no "./" prefix
	result *ingest.Result
	dirty  bool
}

// NewSessionCorpus builds an empty corpus for root.
func NewSessionCorpus(root string) *SessionCorpus {
	return &SessionCorpus{
		root:   root,
		byPath: make(map[string]*ingest.FileExtract),
	}
}

func extractRelKey(fe *ingest.FileExtract) string {
	if fe == nil {
		return ""
	}
	return strings.TrimPrefix(filepath.ToSlash(fe.Path), "./")
}

// Absorb records fe if its path is new. Returns true when the corpus grew.
func (c *SessionCorpus) Absorb(fe *ingest.FileExtract) bool {
	if c == nil || fe == nil {
		return false
	}
	key := extractRelKey(fe)
	if key == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.byPath[key]; ok {
		return false
	}
	c.byPath[key] = fe
	c.dirty = true
	c.result = nil
	return true
}

// Has reports whether path (project-relative) is already in the corpus.
func (c *SessionCorpus) Has(rel string) bool {
	if c == nil {
		return false
	}
	key := strings.TrimPrefix(filepath.ToSlash(rel), "./")
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.byPath[key]
	return ok
}

// Len returns number of cached extracts.
func (c *SessionCorpus) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.byPath)
}

// AbsorbSeed runs Seed BFS from seedAbs. onNew is called only for newly
// absorbed extracts (return false to stop). Already-cached paths are skipped.
func (c *SessionCorpus) AbsorbSeed(seedAbs string, onNew func(*ingest.FileExtract) bool) error {
	if c == nil {
		return fmt.Errorf("nil corpus")
	}
	return ingest.WalkExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractSeed,
		Root:  c.root,
		Paths: []string{seedAbs},
	}, func(fe *ingest.FileExtract) bool {
		if !c.Absorb(fe) {
			return true // known path — do not re-parse into corpus
		}
		if onNew != nil {
			return onNew(fe)
		}
		return true
	})
}

// AbsorbDir walks the project (or subdir). onNew only for newly absorbed files.
func (c *SessionCorpus) AbsorbDir(dir string, recursive bool, onNew func(*ingest.FileExtract) bool) error {
	if c == nil {
		return fmt.Errorf("nil corpus")
	}
	src := ingest.ExtractSource{
		Kind:      ingest.ExtractDir,
		Root:      c.root,
		Recursive: recursive,
	}
	if dir != "" {
		src.Dir = dir
	}
	return ingest.WalkExtracts(src, func(fe *ingest.FileExtract) bool {
		if !c.Absorb(fe) {
			return true
		}
		if onNew != nil {
			return onNew(fe)
		}
		return true
	})
}

// Result returns Materialize over all absorbed extracts.
// Recomputes only when the corpus grew since the last call.
func (c *SessionCorpus) Result() *ingest.Result {
	if c == nil {
		return &ingest.Result{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty && c.result != nil {
		return c.result
	}
	extracts := make([]*ingest.FileExtract, 0, len(c.byPath))
	for _, fe := range c.byPath {
		extracts = append(extracts, fe)
	}
	c.result = ingest.Materialize(c.root, extracts, ingest.MaterializeOptions{ExpandImports: false})
	c.dirty = false
	return c.result
}

// MaterializeOne resolves a single extract (progressive local edges; no corpus dirty change).
func (c *SessionCorpus) MaterializeOne(fe *ingest.FileExtract) *ingest.Result {
	if c == nil || fe == nil {
		return &ingest.Result{}
	}
	return ingest.Materialize(c.root, []*ingest.FileExtract{fe}, ingest.MaterializeOptions{ExpandImports: false})
}

// SnapshotExtracts returns a copy of the extract slice (for tests).
func (c *SessionCorpus) SnapshotExtracts() []*ingest.FileExtract {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*ingest.FileExtract, 0, len(c.byPath))
	for _, fe := range c.byPath {
		out = append(out, fe)
	}
	return out
}
