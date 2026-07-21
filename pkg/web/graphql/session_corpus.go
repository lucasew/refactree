package graphql

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lucasew/refactree/pkg/ingest"
)

// SessionCorpus is the core session store for graph exploration:
//
//   - FileExtracts keyed by relative path (each project file at most once)
//   - Parse goes through ingest.parseFileCached (mtime)
//   - Resolve is visit-scoped: Materialize only the extracts discovered for
//     that visit, not the entire session history
//
// This is the single source of truth for "what have we already read".
type SessionCorpus struct {
	root string

	mu     sync.Mutex
	byPath map[string]*ingest.FileExtract // key: ToSlash rel path, no "./" prefix
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

// Touch stores fe if new and returns the corpus-owned extract for that path.
func (c *SessionCorpus) Touch(fe *ingest.FileExtract) *ingest.FileExtract {
	if c == nil || fe == nil {
		return fe
	}
	key := extractRelKey(fe)
	if key == "" {
		return fe
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.byPath[key]; ok {
		return existing
	}
	c.byPath[key] = fe
	return fe
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

// GetByRel returns a cached extract by project-relative path, or nil.
func (c *SessionCorpus) GetByRel(rel string) *ingest.FileExtract {
	if c == nil {
		return nil
	}
	key := strings.TrimPrefix(filepath.ToSlash(rel), "./")
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.byPath[key]
}

// GetByAbs returns a cached extract for an absolute path under root, or nil.
func (c *SessionCorpus) GetByAbs(abs string) *ingest.FileExtract {
	if c == nil {
		return nil
	}
	rootAbs, err := filepath.Abs(c.root)
	if err != nil {
		rootAbs = c.root
	}
	abs, err = filepath.Abs(abs)
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return nil
	}
	return c.GetByRel(rel)
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

// PrimeVisit warms the corpus with the opened file/package extracts only
// (no import BFS). Used when the code browser opens a file so the graph
// session reuses the same parse cache. StreamVisit still does Seed BFS for
// neighborhood edges — PrimeVisit must stay cheap so go-to-def does not
// re-walk the import graph on every click.
func (c *SessionCorpus) PrimeVisit(ref string) error {
	if c == nil {
		return nil
	}
	parsed := ingest.CanonicalizeReference(c.root, ingest.ParseReference(ref))
	// Align with StreamVisit: normalize to graph module id first.
	focus := graphNodeForRef(c.root, parsed)
	parsed = ingest.ParseReference(focus.ID)
	visit := make(map[string]*ingest.FileExtract)
	return c.primeVisitHop(parsed, visit)
}

// primeVisitHop records direct package/file extracts without neighbor BFS.
func (c *SessionCorpus) primeVisitHop(parsed ingest.Reference, visit map[string]*ingest.FileExtract) error {
	if visit == nil {
		visit = make(map[string]*ingest.FileExtract)
	}
	if parsed.Provider != "" && parsed.Provider != "path" {
		scope, ok, err := ingest.NewResolver(c.root).ResolveScopeTarget(ingest.Reference{
			Provider: parsed.Provider,
			Path:     parsed.Path,
		})
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		files, err := directSourceFilesAbs(scope.Dir)
		if err != nil {
			return err
		}
		return c.DiscoverHop(files, visit)
	}

	abs, isDir, err := resolvePath(c.root, scopeRef(parsed))
	if err != nil {
		return err
	}
	var seeds []string
	if parsed.Symbol == "" {
		if isDir {
			seeds, err = directSourceFilesAbs(abs)
			if err != nil {
				return err
			}
		} else {
			seeds = []string{abs}
		}
	} else if isDir {
		seeds, err = directSourceFilesAbs(abs)
		if err != nil {
			return err
		}
	} else {
		seeds = []string{abs}
	}
	return c.DiscoverHop(seeds, visit)
}

// DiscoverHop parses only the given paths (no neighbor/import BFS).
func (c *SessionCorpus) DiscoverHop(seedAbs []string, visit map[string]*ingest.FileExtract) error {
	if c == nil {
		return fmt.Errorf("nil corpus")
	}
	if len(seedAbs) == 0 {
		return nil
	}
	if visit == nil {
		visit = make(map[string]*ingest.FileExtract)
	}
	return ingest.WalkExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractHop,
		Root:  c.root,
		Paths: seedAbs,
	}, func(fe *ingest.FileExtract) bool {
		if fe == nil {
			return true
		}
		stored := c.Touch(fe)
		visit[extractRelKey(stored)] = stored
		return true
	})
}

// DiscoverSeeds runs one Seed BFS from all seed paths at once.
// Every yielded extract is Touched into the corpus and recorded in visit
// (visit is the resolve universe for this operation).
func (c *SessionCorpus) DiscoverSeeds(seedAbs []string, visit map[string]*ingest.FileExtract) error {
	if c == nil {
		return fmt.Errorf("nil corpus")
	}
	if len(seedAbs) == 0 {
		return nil
	}
	if visit == nil {
		visit = make(map[string]*ingest.FileExtract)
	}
	return ingest.WalkExtracts(ingest.ExtractSource{
		Kind:  ingest.ExtractSeed,
		Root:  c.root,
		Paths: seedAbs,
	}, func(fe *ingest.FileExtract) bool {
		if fe == nil {
			return true
		}
		stored := c.Touch(fe)
		visit[extractRelKey(stored)] = stored
		return true
	})
}

// DiscoverDir walks a directory into the corpus and visit set.
// If onlyNew is true, paths already in the corpus are skipped (fast re-crawl after
// file-browser primes); still walks the tree to find new files.
func (c *SessionCorpus) DiscoverDir(dir string, recursive bool, visit map[string]*ingest.FileExtract) error {
	return c.discoverDir(dir, recursive, visit, false)
}

// DiscoverDirNew is DiscoverDir but only records extracts not already in the corpus.
// StreamProject uses this so crawl after opening files only materializes new paths.
func (c *SessionCorpus) DiscoverDirNew(dir string, recursive bool, visit map[string]*ingest.FileExtract) error {
	return c.discoverDir(dir, recursive, visit, true)
}

func (c *SessionCorpus) discoverDir(dir string, recursive bool, visit map[string]*ingest.FileExtract, onlyNew bool) error {
	if c == nil {
		return fmt.Errorf("nil corpus")
	}
	if visit == nil {
		visit = make(map[string]*ingest.FileExtract)
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
		if fe == nil {
			return true
		}
		key := extractRelKey(fe)
		if onlyNew && c.Has(key) {
			return true
		}
		stored := c.Touch(fe)
		visit[extractRelKey(stored)] = stored
		return true
	})
}

