package ingest

import (
	"path/filepath"
	"strings"
)

// entityLoc pairs an entityDef with its file and language metadata.
type entityLoc struct {
	file     string
	entity   entityDef
	pkg      string
	language string
}

// resolvedImport is the result of mapping a raw import to a reference.
type resolvedImport struct {
	target     string // full reference string
	memberName string // non-empty when a specific member was imported
	// True when the imported name is bound through explicit alias syntax.
	hasAliasBinding bool
}

// resolve takes per-file extracts and produces the final Result by
// mapping imports to references and resolving usages to relations.
func resolve(rootDir string, extracts []*fileExtract) *Result {
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
		knownFiles[fe.path] = true
		parts := strings.Split(fe.path, "/")
		if len(parts) > 1 {
			knownDirs[parts[0]] = true
		}
	}

	// Index all entities by name for scope lookups.
	allEntities := map[string][]entityLoc{}
	for _, fe := range extracts {
		for _, ent := range fe.entities {
			allEntities[ent.name] = append(allEntities[ent.name], entityLoc{
				file:     fe.path,
				entity:   ent,
				pkg:      fe.pkg,
				language: fe.language,
			})
		}
	}

	// Build import tables and emit aliases.
	importTables := map[string]map[string]resolvedImport{}

	for _, fe := range extracts {
		table := map[string]resolvedImport{}
		driver, hasDriver := languageDriverForName(fe.language)
		ctx := ImportResolveContext{
			RootDir:      rootAbs,
			ImporterPath: fe.path,
			KnownFiles:   knownFiles,
			KnownDirs:    knownDirs,
		}
		for _, imp := range fe.imports {
			base := imp.sourcePath
			if hasDriver {
				base = driver.ResolveImport(imp.sourcePath, ctx)
			}
			ri := resolvedImport{
				target:          base,
				hasAliasBinding: imp.hasAliasBinding,
			}
			if imp.memberName != "" {
				ri.target = base + "::" + imp.memberName
				ri.memberName = imp.memberName
			}
			table[imp.localName] = ri

			startByte := imp.startByte
			endByte := imp.endByte
			if imp.targetStartByte != 0 || imp.targetEndByte != 0 {
				startByte = imp.targetStartByte
				endByte = imp.targetEndByte
			}

			res.Aliases = append(res.Aliases, Alias{
				Reference: FileRef("./" + fe.path),
				StartByte: startByte,
				EndByte:   endByte,
				Target:    ri.target,
			})
		}
		importTables[fe.path] = table
	}

	// Emit files and entities.
	for _, fe := range extracts {
		res.Files = append(res.Files, File{
			Language: fe.language,
			Path:     fe.path,
		})
		for _, ent := range fe.entities {
			res.Entities = append(res.Entities, Entity{
				Reference: SymbolRef("./"+fe.path, ent.name),
				StartByte: ent.startByte,
				EndByte:   ent.endByte,
			})
		}
	}

	// Resolve usages to relations.
	for _, fe := range extracts {
		imports := importTables[fe.path]
		for _, u := range fe.usages {
			scopeRef := SymbolRef("./"+fe.path, u.scope)
			if u.scope == "" {
				scopeRef = FileRef("./" + fe.path)
			}

			if u.qualifier != "" {
				resolveQualifiedUsage(res, imports, scopeRef, u, allEntities)
			} else {
				resolveDirectUsage(res, fe, u, imports, allEntities, scopeRef)
			}
		}
	}

	return res
}

// resolveQualifiedUsage handles pkg.Member access: emits two relations.
func resolveQualifiedUsage(res *Result, imports map[string]resolvedImport, scopeRef string, u usageDef, allEntities map[string][]entityLoc) {
	ri, ok := imports[u.qualifier]
	if !ok {
		return
	}
	baseTarget := ri.target
	if ri.memberName != "" {
		baseTarget = strings.TrimSuffix(baseTarget, "::"+ri.memberName)
	}

	res.Relations = append(res.Relations, Relation{
		Reference: scopeRef,
		StartByte: u.qualStartByte,
		EndByte:   u.qualEndByte,
		Target:    baseTarget,
	})

	// For local directory targets (path:./dir), resolve the member to
	// the specific file that defines it.
	memberTarget := baseTarget + "::" + u.name
	baseRef := ParseReference(baseTarget)
	if baseRef.Provider == "path" && baseRef.Symbol == "" {
		dirPrefix := strings.TrimPrefix(baseRef.Path, "./")
		for _, loc := range allEntities[u.name] {
			if strings.HasPrefix(loc.file, dirPrefix+"/") {
				memberTarget = SymbolRef("./"+loc.file, loc.entity.name)
				break
			}
		}
	}

	res.Relations = append(res.Relations, Relation{
		Reference: scopeRef,
		StartByte: u.startByte,
		EndByte:   u.endByte,
		Target:    memberTarget,
	})
}

// resolveDirectUsage handles bare identifier access.
func resolveDirectUsage(res *Result, fe *fileExtract, u usageDef, imports map[string]resolvedImport, allEntities map[string][]entityLoc, scopeRef string) {
	target := ""
	viaImportAlias := false

	// 1. Check import table.
	if ri, ok := imports[u.name]; ok {
		target = ri.target
		viaImportAlias = ri.hasAliasBinding
	}

	// 2. For Go: check same-package entities in sibling files.
	if target == "" && fe.language == "go" {
		for _, loc := range allEntities[u.name] {
			if loc.file != fe.path && loc.pkg == fe.pkg && loc.language == "go" {
				target = SymbolRef("./"+loc.file, loc.entity.name)
				break
			}
		}
	}

	// 3. Check entities in the same file (self-references).
	if target == "" {
		for _, loc := range allEntities[u.name] {
			if loc.file == fe.path {
				target = SymbolRef("./"+loc.file, loc.entity.name)
				break
			}
		}
	}

	if target != "" {
		res.Relations = append(res.Relations, Relation{
			Reference:      scopeRef,
			StartByte:      u.startByte,
			EndByte:        u.endByte,
			Target:         target,
			ViaImportAlias: viaImportAlias,
		})
	}
}
