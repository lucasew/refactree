package web

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	refpkg "github.com/lucasew/refactree/pkg/reference"
	"github.com/lucasew/refactree/pkg/web/annotate"
)

// FileView is the template model for a single source file page.
type FileView struct {
	Title      string
	Reference  string
	RootDir    string
	Language   string
	Segments   []annotate.Segment
	Siblings   []NavItem
	ParentHref string
	Error      string
	FocusID    string // fragment target for symbol deep-links
	Provider   string // non-empty when viewing a non-path provider scope
}

// NavItem is a link in the file/dir sidebar.
type NavItem struct {
	Name   string
	Href   string
	Active bool
	IsDir  bool
}

// IndexView lists files under the browse root.
type IndexView struct {
	Title   string
	RootDir string
	Items   []NavItem
	Error   string
}

// LoadOptions carries optional request context (query params).
type LoadOptions struct {
	// File picks a source file inside a provider package scope (basename or rel path).
	File string
}

// Loader resolves source + ingest data for HTTP handlers.
type Loader struct {
	RootDir string
}

// NewLoader resolves root to an absolute path.
func NewLoader(rootDir string) (*Loader, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", abs)
	}
	return &Loader{RootDir: abs}, nil
}

// LoadIndex builds the root index listing.
func (l *Loader) LoadIndex() IndexView {
	v := IndexView{
		Title:   "refactree",
		RootDir: l.RootDir,
	}
	entries, err := os.ReadDir(l.RootDir)
	if err != nil {
		v.Error = err.Error()
		return v
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			v.Items = append(v.Items, NavItem{
				Name:  name + "/",
				Href:  EncodeCodeURL("path:./" + name),
				IsDir: true,
			})
			continue
		}
		if _, ok := ingest.LanguageForFile(name); !ok {
			continue
		}
		v.Items = append(v.Items, NavItem{
			Name: name,
			Href: EncodeCodeURL("path:./" + name),
		})
	}
	sort.Slice(v.Items, func(i, j int) bool {
		if v.Items[i].IsDir != v.Items[j].IsDir {
			return v.Items[i].IsDir
		}
		return v.Items[i].Name < v.Items[j].Name
	})
	return v
}

// LoadFile builds a FileView for the given reference (file or symbol).
func (l *Loader) LoadFile(refStr string) FileView {
	return l.LoadFileWithOptions(refStr, LoadOptions{})
}

// LoadFileWithOptions is LoadFile plus optional provider file selection.
func (l *Loader) LoadFileWithOptions(refStr string, opts LoadOptions) FileView {
	focusRef := refStr
	scopeRef := FileReferenceForView(refStr)
	v := FileView{
		Title:     scopeRef.String(),
		Reference: scopeRef.String(),
		RootDir:   l.RootDir,
		Provider:  scopeRef.Provider,
	}
	if focusRef != scopeRef.String() {
		v.FocusID = annotate.AnchorID(focusRef)
	}

	if scopeRef.Provider == "" || scopeRef.Provider == "path" {
		v.Provider = ""
		return l.loadPathView(v, scopeRef, opts)
	}
	return l.loadProviderView(v, scopeRef, focusRef, opts)
}

