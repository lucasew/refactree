package web

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

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
	Files      []NavItem // Files tab: dirs and files nearby
	Symbols    []SymItem // Symbols tab: defs + imports in current file
	ParentHref string
	Error      string // hard failure (no usable page body)
	Warning    string // soft failure (still show source when possible)
	FocusID    string // fragment target for symbol deep-links
	Provider   string // non-empty when viewing a non-path provider scope
	NonText    bool   // listed path is not text; do not render body
}

// NavItem is a link in the Files tab.
type NavItem struct {
	Name   string
	Href   string
	Active bool
	IsDir  bool
}

// SymItem is a link in the Symbols tab (definition or import).
type SymItem struct {
	Name  string
	Href  string
	Kind  string // "def" or "import"
	Title string // light hover: reference (+ signature later)
}

// IndexView lists entries under the browse root.
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
	// Prefer resolved root so symlink checks compare consistently.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
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

// LoadIndex builds the root index listing (all entries).
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
		if e.IsDir() {
			v.Items = append(v.Items, NavItem{
				Name:  name + "/",
				Href:  EncodeCodeURL("path:./" + name),
				IsDir: true,
			})
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
// Serve redirects via ingest.CanonicalizeReference before calling LoadFile when the
// requested ref is not already canonical.
func (l *Loader) LoadFile(refStr string) FileView {
	parsed := ingest.ParseReference(refStr)
	parsed = ingest.CanonicalizeReference(l.RootDir, parsed)
	refStr = parsed.String()

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
		return l.loadPathView(v, scopeRef)
	}
	return l.loadProviderView(v, scopeRef, focusRef, parsed)
}

func (l *Loader) loadPathView(v FileView, scopeRef ingest.Reference) FileView {
	rel := strings.TrimPrefix(scopeRef.Path, "./")
	if rel == "" || rel == "." {
		idx := l.LoadIndex()
		v.Files = idx.Items
		v.Title = "path:./"
		v.Reference = "path:./"
		return v
	}

	abs, err := l.resolveUnderRoot(rel)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	st, err := os.Stat(abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	if st.IsDir() {
		v.Files = l.listPathDir(abs, rel)
		v.ParentHref = l.pathParentHref(rel)
		return v
	}

	v.Files = l.listPathDir(filepath.Dir(abs), filepath.Dir(rel))
	markActive(&v.Files, filepath.Base(rel))
	v.ParentHref = l.pathParentHref(filepath.Dir(rel))

	source, err := os.ReadFile(abs)
	if err != nil {
		v.Error = err.Error()
		return v
	}
	if !isTextContent(source) {
		v.NonText = true
		return v
	}

	if lang, ok := ingest.LanguageForFile(abs); ok {
		v.Language = lang
	}

	// Inspection only: BFS from this file (peers + import targets), not whole-tree walk.
	result, err := ingest.IngestForFile(l.RootDir, abs)
	if err != nil {
		v.Warning = err.Error()
		v.Segments = []annotate.Segment{{Text: string(source)}}
		return v
	}

	v.Segments = annotate.Build(source, rel, result, func(r string) string {
		return EncodeCodeURLInRoot(l.RootDir, r)
	})
	v.Symbols = symbolsForFile(rel, result, EncodeCodeURLInRoot, l.RootDir)
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
		v.Warning = err.Error()
		// Still try to list children without symbols from ingest.
		v.Files = l.listProviderFiles(scopeRef, scope, nil)
		v.ParentHref = providerParentHref(scopeRef)
		return v
	}

	v.Files = l.listProviderFiles(scopeRef, scope, result)
	v.ParentHref = providerParentHref(scopeRef)

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
	if !isTextContent(source) {
		v.NonText = true
		return v
	}

	// Re-ingest from the concrete file so annotate gets peer/import neighbors.
	focused := result
	if f, err := ingest.IngestForFile(scope.Dir, abs); err == nil {
		focused = f
	} else if v.Warning == "" {
		v.Warning = err.Error()
	}

	if lang, ok := ingest.LanguageForFile(abs); ok {
		v.Language = lang
	}

	mapRef := providerRefMapper(scopeRef, focused)
	v.Segments = annotate.BuildWithOptions(source, fileRel, focused, EncodeCodeURL, annotate.Options{
		MapRef: mapRef,
	})
	v.Symbols = symbolsForFileMapped(fileRel, focused, EncodeCodeURL, mapRef)
	markActiveHref(&v.Files, EncodeCodeURL(focusRef))
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

// symbolsForFile builds a ctags-ish list of defs + imports for one file (source order).
func symbolsForFile(fileRel string, result *ingest.Result, codeURL func(root, ref string) string, rootDir string) []SymItem {
	return symbolsForFileMapped(fileRel, result, func(ref string) string {
		return codeURL(rootDir, ref)
	}, nil)
}

func symbolsForFileMapped(fileRel string, result *ingest.Result, codeURL func(ref string) string, mapRef func(string) string) []SymItem {
	if result == nil {
		return nil
	}
	if mapRef == nil {
		mapRef = func(ref string) string { return ref }
	}
	norm := normalizeRel(fileRel)

	type row struct {
		start uint32
		item  SymItem
	}
	var rows []row

	for _, ent := range result.Entities {
		r := ingest.ParseReference(ent.Reference)
		if normalizeRel(r.Path) != norm || r.Symbol == "" {
			continue
		}
		display := mapRef(ent.Reference)
		name := r.Symbol
		href := codeURL(display)
		rows = append(rows, row{
			start: ent.StartByte,
			item: SymItem{
				Name:  name,
				Href:  href,
				Kind:  "def",
				Title: display,
			},
		})
	}

	for _, alias := range result.Aliases {
		r := ingest.ParseReference(alias.Reference)
		if normalizeRel(r.Path) != norm {
			continue
		}
		display := mapRef(alias.Reference)
		name := r.Symbol
		if name == "" {
			// Import of whole module: use last path segment or raw ref tail.
			name = aliasLocalName(alias, display)
		}
		target := alias.Target
		if target != "" {
			target = mapRef(target)
		}
		href := ""
		title := display
		if target != "" {
			href = codeURL(target)
			title = target
		} else if display != "" {
			href = codeURL(display)
		}
		rows = append(rows, row{
			start: alias.StartByte,
			item: SymItem{
				Name:  name,
				Href:  href,
				Kind:  "import",
				Title: title,
			},
		})
	}

	slices.SortStableFunc(rows, func(a, b row) int {
		return cmp.Compare(a.start, b.start)
	})
	out := make([]SymItem, len(rows))
	for i := range rows {
		out[i] = rows[i].item
	}
	return out
}

func aliasLocalName(alias ingest.Alias, displayRef string) string {
	r := ingest.ParseReference(alias.Reference)
	if r.Symbol != "" {
		return r.Symbol
	}
	// path:./file with no symbol — binding name often is the last component of target.
	if alias.Target != "" {
		t := ingest.ParseReference(alias.Target)
		base := filepath.Base(strings.TrimSuffix(t.Path, "/"))
		if base != "" && base != "." {
			return base
		}
	}
	if displayRef != "" {
		return displayRef
	}
	return "import"
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
		items = append(items, NavItem{
			Name: name,
			Href: EncodeCodeURL("path:./" + childRel),
		})
	}
	sortNav(items)
	return items
}

