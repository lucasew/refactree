package ingest

import (
	"path/filepath"
	"strings"
)

// entityLoc pairs an EntityDef with its file and language metadata.
type entityLoc struct {
	File     string
	Entity   EntityDef
	Package  string
	Language string
}

// resolvedImport is the result of mapping a raw import to a reference.
type resolvedImport struct {
	Target     string // full reference string
	MemberName string // non-empty when a specific member was imported
	// True when the imported name is bound through explicit alias syntax.
	HasAliasBinding bool
}

// resolve takes per-file extracts and produces the final Result by
// mapping imports to references and resolving usages to relations.
func resolve(rootDir string, extracts []*FileExtract) *Result {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		rootAbs = rootDir
	}

	res := &Result{
		Files:     []File{},
		Entities:  []Entity{},
		Relations: []Relation{},
	}

	// Collect known files and directories for import resolution.
	knownFiles := map[string]bool{}
	knownDirs := map[string]bool{}
	for _, fe := range extracts {
		knownFiles[fe.Path] = true
		parts := strings.Split(fe.Path, "/")
		if len(parts) > 1 {
			knownDirs[parts[0]] = true
		}
	}

	// Index all entities by name for scope lookups.
	allEntities := map[string][]entityLoc{}
	for _, fe := range extracts {
		for _, ent := range fe.Entities {
			allEntities[ent.Name] = append(allEntities[ent.Name], entityLoc{
				File:     fe.Path,
				Entity:   ent,
				Package:  fe.Package,
				Language: fe.Language,
			})
		}
	}

	// Build import tables and emit aliases.
	importTables := map[string]map[string]resolvedImport{}

	for _, fe := range extracts {
		table := map[string]resolvedImport{}
		driver, hasDriver := languageDriverForName(fe.Language)
		ctx := ImportResolveContext{
			RootDir:      rootAbs,
			ImporterPath: fe.Path,
			KnownFiles:   knownFiles,
			KnownDirs:    knownDirs,
		}
		for _, imp := range fe.Imports {
			base := imp.SourcePath
			if hasDriver {
				base = driver.ResolveImport(imp.SourcePath, ctx)
			}
			ri := resolvedImport{
				Target:          base,
				HasAliasBinding: imp.HasAliasBinding,
			}
			if imp.MemberName != "" {
				ri.Target = base + "::" + imp.MemberName
				ri.MemberName = imp.MemberName
			}
			table[imp.LocalName] = ri

			startByte := imp.StartByte
			endByte := imp.EndByte
			if imp.TargetStartByte != 0 || imp.TargetEndByte != 0 {
				startByte = imp.TargetStartByte
				endByte = imp.TargetEndByte
			}

			res.Aliases = append(res.Aliases, Alias{
				Reference: FileRef("./" + fe.Path),
				StartByte: startByte,
				EndByte:   endByte,
				Target:    ri.Target,
			})
		}
		importTables[fe.Path] = table
	}

	// Emit files and entities.
	for _, fe := range extracts {
		res.Files = append(res.Files, File{
			Language: fe.Language,
			Path:     fe.Path,
		})
		for _, ent := range fe.Entities {
			res.Entities = append(res.Entities, Entity{
				Reference: SymbolRef("./"+fe.Path, ent.Name),
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
			})
		}
	}

	// Resolve usages to relations.
	for _, fe := range extracts {
		imports := importTables[fe.Path]
		for _, u := range fe.Usages {
			scopeRef := SymbolRef("./"+fe.Path, u.Scope)
			if u.Scope == "" {
				scopeRef = FileRef("./" + fe.Path)
			}

			if u.Qualifier != "" {
				resolveQualifiedUsage(res, imports, scopeRef, u, allEntities)
			} else {
				resolveDirectUsage(res, fe, u, imports, allEntities, scopeRef)
			}
		}
	}

	return res
}

// resolveQualifiedUsage handles pkg.Member access: emits two relations.
func resolveQualifiedUsage(res *Result, imports map[string]resolvedImport, scopeRef string, u UsageDef, allEntities map[string][]entityLoc) {
	ri, ok := imports[u.Qualifier]
	if !ok {
		return
	}
	baseTarget := ri.Target
	if ri.MemberName != "" {
		baseTarget = strings.TrimSuffix(baseTarget, "::"+ri.MemberName)
	}

	res.Relations = append(res.Relations, Relation{
		Reference: scopeRef,
		StartByte: u.QualStartByte,
		EndByte:   u.QualEndByte,
		Target:    baseTarget,
	})

	// For local directory targets (Path:./dir), resolve the member to
	// the specific file that defines it.
	memberTarget := baseTarget + "::" + u.Name
	baseRef := ParseReference(baseTarget)
	if baseRef.Provider == "path" && baseRef.Symbol == "" {
		dirPrefix := strings.TrimPrefix(baseRef.Path, "./")
		for _, loc := range allEntities[u.Name] {
			if strings.HasPrefix(loc.File, dirPrefix+"/") {
				memberTarget = SymbolRef("./"+loc.File, loc.Entity.Name)
				break
			}
		}
	}

	res.Relations = append(res.Relations, Relation{
		Reference: scopeRef,
		StartByte: u.StartByte,
		EndByte:   u.EndByte,
		Target:    memberTarget,
	})
}

// resolveDirectUsage handles bare identifier access.
func resolveDirectUsage(res *Result, fe *FileExtract, u UsageDef, imports map[string]resolvedImport, allEntities map[string][]entityLoc, scopeRef string) {
	target := ""
	viaImportAlias := false

	// 1. Check import table.
	if ri, ok := imports[u.Name]; ok {
		target = ri.Target
		viaImportAlias = ri.HasAliasBinding
	}

	// 2. For Go: check same-package entities in sibling files.
	if target == "" && fe.Language == "go" {
		for _, loc := range allEntities[u.Name] {
			if loc.File != fe.Path && loc.Package == fe.Package && loc.Language == "go" {
				target = SymbolRef("./"+loc.File, loc.Entity.Name)
				break
			}
		}
	}

	// 3. Check entities in the same file (self-references).
	if target == "" {
		for _, loc := range allEntities[u.Name] {
			if loc.File == fe.Path {
				target = SymbolRef("./"+loc.File, loc.Entity.Name)
				break
			}
		}
	}

	if target != "" {
		res.Relations = append(res.Relations, Relation{
			Reference:      scopeRef,
			StartByte:      u.StartByte,
			EndByte:        u.EndByte,
			Target:         target,
			ViaImportAlias: viaImportAlias,
		})
	}
}
