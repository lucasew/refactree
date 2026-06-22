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
	Reference  string // full reference as requested (may include ::symbol)
	RootDir    string
	Language   string
	Segments   []annotate.Segment
	Siblings   []NavItem
	ParentHref string
	Error      string
	FocusID    string // fragment target for symbol deep-links
	Provider   string // non-empty when viewing a non-path provider scope
}

// NavItem is a link in the rail (scopes, files, or symbols — always full references).
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

// Loader resolves source + ingest data for HTTP handlers.
type Loader struct {
	RootDir  string
	resolver *ingest.Resolver // project-scoped; dispatches to go/python/nix/…
}

// NewLoader resolves root to an absolute path and builds a provider Resolver
// for that project tree (so go:workspaced resolves via go list in --dir).
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
	return &Loader{
		RootDir:  abs,
		resolver: ingest.NewResolver(abs),
	}, nil
}

func (l *Loader) refs() *ingest.Resolver {
	if l != nil && l.resolver != nil {
		return l.resolver
	}
	return ingest.NewResolver("")
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
	sortNav(v.Items)
	return v
}

// LoadFile builds a FileView for the given full reference (file, scope, or symbol).
// Symbol refs open the source file where that symbol is defined.
func (l *Loader) LoadFile(refStr string) FileView {
	parsed := ingest.ParseReference(refStr)
	if parsed.Provider == "" || parsed.Provider == "path" {
		parsed = ingest.CanonicalizePathReference(l.RootDir, parsed)
		refStr = parsed.String()
	}

	focusRef := refStr
	scopeRef := ScopeReferenceForView(refStr)

	v := FileView{
		Title:     focusRef,
		Reference: focusRef,
		RootDir:   l.RootDir,
		Provider:  scopeRef.Provider,
	}
	if parsed.Symbol != "" {
		v.FocusID = annotate.AnchorID(focusRef)
	}

	if scopeRef.Provider == "" || scopeRef.Provider == "path" {
		v.Provider = ""
		return l.loadPathView(v, scopeRef, parsed)
	}
	return l.loadProviderView(v, scopeRef, focusRef, parsed)
}

func (l *Loader) loadPathView(v FileView, scopeRef ingest.Reference, parsed ingest.Reference) FileView {
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
		// Directory scope: if a symbol was requested, it doesn't apply at dir level.
		v.Siblings = l.listPathDir(abs, rel)
		v.ParentHref = l.pathParentHref(rel)
		return v
	}

	// File view. Symbol (if any) is only for focus within this file.
	source, err := os.ReadFile(abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	if lang, ok := ingest.LanguageForFile(abs); ok {
		v.Language = lang
	}

	// Inspection only: BFS from this file (peers + import targets), not whole-tree walk.
	result, err := ingest.IngestForFile(l.RootDir, abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	v.Segments = annotate.Build(source, rel, result, func(r string) string {
		return EncodeCodeURLInRoot(l.RootDir, r)
	})
	v.Siblings = l.listPathDir(filepath.Dir(abs), filepath.Dir(rel))
	markActive(&v.Siblings, filepath.Base(rel))
	v.ParentHref = l.pathParentHref(filepath.Dir(rel))
	_ = parsed
	return v
}

func (l *Loader) loadProviderView(v FileView, scopeRef ingest.Reference, focusRef string, parsed ingest.Reference) FileView {
	scope, ok, err := l.refs().ResolveScopeTarget(scopeRef)
	if err != nil {
		v.Error = err.Error()
		return v
	}
	if !ok {
		v.Error = fmt.Sprintf("provider %q does not support scope browsing for %q", scopeRef.Provider, scopeRef.Path)
		return v
	}

	v.RootDir = scope.Dir

	result, err := ingest.IngestWithRecursion(scope.Dir, false)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	v.Siblings = l.listProviderScope(scopeRef, scope, result)
	v.ParentHref = providerParentHref(scopeRef)
	markActiveHref(&v.Siblings, EncodeCodeURL(focusRef))

	// Scope-only ref (no symbol): overview, no source pane.
	if parsed.Symbol == "" {
		return v
	}

	// Symbol ref: open the file that defines it.
	fileRel := findEntityFile(result, parsed.Symbol)
	if fileRel == "" {
		v.Error = fmt.Sprintf("symbol %q not found in %s", parsed.Symbol, scopeRef.String())
		return v
	}

	fileRel = strings.TrimPrefix(filepath.ToSlash(fileRel), "./")
	abs := filepath.Join(scope.Dir, filepath.FromSlash(fileRel))
	source, err := os.ReadFile(abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	// Re-ingest from the concrete file so annotate gets peer/import neighbors,
	// not only the non-recursive package dir listing pass above.
	if focused, err := ingest.IngestForFile(scope.Dir, abs); err == nil {
		result = focused
	}

	if lang, ok := ingest.LanguageForFile(abs); ok {
		v.Language = lang
	}

	mapRef := providerRefMapper(scopeRef, result)
	v.Segments = annotate.BuildWithOptions(source, fileRel, result, EncodeCodeURL, annotate.Options{
		MapRef: mapRef,
	})
	v.Title = focusRef
	v.Reference = focusRef
	return v
}

func providerRefMapper(scopeRef ingest.Reference, result *ingest.Result) func(string) string {
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

func (l *Loader) listProviderScope(scopeRef ingest.Reference, scope refpkg.ScopeTarget, result *ingest.Result) []NavItem {
	var items []NavItem

	allowChildren := true
	if scope.CanDescend != nil {
		allowChildren = *scope.CanDescend
	}

	if allowChildren {
		if children, ok, err := l.refs().ResolveScopeChildren(scopeRef, false); err == nil && ok {
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

	// Symbols in this scope — full provider:path::symbol references (no ?file=).
	if result != nil {
		seen := map[string]bool{}
		var syms []NavItem
		for _, ent := range result.Entities {
			r := ingest.ParseReference(ent.Reference)
			if r.Symbol == "" || seen[r.Symbol] {
				continue
			}
			seen[r.Symbol] = true
			full := ingest.Reference{Provider: scopeRef.Provider, Path: scopeRef.Path, Symbol: r.Symbol}.String()
			syms = append(syms, NavItem{
				Name: r.Symbol,
				Href: EncodeCodeURL(full),
			})
		}
		sort.Slice(syms, func(i, j int) bool { return syms[i].Name < syms[j].Name })
		items = append(items, syms...)
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
	if parent == scopeRef.Path || parent == "" {
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

func markActiveHref(items *[]NavItem, href string) {
	// Compare without fragment.
	base := href
	if i := strings.Index(base, "#"); i >= 0 {
		base = base[:i]
	}
	for i := range *items {
		h := (*items)[i].Href
		if j := strings.Index(h, "#"); j >= 0 {
			h = h[:j]
		}
		if h == base {
			(*items)[i].Active = true
		}
	}
}

func normalizeRel(p string) string {
	return strings.TrimPrefix(filepath.ToSlash(p), "./")
}