func (l *Loader) listProviderFiles(scopeRef ingest.Reference, scope refpkg.ScopeTarget, result *ingest.Result) []NavItem {
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
					name := e.Name()
					if e.IsDir() {
						childPath := refpkg.JoinProviderPath(scopeRef.Path, name)
						items = append(items, NavItem{
							Name:  name + "/",
							Href:  EncodeCodeURL(ingest.Reference{Provider: scopeRef.Provider, Path: childPath}.String()),
							IsDir: true,
						})
						continue
					}
					// Files under provider dir: link as path under scope dir is not used;
					// list name only if we can form a provider child path.
					childPath := refpkg.JoinProviderPath(scopeRef.Path, name)
					items = append(items, NavItem{
						Name: name,
						Href: EncodeCodeURL(ingest.Reference{Provider: scopeRef.Provider, Path: childPath}.String()),
					})
				}
			}
		}
	}

	// Source files in this package (provider URLs via first symbol — no path: escape).
	if result != nil {
		seen := map[string]bool{}
		for _, it := range items {
			seen[it.Name] = true
		}
		for _, f := range result.Files {
			p := normalizeRel(f.Path)
			if p == "" {
				continue
			}
			base := filepath.Base(p)
			if seen[base] {
				continue
			}
			seen[base] = true
			href := ""
			if sym := firstEntitySymbolInFile(result, p); sym != "" {
				href = EncodeCodeURL(ingest.Reference{
					Provider: scopeRef.Provider,
					Path:     scopeRef.Path,
					Symbol:   sym,
				}.String())
			}
			items = append(items, NavItem{
				Name: base,
				Href: href,
			})
		}
	}

	sortNav(items)
	return items
}

func firstEntitySymbolInFile(result *ingest.Result, fileRel string) string {
	if result == nil {
		return ""
	}
	norm := normalizeRel(fileRel)
	var best string
	var bestStart uint32
	found := false
	for _, ent := range result.Entities {
		r := ingest.ParseReference(ent.Reference)
		if normalizeRel(r.Path) != norm || r.Symbol == "" {
			continue
		}
		if !found || ent.StartByte < bestStart {
			best = r.Symbol
			bestStart = ent.StartByte
			found = true
		}
	}
	return best
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

// resolveUnderRoot joins rel to the serve root and ensures the result stays under
// root (no .. escape). If the path exists, symlinks are resolved and re-checked.
func (l *Loader) resolveUnderRoot(rel string) (string, error) {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == "." {
		return l.RootDir, nil
	}
	if strings.HasPrefix(rel, "/") || filepath.IsAbs(filepath.FromSlash(rel)) {
		return "", fmt.Errorf("path escapes serve root: %s", rel)
	}
	// Reject ".." segments before join.
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path escapes serve root: %s", rel)
		}
	}

	root := filepath.Clean(l.RootDir)
	abs := filepath.Clean(filepath.Join(root, filepath.FromSlash(rel)))
	if err := pathWithinRoot(root, abs); err != nil {
		return "", err
	}

	// If present, resolve symlinks and require target still under root.
	if _, err := os.Lstat(abs); err == nil {
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			// Dangling or intermediate missing: keep cleaned path under root.
			return abs, nil
		}
		if err := pathWithinRoot(root, resolved); err != nil {
			return "", err
		}
		return resolved, nil
	}
	return abs, nil
}

func pathWithinRoot(root, abs string) error {
	root = filepath.Clean(root)
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return fmt.Errorf("path escapes serve root: %s", abs)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes serve root: %s", abs)
	}
	return nil
}

// isTextContent reports whether b should be rendered as text in the browser.
func isTextContent(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	sample := b
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	for _, c := range sample {
		if c == 0 {
			return false
		}
	}
	return utf8.Valid(sample)
}

func sortNav(items []NavItem) {
	slices.SortFunc(items, func(a, b NavItem) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
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
