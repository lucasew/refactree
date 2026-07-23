package ingest

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ImportNeed is one import the file should have after a structural edit.
// Interpretation is language-specific (Go: ImportPath is the quoted path).
type ImportNeed struct {
	ImportPath string
}

// ImportHygiene ensures missing imports after site rewrites (and similar).
// It is not package-path rewrite on move (see MoveDriver.RewriteImports).
type ImportHygiene interface {
	Language() string
	// NeedsFromRef maps a product ref (with or without leading @) to an import
	// need, or false if this language does not import for that ref.
	NeedsFromRef(ref string) (ImportNeed, bool)
	// EnsureImportEdits returns localized edits on content that add any missing
	// needs (typically only the import section). Empty means nothing to do.
	// Edits use content's byte offsets so they compose with later body edits
	// when ApplyEdits runs high offsets first.
	EnsureImportEdits(fileRel string, content []byte, needs []ImportNeed) []Edit
}

var (
	importHygieneMu sync.RWMutex
	importHygiene   = map[string]ImportHygiene{}
)

// RegisterImportHygiene registers hygiene by language name.
// Panics on empty name, nil, or duplicate language.
func RegisterImportHygiene(h ImportHygiene) {
	if h == nil {
		panic("ingest: RegisterImportHygiene with nil")
	}
	name := h.Language()
	if name == "" {
		panic("ingest: RegisterImportHygiene with empty language")
	}
	importHygieneMu.Lock()
	defer importHygieneMu.Unlock()
	if _, exists := importHygiene[name]; exists {
		panic(fmt.Sprintf("ingest: import hygiene %q already registered", name))
	}
	importHygiene[name] = h
}

// ImportHygieneForLanguage returns the registered hygiene for lang, if any.
func ImportHygieneForLanguage(lang string) (ImportHygiene, bool) {
	importHygieneMu.RLock()
	defer importHygieneMu.RUnlock()
	h, ok := importHygiene[lang]
	return h, ok
}

// ApplyEditsInMemory applies edits to content using descending start offsets
// (same order as ApplyEdits). Invalid spans are skipped.
func ApplyEditsInMemory(content []byte, edits []Edit) []byte {
	if len(edits) == 0 {
		return content
	}
	sorted := append([]Edit(nil), edits...)
	slices.SortFunc(sorted, func(a, b Edit) int {
		return cmp.Compare(b.StartByte, a.StartByte)
	})
	buf := append([]byte(nil), content...)
	for _, e := range sorted {
		if int(e.EndByte) > len(buf) || e.StartByte > e.EndByte {
			continue
		}
		buf = append(buf[:e.StartByte], append([]byte(e.NewText), buf[e.EndByte:]...)...)
	}
	return buf
}

// EditContentDiff builds a single edit that turns before into after by replacing
// the minimal mid-span that differs (common prefix/suffix). If equal, returns nil.
func EditContentDiff(file string, before, after []byte) []Edit {
	if string(before) == string(after) {
		return nil
	}
	i := 0
	for i < len(before) && i < len(after) && before[i] == after[i] {
		i++
	}
	ja, jb := len(before), len(after)
	for ja > i && jb > i && before[ja-1] == after[jb-1] {
		ja--
		jb--
	}
	return []Edit{{
		File:    strings.TrimPrefix(file, "./"),
		Span:    Span{StartByte: uint32(i), EndByte: uint32(ja)},
		NewText: string(after[i:jb]),
	}}
}
