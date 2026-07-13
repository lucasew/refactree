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
// Listing is incremental: each source file is parsed (or served from the
// extract cache) and its entities are yielded before the next file is
// processed. It does not wait for a full-module Ingest/resolve graph.
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

	// Single-file scope: parse that file only.
	if !refIsDir && refPath != "" {
		abs := ref.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(ingestDir, filepath.FromSlash(refPath))
		}
		return yieldSymbolsFromFile(ingestDir, abs, ref, refPath, refIsDir, opts, yield)
	}

	rootAbs, err := filepath.Abs(ingestDir)
	if err != nil {
		rootAbs = ingestDir
	}

	// Directory / module scope: walk and yield as each file is parsed.
	// Skip full Ingest (no import-target expansion, no resolve graph).
	recursive := providerListIngestRecursive(ref, opts)
	return filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != rootAbs && isSkippedDirName(d.Name()) {
				return filepath.SkipDir
			}
			if !recursive && path != rootAbs {
				return filepath.SkipDir
			}
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		stop, yerr := yieldSymbolsFromFileInfo(rootAbs, path, info, ref, refPath, refIsDir, recursive, opts, yield)
		if yerr != nil {
			return yerr
		}
		if stop {
			return filepath.SkipAll
		}
		return nil
	})
}

// yieldSymbolsFromFile parses one file and yields matching symbols.
func yieldSymbolsFromFile(rootAbs, absPath string, ref Reference, refPath string, refIsDir bool, opts ListOptions, yield func(SymbolInfo) bool) error {
	recursive := providerListIngestRecursive(ref, opts)
	_, err := yieldSymbolsFromFileInfo(rootAbs, absPath, nil, ref, refPath, refIsDir, recursive, opts, yield)
	return err
}

func yieldSymbolsFromFileInfo(rootAbs, absPath string, info os.FileInfo, ref Reference, refPath string, refIsDir, recursive bool, opts ListOptions, yield func(SymbolInfo) bool) (stop bool, err error) {
	fe, err := parseFileCached(rootAbs, absPath, info)
	if err != nil {
		return false, err
	}
	if fe == nil {
		return false, nil
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
			return true, nil
		}
	}
	return false, nil
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