func (l *Loader) loadPathView(v FileView, scopeRef ingest.Reference, opts LoadOptions) FileView {
	_ = opts
	rel := strings.TrimPrefix(scopeRef.Path, "./")
	if rel == "" || rel == "." {
		idx := l.LoadIndex()
		v.Siblings = idx.Items
		v.Title = "path:./"
		v.Reference = "path:./"
		return v
	}

	abs := filepath.Join(l.RootDir, filepath.FromSlash(rel))
	st, err := os.Stat(abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	if st.IsDir() {
		v.Siblings = l.listPathDir(abs, rel)
		v.ParentHref = l.pathParentHref(rel)
		return v
	}

	source, err := os.ReadFile(abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	if lang, ok := ingest.LanguageForFile(abs); ok {
		v.Language = lang
	}

	result, err := ingest.Ingest(l.RootDir)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	v.Segments = annotate.Build(source, rel, result, EncodeCodeURL)
	v.Siblings = l.listPathDir(filepath.Dir(abs), filepath.Dir(rel))
	markActive(&v.Siblings, filepath.Base(rel))
	v.ParentHref = l.pathParentHref(filepath.Dir(rel))
	return v
}

func (l *Loader) loadProviderView(v FileView, scopeRef ingest.Reference, focusRef string, opts LoadOptions) FileView {
	scope, ok, err := refpkg.ResolveScopeTarget(scopeRef)
	if err != nil {
		v.Error = err.Error()
		return v
	}
	if !ok {
		v.Error = fmt.Sprintf("provider %q does not support scope browsing for %q", scopeRef.Provider, scopeRef.Path)
		return v
	}

	v.RootDir = scope.Dir
	v.Siblings = l.listProviderScope(scopeRef, scope)
	v.ParentHref = providerParentHref(scopeRef)

	result, err := ingest.IngestWithRecursion(scope.Dir, false)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	fileRel := opts.File
	focusParsed := ingest.ParseReference(focusRef)
	if fileRel == "" && focusParsed.Symbol != "" {
		fileRel = findEntityFile(result, focusParsed.Symbol)
	}

	if fileRel == "" {
		// Package/module overview: rail only, no source pane.
		return v
	}

	fileRel = strings.TrimPrefix(filepath.ToSlash(fileRel), "./")
	abs := filepath.Join(scope.Dir, filepath.FromSlash(fileRel))
	source, err := os.ReadFile(abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	if lang, ok := ingest.LanguageForFile(abs); ok {
		v.Language = lang
	}

	mapRef := providerRefMapper(scopeRef, result)
	v.Segments = annotate.BuildWithOptions(source, fileRel, result, EncodeCodeURL, annotate.Options{
		MapRef: mapRef,
	})
	v.Title = scopeRef.String() + " · " + fileRel
	markActiveProviderFile(&v.Siblings, fileRel, scopeRef)
	return v
}

func providerRefMapper(scopeRef ingest.Reference, result *ingest.Result) func(string) string {
	// Paths inside this ingest dir that belong to the provider scope.
	local := map[string]bool{}
	if result != nil {
		for _, f := range result.Files {
			local[normalizeRel(f.Path)] = true
		}
	}
	return func(ref string) string {
		r := ingest.ParseReference(ref)
		if r.Provider != "" && r.Provider != "path" {
			return ref
		}
		// Only rewrite symbol refs so definitions/usages become go:pkg::Sym.
		// File-level / import-span refs without a symbol stay as ingest produced them
		// (avoids collapsing every import alias to the same go:pkg id).
		if r.Symbol == "" {
			return ref
		}
		p := normalizeRel(r.Path)
		if !local[p] {
			return ref
		}
		return ingest.Reference{Provider: scopeRef.Provider, Path: scopeRef.Path, Symbol: r.Symbol}.String()
	}
}

func findEntityFile(result *ingest.Result, symbol string) string {
	if result == nil || symbol == "" {
		return ""
	}
	for _, ent := range result.Entities {
		r := ingest.ParseReference(ent.Reference)
		if r.Symbol == symbol {
			return normalizeRel(r.Path)
		}
	}
	return ""
}

func (l *Loader) listPathDir(absDir, relDir string) []NavItem {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil
	}
	var items []NavItem
	prefix := strings.TrimPrefix(filepath.ToSlash(relDir), ".")
	prefix = strings.Trim(prefix, "/")
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		var childRel string
		if prefix == "" {
			childRel = name
		} else {
			childRel = prefix + "/" + name
		}
		if e.IsDir() {
			items = append(items, NavItem{
				Name:  name + "/",
				Href:  EncodeCodeURL("path:./" + childRel),
				IsDir: true,
			})
			continue
		}
		if _, ok := ingest.LanguageForFile(name); !ok {
			continue
		}
		items = append(items, NavItem{
			Name: name,
			Href: EncodeCodeURL("path:./" + childRel),
		})
	}
	sortNav(items)
	return items
}

func (l *Loader) listProviderScope(scopeRef ingest.Reference, scope refpkg.ScopeTarget) []NavItem {
	var items []NavItem

	parent := parentProviderPath(scopeRef.Path)
	if parent != scopeRef.Path {
		// parent link is handled via ParentHref; children only here
	}

	allowChildren := true
	if scope.CanDescend != nil {
		allowChildren = *scope.CanDescend
	}

	if allowChildren {
		if children, ok, err := refpkg.ResolveScopeChildren(scopeRef, false); err == nil && ok {
			for _, child := range children {
				name := filepath.Base(filepath.FromSlash(strings.Trim(child.Ref.Path, "/")))
				if name == "" || name == "." {
					name = child.Ref.Path
				}
				isDir := child.Kind != refpkg.ScopeChildFile
				if isDir {
					name += "/"
				}
				items = append(items, NavItem{
					Name:  name,
					Href:  EncodeCodeURL(child.Ref.String()),
					IsDir: isDir,
				})
			}
		} else if allowChildren {
			// Fallback: subdirs with sources on disk.
			entries, err := os.ReadDir(scope.Dir)
			if err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					name := e.Name()
					if strings.HasPrefix(name, ".") {
						continue
					}
					childPath := joinProviderPath(scopeRef.Path, name)
					items = append(items, NavItem{
						Name:  name + "/",
						Href:  EncodeCodeURL(ingest.Reference{Provider: scopeRef.Provider, Path: childPath}.String()),
						IsDir: true,
					})
				}
			}
		}
	}

	// Source files in this package/module scope.
	entries, err := os.ReadDir(scope.Dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if _, ok := ingest.LanguageForFile(name); !ok {
				continue
			}
			items = append(items, NavItem{
				Name: name,
				Href: EncodeProviderFileURL(scopeRef, name),
			})
		}
	}

	sortNav(items)
	return items
}

