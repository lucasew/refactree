package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
	gql "github.com/lucasew/refactree/pkg/web/graphql"
)

// GraphStore adapts Loader to the GraphQL Store interface.
// Corpus is optional shared session extract cache (same as graph WS explore).
type GraphStore struct {
	Loader *Loader
	Corpus *gql.SessionCorpus
}

func (s *GraphStore) RootDir() string {
	if s == nil || s.Loader == nil {
		return ""
	}
	return s.Loader.RootDir
}

func (s *GraphStore) Filesystem(ref *string) ([]*gql.FsEntry, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("loader not configured")
	}
	refStr := "path:./"
	if ref != nil && strings.TrimSpace(*ref) != "" {
		refStr = strings.TrimSpace(*ref)
	}
	parsed := ingest.CanonicalizeReference(s.Loader.RootDir, ingest.ParseReference(refStr))
	if parsed.Provider == "" || parsed.Provider == "path" {
		return s.listPathFS(parsed)
	}
	v := s.Loader.LoadFile(parsed.String())
	if v.Error != "" {
		return nil, fmt.Errorf("%s", v.Error)
	}
	out := make([]*gql.FsEntry, 0, len(v.Files))
	for _, it := range v.Files {
		id := fsEntryRef(it.Href, it.Name, "")
		out = append(out, &gql.FsEntry{Name: it.Name, Reference: id, IsDir: it.IsDir})
	}
	return out, nil
}

func (s *GraphStore) listPathFS(parsed ingest.Reference) ([]*gql.FsEntry, error) {
	rel := strings.TrimPrefix(parsed.Path, "./")
	dir := s.Loader.RootDir
	if rel != "" && rel != "." {
		dir = filepath.Join(s.Loader.RootDir, filepath.FromSlash(rel))
	}
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		dir = filepath.Dir(dir)
		rel = filepath.ToSlash(filepath.Dir(rel))
		if rel == "." {
			rel = ""
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]*gql.FsEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		childRel := name
		if rel != "" && rel != "." {
			childRel = rel + "/" + name
		}
		ref := "path:./" + childRel
		if e.IsDir() {
			out = append(out, &gql.FsEntry{Name: name + "/", Reference: ref, IsDir: true})
		} else {
			out = append(out, &gql.FsEntry{Name: name, Reference: ref, IsDir: false})
		}
	}
	return out, nil
}

func (s *GraphStore) Neighborhood(ref string) (*gql.Neighborhood, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("loader not configured")
	}
	return gql.BuildNeighborhood(s.Loader.RootDir, ref)
}

func (s *GraphStore) ProjectGraph() (*gql.Neighborhood, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("loader not configured")
	}
	return gql.BuildProjectGraph(s.Loader.RootDir)
}

func (s *GraphStore) Code(ref string) (*gql.CodeDocument, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("loader not configured")
	}
	v := s.Loader.LoadFile(ref)
	// Feed the shared explore corpus so graph visit/crawl reuse the same extracts
	// the code pane just paid to parse (file browser ↔ graph cache).
	if s.Corpus != nil && strings.TrimSpace(ref) != "" {
		_ = s.Corpus.PrimeVisit(ref)
	}
	doc := &gql.CodeDocument{
		Reference: v.Reference,
		NonText:   v.NonText,
	}
	if v.Language != "" {
		doc.Language = &v.Language
	}
	if v.Error != "" {
		doc.Error = &v.Error
	}
	if v.Warning != "" {
		doc.Warning = &v.Warning
	}
	if v.FocusID != "" {
		doc.FocusID = &v.FocusID
	}
	if v.ParentHref != "" {
		doc.ParentHref = &v.ParentHref
	}
	for _, seg := range v.Segments {
		cs := &gql.CodeSegment{Text: seg.Text, IsLink: seg.IsLink, IsDef: seg.IsDef}
		if seg.Href != "" {
			cs.Href = &seg.Href
		}
		if seg.ID != "" {
			cs.AnchorID = &seg.ID
		}
		if seg.Reference != "" {
			r := seg.Reference
			cs.Reference = &r
		}
		doc.Segments = append(doc.Segments, cs)
	}
	for _, f := range v.Files {
		id := fsEntryRef(f.Href, f.Name, "")
		doc.Files = append(doc.Files, &gql.FsEntry{Name: f.Name, Reference: id, IsDir: f.IsDir})
	}
	for _, sym := range v.Symbols {
		id := stripCodeURL(sym.Href)
		if id == "" && sym.Name != "" {
			// Fallback: symbol on the current file/module ref.
			base := scopeRefString(v.Reference)
			id = canonicalFSRef(base + "::" + sym.Name)
		}
		if id == "" {
			id = sym.Name
		}
		doc.Symbols = append(doc.Symbols, &gql.FsEntry{Name: sym.Name, Reference: id, IsDir: false})
	}
	return doc, nil
}