// MaterializeVisit resolves exactly the visit extract set (not the whole session).
func (c *SessionCorpus) MaterializeVisit(visit map[string]*ingest.FileExtract) *ingest.Result {
	if c == nil || len(visit) == 0 {
		return &ingest.Result{}
	}
	extracts := make([]*ingest.FileExtract, 0, len(visit))
	for _, fe := range visit {
		if fe != nil {
			extracts = append(extracts, fe)
		}
	}
	return ingest.Materialize(c.root, extracts, ingest.MaterializeOptions{ExpandImports: false})
}

// SnapshotExtracts returns a copy of all corpus extracts (tests).
func (c *SessionCorpus) SnapshotExtracts() []*ingest.FileExtract {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*ingest.FileExtract, 0, len(c.byPath))
	for _, fe := range c.byPath {
		out = append(out, fe)
	}
	return out
}

// AbsorbSeed is kept for tests: seed from one path into the corpus only.
func (c *SessionCorpus) AbsorbSeed(seedAbs string, onNew func(*ingest.FileExtract) bool) error {
	visit := make(map[string]*ingest.FileExtract)
	if err := c.DiscoverSeeds([]string{seedAbs}, visit); err != nil {
		return err
	}
	if onNew != nil {
		for _, fe := range visit {
			// onNew historically meant "newly absorbed"; call for all in visit for compat
			if !onNew(fe) {
				return nil
			}
		}
	}
	return nil
}