func (l *Loader) pathParentHref(rel string) string {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if rel == "" || rel == "." {
		return "/"
	}
	parent := filepath.ToSlash(filepath.Dir(rel))
	if parent == "." {
		return EncodeCodeURL("path:./")
	}
	return EncodeCodeURL("path:./" + parent)
}

func providerParentHref(scopeRef ingest.Reference) string {
	parent := parentProviderPath(scopeRef.Path)
	if parent == scopeRef.Path {
		return "/"
	}
	if parent == "" {
		return "/"
	}
	return EncodeCodeURL(ingest.Reference{Provider: scopeRef.Provider, Path: parent}.String())
}

func parentProviderPath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
	if parent == "." {
		return ""
	}
	return parent
}

func joinProviderPath(base, name string) string {
	base = strings.Trim(base, "/")
	name = strings.Trim(name, "/")
	if base == "" {
		return name
	}
	if name == "" {
		return base
	}
	return base + "/" + name
}

func sortNav(items []NavItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})
}

func markActive(items *[]NavItem, baseName string) {
	for i := range *items {
		if !(*items)[i].IsDir && (*items)[i].Name == baseName {
			(*items)[i].Active = true
		}
	}
}

func markActiveProviderFile(items *[]NavItem, fileRel string, scopeRef ingest.Reference) {
	base := filepath.Base(fileRel)
	want := EncodeProviderFileURL(scopeRef, base)
	for i := range *items {
		if (*items)[i].Href == want || (*items)[i].Name == base {
			(*items)[i].Active = true
		}
	}
}

func normalizeRel(p string) string {
	return strings.TrimPrefix(filepath.ToSlash(p), "./")
}
