package web

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
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
	focusRef := refStr
	fileRef := FileReferenceForView(refStr)
	v := FileView{
		Title:     fileRef.String(),
		Reference: fileRef.String(),
		RootDir:   l.RootDir,
	}
	if focusRef != fileRef.String() {
		v.FocusID = annotate.AnchorID(focusRef)
	}

	if fileRef.Provider != "path" {
		v.Error = fmt.Sprintf("web browser currently supports path: references only, got %q", fileRef.Provider)
		return v
	}

	rel := strings.TrimPrefix(fileRef.Path, "./")
	if rel == "" || rel == "." {
		// Directory / index via /code/path:./
		v.Error = ""
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
		v.Title = fileRef.String()
		v.Siblings = l.listDir(abs, rel)
		v.ParentHref = l.parentHref(rel)
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

	// Ingest from project root so cross-file targets resolve.
	result, err := ingest.Ingest(l.RootDir)
	if err != nil {
		v.Error = err.Error()
		return v
	}

	v.Segments = annotate.Build(source, rel, result, EncodeCodeURL)
	v.Siblings = l.listDir(filepath.Dir(abs), filepath.Dir(rel))
	baseName := filepath.Base(rel)
	for i := range v.Siblings {
		if !v.Siblings[i].IsDir && v.Siblings[i].Name == baseName {
			v.Siblings[i].Active = true
		}
	}
	v.ParentHref = l.parentHref(filepath.Dir(rel))
	return v
}

func (l *Loader) listDir(absDir, relDir string) []NavItem {
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
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (l *Loader) parentHref(rel string) string {
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
