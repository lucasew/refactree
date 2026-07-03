package ingest

import (
	"fmt"
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
	ingestDir := dir
	refPath := ""
	refIsDir := false

	if ref.Provider != "" && ref.Provider != "path" {
		scope, ok, err := resolveProviderScopeTarget(ref)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("listing not supported for provider %q", ref.Provider)
		}
		ingestDir = scope.Dir
		refPath = ""
		refIsDir = true
	} else {
		refPath, refIsDir = listScopeForRef(ingestDir, ref)
	}

	result, err := IngestWithRecursion(ingestDir, providerListIngestRecursive(ref, opts))
	if err != nil {
		return err
	}

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
		if !providerAllowListEntity(ref, entRef, entPath, language, opts) {
			continue
		}
		if !allowListedEntity(ent, language, SymbolListOptions{
			IncludeHidden: opts.IncludeHidden,
		}) {
			continue
		}

		out := SymbolInfo{
			Entity:    ent,
			Reference: entRef,
			Language:  language,
		}
		out.Reference = providerListOutputReference(ref, entRef)
		out.Entity.Reference = out.Reference.String()

		if !yield(out) {
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

func allowListedEntity(ent Entity, language string, opts SymbolListOptions) bool {
	if opts.IncludeHidden {
		return true
	}
	driver, ok := languageDriverForName(language)
	if !ok {
		return true
	}
	if filter, ok := driver.(ExportedFlagFilter); ok && filter.UseExportedFlag() {
		return ent.Exported
	}
	entRef := ParseReference(ent.Reference)
	return driver.AllowListSymbol(entRef.Symbol, opts)
}

func allowSymbolByLanguage(name, language string, opts SymbolListOptions) bool {
	driver, ok := languageDriverForName(language)
	if !ok {
		return true
	}
	return driver.AllowListSymbol(name, opts)
}
