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

		// Re-exports / barrels: record as Aliases (zero span) so CanonicalizeInResult
		// can follow hops using only the provider-agnostic Result graph.
		//
		// Named: Reference is this module's export name (path:./barrel.js::Search).
		// Star:  Reference is the module file; Target is the source module (no symbol).
		for _, re := range fe.Reexports {
			base := re.SourcePath
			if hasDriver {
				base = driver.ResolveImport(re.SourcePath, ctx)
			}
			// Normalize resolved path:./file or bare path into a path reference string.
			baseRef := ParseReference(base)
			if baseRef.Provider == "" {
				baseRef = ParseReference(FileRef(strings.TrimPrefix(base, "path:")))
			}
			if baseRef.Provider == "" {
				baseRef.Provider = "path"
			}
			if re.Star {
				res.Aliases = append(res.Aliases, Alias{
					Reference: FileRef("./" + fe.Path),
					Target:    FileRef(baseRef.Path),
				})
				continue
			}
			exportName := re.ExportName
			if exportName == "" {
				exportName = re.SourceName
			}
			sourceName := re.SourceName
			if sourceName == "" {
				sourceName = exportName
			}
			if exportName == "" {
				continue
			}
			target := FileRef(baseRef.Path)
			if sourceName != "" {
				// export { default as Search } → target …::default (or file::SourceName).
				target = SymbolRef(baseRef.Path, sourceName)
			}
			// Prefer the source-name token span so rename rewrites export { name } from
			// without inserting at [0:0]. Zero-span when the driver did not record it
			// (canonicalize-only hop).
			res.Aliases = append(res.Aliases, Alias{
				Reference: SymbolRef("./"+fe.Path, exportName),
				StartByte: re.SourceStartByte,
				EndByte:   re.SourceEndByte,
				Target:    target,
			})
		}

		// Primary/module export (driver-supplied DefaultExport): file scope → symbol
		// in the same file so CanonicalizeInResult can refine module-only refs.
		if fe.DefaultExport != "" {
			res.Aliases = append(res.Aliases, Alias{
				Reference: FileRef("./" + fe.Path),
				Target:    SymbolRef("./"+fe.Path, fe.DefaultExport),
			})
		}
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
				Exported:  ent.Exported,
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
				resolveQualifiedUsage(res, imports, scopeRef, u, allEntities, fe)
			} else {
				resolveDirectUsage(res, fe, u, imports, allEntities, scopeRef)
			}
		}
	}

	return res
}

// resolveQualifiedUsage handles pkg.Member access: emits two relations.
// Qualifier may be an import alias (cobra) or a same-package/local entity (Registry).
func resolveQualifiedUsage(res *Result, imports map[string]resolvedImport, scopeRef string, u UsageDef, allEntities map[string][]entityLoc, fe *FileExtract) {
	var baseTarget string
	var importMember string

	if ri, ok := imports[u.Qualifier]; ok {
		baseTarget = ri.Target
		importMember = ri.MemberName
		if ri.MemberName != "" {
			baseTarget = strings.TrimSuffix(baseTarget, "::"+ri.MemberName)
		}
	} else {
		// Qualifier is a local/package entity (var, type, func), not an import.
		baseTarget = resolveEntityName(fe, u.Qualifier, allEntities)
	}
	if baseTarget == "" {
		return
	}

	res.Relations = append(res.Relations, Relation{
		Reference: scopeRef,
		StartByte: u.QualStartByte,
		EndByte:   u.QualEndByte,
		Target:    baseTarget,
	})

	memberTarget := resolveQualifiedMemberTarget(baseTarget, importMember, u.Name, allEntities)

	res.Relations = append(res.Relations, Relation{
		Reference: scopeRef,
		StartByte: u.StartByte,
		EndByte:   u.EndByte,
		Target:    memberTarget,
	})
}

func resolveQualifiedMemberTarget(baseTarget, importMember, member string, allEntities map[string][]entityLoc) string {
	baseRef := ParseReference(baseTarget)
	memberTarget := baseTarget + "::" + member
	file := strings.TrimPrefix(baseRef.Path, "./")

	if baseRef.Symbol != "" {
		qualified := baseRef.Symbol + "." + member
		if baseRef.Provider == "path" {
			if target, ok := entityInFile(allEntities, qualified, file); ok {
				return target
			}
		}
		return memberTarget
	}

	if importMember != "" {
		qualified := importMember + "." + member
		if baseRef.Provider == "path" {
			if target, ok := entityInFile(allEntities, qualified, file); ok {
				return target
			}
			if target, ok := entityInFile(allEntities, member, file); ok {
				return target
			}
		}
		return memberTarget
	}

	if baseRef.Provider != "path" {
		return memberTarget
	}

	dirPrefix := file
	for _, loc := range allEntities[member] {
		if strings.HasPrefix(loc.File, dirPrefix+"/") || loc.File == dirPrefix {
			return SymbolRef("./"+loc.File, loc.Entity.Name)
		}
	}
	suffix := "." + member
	for name, locs := range allEntities {
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		for _, loc := range locs {
			if loc.File == dirPrefix || strings.HasPrefix(loc.File, dirPrefix+"/") {
				return SymbolRef("./"+loc.File, loc.Entity.Name)
			}
		}
	}
	return memberTarget
}

func entityInFile(allEntities map[string][]entityLoc, name, file string) (string, bool) {
	for _, loc := range allEntities[name] {
		if loc.File == file {
			return SymbolRef("./"+loc.File, loc.Entity.Name), true
		}
	}
	return "", false
}

// resolveEntityName finds a symbol reference for a bare name in package/file scope.
func resolveEntityName(fe *FileExtract, name string, allEntities map[string][]entityLoc) string {
	if fe != nil && (fe.Language == "go" || fe.Language == "java") && fe.Package != "" {
		for _, loc := range allEntities[name] {
			if loc.File != fe.Path && loc.Package == fe.Package && loc.Language == fe.Language {
				return SymbolRef("./"+loc.File, loc.Entity.Name)
			}
		}
	}
	for _, loc := range allEntities[name] {
		if fe == nil || loc.File == fe.Path {
			return SymbolRef("./"+loc.File, loc.Entity.Name)
		}
	}
	return ""
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

	// 2–3. Same-package / same-file entities (vars, funcs, types).
	if target == "" {
		target = resolveEntityName(fe, u.Name, allEntities)
	}
	// 4. Nested members referenced unqualified inside a type scope (Java fields).
	if target == "" && fe != nil && fe.Language == "java" && u.Scope != "" {
		typeName := u.Scope
		if i := strings.Index(typeName, "."); i >= 0 {
			typeName = typeName[:i]
		}
		qualified := typeName + "." + u.Name
		for _, loc := range allEntities[qualified] {
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
