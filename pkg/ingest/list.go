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
//
// Thin convenience over WalkExtracts (no Materialize, no ExpandImports).
func WalkSymbols(dir, reference string, opts ListOptions, yield func(SymbolInfo) bool) error {
	ref := ParseReference(reference)
	ingestDir := dir
	refPath := ""
	refIsDir := false

	if ref.Provider != "" && ref.Provider != "path" {
		scope, ok, err := NewResolver("").ResolveScopeTarget(ref)
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

	recursive := providerListIngestRecursive(ref, opts)

	// Single-file scope: hop parse only that file.
	if !refIsDir && refPath != "" {
		abs := ref.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(ingestDir, filepath.FromSlash(refPath))
		}
		// Resolve once against the process cwd so relative fixture roots work.
		if a, err := filepath.Abs(abs); err == nil {
			abs = a
		}
		return WalkExtracts(ExtractSource{
			Kind:  ExtractHop,
			Root:  ingestDir,
			Paths: []string{abs},
		}, func(fe *FileExtract) bool {
			return yieldEntitiesFromExtract(fe, ref, refPath, refIsDir, recursive, opts, yield)
		})
	}

	// Directory scope: stream Dir extracts.
	return WalkExtracts(ExtractSource{
		Kind:      ExtractDir,
		Root:      ingestDir,
		Recursive: recursive,
	}, func(fe *FileExtract) bool {
		return yieldEntitiesFromExtract(fe, ref, refPath, refIsDir, recursive, opts, yield)
	})
}

// yieldEntitiesFromExtract returns false if the caller should stop WalkExtracts.
func yieldEntitiesFromExtract(fe *FileExtract, ref Reference, refPath string, refIsDir, recursive bool, opts ListOptions, yield func(SymbolInfo) bool) bool {
	if fe == nil {
		return true
	}
	for _, entDef := range fe.Entities {
		entPath := strings.TrimPrefix(filepath.ToSlash(fe.Path), "./")
		entRef := ParseReference(SymbolRef("./"+entPath, entDef.Name))

		if ref.Symbol != "" && entRef.Symbol != ref.Symbol {
			continue
		}
		if !matchesListPathScope(entPath, refPath, refIsDir, recursive) {
			continue
		}

		ent := Entity{
			Reference: entRef.String(),
			StartByte: entDef.StartByte,
			EndByte:   entDef.EndByte,
			Exported:  entDef.Exported,
		}
		language := fe.Language
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
			return false
		}
	}
	return true
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
