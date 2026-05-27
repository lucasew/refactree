package ingest

import "strings"

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
}

// resolve takes per-file extracts and produces the final Result by
// mapping imports to references and resolving usages to relations.
func resolve(extracts []*fileExtract) *Result {
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
		for _, imp := range fe.imports {
			base := resolveImportPath(fe.language, imp.sourcePath, knownFiles, knownDirs)
			ri := resolvedImport{target: base}
			if imp.memberName != "" {
				ri.target = base + "::" + imp.memberName
				ri.memberName = imp.memberName
			}
			table[imp.localName] = ri

			res.Aliases = append(res.Aliases, Alias{
				Reference: FileRef("./" + fe.path),
				StartByte: imp.startByte,
				EndByte:   imp.endByte,
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

	// 1. Check import table.
	if ri, ok := imports[u.name]; ok {
		target = ri.target
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
			Reference: scopeRef,
			StartByte: u.startByte,
			EndByte:   u.endByte,
			Target:    target,
		})
	}
}

// --- import path resolution per language ---

func resolveImportPath(language, sourcePath string, knownFiles, knownDirs map[string]bool) string {
	switch language {
	case "go":
		return resolveGoImportPath(sourcePath, knownDirs)
	case "python":
		return resolvePythonImportPath(sourcePath, knownFiles)
	case "javascript":
		return resolveJSImportPath(sourcePath)
	}
	return sourcePath
}

func resolveGoImportPath(importPath string, knownDirs map[string]bool) string {
	last := lastPathComponent(importPath)
	if knownDirs[last] {
		return FileRef("./" + last)
	}
	return "go:" + last
}

func resolvePythonImportPath(moduleName string, knownFiles map[string]bool) string {
	if knownFiles[moduleName+".py"] {
		return FileRef("./" + moduleName + ".py")
	}
	if knownFiles[moduleName+"/__init__.py"] {
		return FileRef("./" + moduleName)
	}
	return "python:" + moduleName
}

func resolveJSImportPath(sourcePath string) string {
	if strings.HasPrefix(sourcePath, "./") || strings.HasPrefix(sourcePath, "../") {
		return FileRef(sourcePath)
	}
	if strings.HasPrefix(sourcePath, "node:") {
		return sourcePath // already in provider:path form
	}
	return "node:" + sourcePath
}
