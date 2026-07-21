package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	gql "github.com/lucasew/refactree/pkg/web/graphql"
)

// GraphStore adapts Loader to the GraphQL Store interface.
type GraphStore struct {
	Loader *Loader
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
		id := stripCodeURL(it.Href)
		if id == "" {
			id = it.Name
		}
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
		id := stripCodeURL(f.Href)
		doc.Files = append(doc.Files, &gql.FsEntry{Name: f.Name, Reference: id, IsDir: f.IsDir})
	}
	for _, sym := range v.Symbols {
		id := stripCodeURL(sym.Href)
		if id == "" {
			id = sym.Name
		}
		doc.Symbols = append(doc.Symbols, &gql.FsEntry{Name: sym.Name, Reference: id, IsDir: false})
	}
	return doc, nil
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
			return ref
		}
	}
	return href
}
