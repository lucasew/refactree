package ingest

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ListOptions controls symbol listing behavior.
type ListOptions struct {
	IncludeHidden bool
	Recursive     bool
}

// SymbolInfo is one listed symbol with extracted metadata.
type SymbolInfo struct {
	Entity    Entity
	Reference Reference
	Language  string
}

// WalkSymbols iterates symbols in a reference scope, invoking yield for each
// matching symbol. Returning false from yield stops iteration early.
func WalkSymbols(dir, reference string, opts ListOptions, yield func(SymbolInfo) bool) error {
	ref := ParseReference(reference)

	result, err := Ingest(dir)
	if err != nil {
		return err
	}

	refPath, refIsDir := listScopeForRef(dir, ref)

	langOf := map[string]string{}
	for _, f := range result.Files {
		langOf[f.Path] = f.Language
	}

	for _, ent := range result.Entities {
		entRef := ParseReference(ent.Reference)

		if ref.Symbol != "" && entRef.Symbol != ref.Symbol {
			continue
		}

		entPath := strings.TrimPrefix(entRef.Path, "./")
		if !matchesListPathScope(entPath, refPath, refIsDir, opts.Recursive) {
			continue
		}

		language := langOf[entPath]
		if !opts.IncludeHidden && isHiddenSymbol(entRef.Symbol, language) {
			continue
		}

		if !yield(SymbolInfo{
			Entity:    ent,
			Reference: entRef,
			Language:  language,
		}) {
			break
		}
	}

	return nil
}

func listScopeForRef(dir string, ref Reference) (refPath string, refIsDir bool) {
	if ref.Provider != "path" {
		return "", false
	}

	refPath = strings.TrimPrefix(ref.Path, "./")
	if refPath == "." {
		refPath = ""
	}

	absPath := ref.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(dir, refPath)
	}
	if st, err := os.Stat(absPath); err == nil && st.IsDir() {
		refIsDir = true
	}
	return refPath, refIsDir
}

func matchesListPathScope(entPath, refPath string, refIsDir, recursive bool) bool {
	entPath = filepath.ToSlash(entPath)
	refPath = filepath.ToSlash(refPath)
	refPath = strings.TrimPrefix(refPath, "./")
	if refPath == "." {
		refPath = ""
	}

	if refIsDir {
		if recursive {
			if refPath == "" {
				return true
			}
			return entPath == refPath || strings.HasPrefix(entPath, refPath+"/")
		}
		parent := filepath.ToSlash(path.Dir(entPath))
		if parent == "." {
			parent = ""
		}
		return parent == refPath
	}

	if refPath == "" {
		return true
	}
	return entPath == refPath
}

func isHiddenSymbol(name, language string) bool {
	switch language {
	case "go":
		return len(name) > 0 && name[0] >= 'a' && name[0] <= 'z'
	case "python":
		return strings.HasPrefix(name, "_")
	}
	return false
}