// scopeRefString strips ::symbol for building child symbol refs.
func scopeRefString(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "path:./"
	}
	if i := strings.Index(ref, "::"); i >= 0 {
		ref = ref[:i]
	}
	return canonicalFSRef(ref)
}

func (s *GraphStore) Doc(ref string) (*gql.Doc, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("loader not configured")
	}
	d, err := ingest.DocFor(s.Loader.RootDir, ref)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	out := &gql.Doc{Name: d.Name}
	if d.Signature != "" {
		out.Signature = &d.Signature
	}
	if d.DocString != "" {
		out.DocString = &d.DocString
	}
	return out, nil
}

func (s *GraphStore) Node(id string) (*gql.GraphNode, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("loader not configured")
	}
	n := gql.LookupNode(s.Loader.RootDir, id)
	if n == nil {
		return nil, fmt.Errorf("empty node id")
	}
	return n, nil
}

func (s *GraphStore) Nodes(ids []string) ([]*gql.GraphNode, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("loader not configured")
	}
	out := make([]*gql.GraphNode, 0, len(ids))
	for _, id := range ids {
		if n := gql.LookupNode(s.Loader.RootDir, id); n != nil {
			out = append(out, n)
		}
	}
	return out, nil
}

func stripCodeURL(href string) string {
	if href == "" {
		return ""
	}
	path := strings.Split(href, "#")[0]
	if strings.HasPrefix(path, CodePathPrefix) {
		ref, ok := DecodeCodePath(path)
		if ok {
			return canonicalFSRef(ref)
		}
	}
	return canonicalFSRef(href)
}

// fsEntryRef builds a stable GraphQL reference for a file-rail entry.
// Always returns a full provider:path form for path entries (path:./cmd, not cmd).
func fsEntryRef(href, name, parentRef string) string {
	if id := stripCodeURL(href); id != "" {
		return id
	}
	name = strings.TrimSuffix(strings.TrimSpace(name), "/")
	if name == "" || name == "." {
		return "path:./"
	}
	// Prefer joining under parent path scope when name alone is not a full ref.
	if parentRef != "" {
		p := ingest.ParseReference(parentRef)
		if p.Provider == "" || p.Provider == "path" {
			base := strings.TrimPrefix(p.Path, "./")
			base = strings.Trim(base, "/")
			if base == "" || base == "." {
				return canonicalFSRef("path:./" + name)
			}
			return canonicalFSRef("path:./" + base + "/" + name)
		}
	}
	return canonicalFSRef("path:./" + name)
}

// canonicalFSRef ensures path-provider refs keep path:./ and never bare "cmd".
func canonicalFSRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "path:./"
	}
	// Accidental full URL path leaked through.
	if strings.HasPrefix(ref, CodePathPrefix) {
		if decoded, ok := DecodeCodePath(ref); ok {
			ref = decoded
		}
	}
	r := ingest.ParseReference(ref)
	if r.Provider == "" {
		// Bare token or path-like without provider → path.
		if r.Symbol != "" || strings.Contains(r.Path, "/") || strings.HasPrefix(r.Path, ".") || r.Path != "" {
			r.Provider = "path"
			if r.Path != "" && !strings.HasPrefix(r.Path, "./") && !strings.HasPrefix(r.Path, "../") && !strings.HasPrefix(r.Path, "/") {
				r.Path = "./" + strings.TrimPrefix(r.Path, "./")
			}
			if r.Path == "" {
				r.Path = "./"
			}
		}
	}
	if strings.EqualFold(r.Provider, "path") {
		r = refpkg.NormalizePathReference(r)
	}
	return r.String()
}
