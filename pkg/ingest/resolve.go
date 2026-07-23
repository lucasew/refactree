package ingest

import (
	"path"
	"path/filepath"
	"strings"
)

// entityLoc pairs an AtomDef with its file and language metadata.
type entityLoc struct {
	File     string
	Atom     AtomDef
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
		Files: []File{},
		Atoms: []Atom{},
		Uses:  []Use{},
	}

	// Collect known files and directories for import resolution.
	// knownDirs includes every parent directory of each file (not only the first
	// path segment) so local packages under nested trees are addressable.
	knownFiles := map[string]bool{}
	knownDirs := map[string]bool{}
	for _, fe := range extracts {
		p := strings.TrimPrefix(filepath.ToSlash(fe.Path), "./")
		knownFiles[p] = true
		dir := path.Dir(p)
		for dir != "." && dir != "/" && dir != "" {
			knownDirs[dir] = true
			parent := path.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
		// First segment remains for bare last-component ResolveImport matching.
		if i := strings.IndexByte(p, '/'); i > 0 {
			knownDirs[p[:i]] = true
		}
	}

	// Index all entities by name for scope lookups.
	allEntities := map[string][]entityLoc{}
	for _, fe := range extracts {
		for _, ent := range fe.Atoms {
			allEntities[ent.Name] = append(allEntities[ent.Name], entityLoc{
				File:     fe.Path,
				Atom:     ent,
				Package:  fe.Package,
				Language: fe.Language,
			})
		}
	}

	// Build import tables and emit aliases.
	importTables := map[string]map[string]resolvedImport{}
	// starModules maps importer file path → resolved module bases from
	// `from mod import *` so bare identifiers can bind to exported top-level names.
	starModules := map[string][]string{}

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
			// Star / wildcard member import: no local name token to rewrite; keep
			// module base for bare-name resolution in resolveDirectUsage.
			if imp.MemberName == "*" || imp.LocalName == "*" {
				starModules[fe.Path] = append(starModules[fe.Path], base)
				continue
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
				target = AtomRef(baseRef.Path, sourceName)
			}
			// Prefer the source-name token span so rename rewrites export { name } from
			// without inserting at [0:0]. Zero-span when the driver did not record it
			// (canonicalize-only hop).
			res.Aliases = append(res.Aliases, Alias{
				Reference: AtomRef("./"+fe.Path, exportName),
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
				Target:    AtomRef("./"+fe.Path, fe.DefaultExport),
			})
		}
	}

	// Emit files and entities.
	for _, fe := range extracts {
		res.Files = append(res.Files, File{
			Language: fe.Language,
			Path:     fe.Path,
		})
		for _, ent := range fe.Atoms {
			res.Atoms = append(res.Atoms, Atom{
				Reference: AtomRef("./"+fe.Path, ent.Name),
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
				Exported:  ent.Exported,
			})
		}
	}

	// Resolve usages to relations.
	for _, fe := range extracts {
		imports := importTables[fe.Path]
		stars := starModules[fe.Path]
		for _, u := range fe.Usages {
			scopeRef := AtomRef("./"+fe.Path, u.Scope)
			if u.Scope == "" {
				scopeRef = FileRef("./" + fe.Path)
			}

			if u.Qualifier != "" {
				resolveQualifiedUsage(res, imports, scopeRef, u, allEntities, fe)
			} else {
				resolveDirectUsage(res, fe, u, imports, stars, allEntities, scopeRef)
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
	// qualTarget is what the qualifier identifier renames with. For member imports
	// (from box import Box) that is the member entity, not the module path — so
	// renaming Box rewrites Box.VALUE qualifiers. Member lookup still uses the
	// module path in baseTarget (strip) + importMember.
	var qualTarget string

	if ri, ok := imports[u.Qualifier]; ok {
		fullTarget := ri.Target
		importMember = ri.MemberName
		baseTarget = fullTarget
		if ri.MemberName != "" {
			baseTarget = strings.TrimSuffix(fullTarget, "::"+ri.MemberName)
			qualTarget = fullTarget
		} else {
			qualTarget = fullTarget
		}
	} else {
		// Qualifier is a local/package entity (var, type, func), not an import.
		baseTarget = resolveEntityName(fe, u.Qualifier, allEntities)
		qualTarget = baseTarget
	}
	// Java enum constants / nested fields used as receivers (RED.ordinal()):
	// qualifier is not a top-level entity name — resolve Type.leaf via enclosing scope.
	if baseTarget == "" {
		baseTarget = resolveJavaNestedMember(fe, u.Scope, u.Qualifier, allEntities)
		qualTarget = baseTarget
	}
	if baseTarget == "" {
		return
	}
	if qualTarget == "" {
		qualTarget = baseTarget
	}

	res.Uses = append(res.Uses, Use{
		Reference: scopeRef,
		StartByte: u.QualStartByte,
		EndByte:   u.QualEndByte,
		Target:    qualTarget,
	})

	memberTarget := resolveQualifiedMemberTarget(baseTarget, importMember, u.Name, allEntities)

	res.Uses = append(res.Uses, Use{
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

	if baseRef.Name != "" {
		qualified := baseRef.Name + "." + member
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
			return AtomRef("./"+loc.File, loc.Atom.Name)
		}
	}
	suffix := "." + member
	for name, locs := range allEntities {
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		for _, loc := range locs {
			if loc.File == dirPrefix || strings.HasPrefix(loc.File, dirPrefix+"/") {
				return AtomRef("./"+loc.File, loc.Atom.Name)
			}
		}
	}
	return memberTarget
}

func entityInFile(allEntities map[string][]entityLoc, name, file string) (string, bool) {
	for _, loc := range allEntities[name] {
		if loc.File == file {
			return AtomRef("./"+loc.File, loc.Atom.Name), true
		}
	}
	return "", false
}

// resolveJavaNestedMember maps a bare leaf (field / enum constant) to Type.leaf
// using the enclosing Java type from scope (e.g. Color.defaultColor + RED → Color.RED).
func resolveJavaNestedMember(fe *FileExtract, scope, leaf string, allEntities map[string][]entityLoc) string {
	if fe == nil || fe.Language != "java" || scope == "" || leaf == "" {
		return ""
	}
	typeName := scope
	if i := strings.Index(typeName, "."); i >= 0 {
		typeName = typeName[:i]
	}
	qualified := typeName + "." + leaf
	for _, loc := range allEntities[qualified] {
		if loc.File == fe.Path {
			return AtomRef("./"+loc.File, loc.Atom.Name)
		}
	}
	return ""
}

// resolveEntityName finds a symbol reference for a bare name in package/file scope.
func resolveEntityName(fe *FileExtract, name string, allEntities map[string][]entityLoc) string {
	if fe != nil && (fe.Language == "go" || fe.Language == "java") {
		feDir := path.Dir(fe.Path)
		if feDir == "." {
			feDir = ""
		}
		for _, loc := range allEntities[name] {
			if loc.File == fe.Path || loc.Package != fe.Package || loc.Language != fe.Language {
				continue
			}
			// Same named package across files. Java's default package ("") is
			// directory-scoped: only peers in the same directory are visible.
			if fe.Package == "" {
				locDir := path.Dir(loc.File)
				if locDir == "." {
					locDir = ""
				}
				if locDir != feDir {
					continue
				}
			}
			return AtomRef("./"+loc.File, loc.Atom.Name)
		}
	}
	for _, loc := range allEntities[name] {
		if fe == nil || loc.File == fe.Path {
			return AtomRef("./"+loc.File, loc.Atom.Name)
		}
	}
	return ""
}

// resolveDirectUsage handles bare identifier access.
func resolveDirectUsage(res *Result, fe *FileExtract, u UsageDef, imports map[string]resolvedImport, starBases []string, allEntities map[string][]entityLoc, scopeRef string) {
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
	// 4. Nested members referenced unqualified inside a type scope (Java fields / enum constants).
	if target == "" {
		target = resolveJavaNestedMember(fe, u.Scope, u.Name, allEntities)
	}
	// 5. Class-scoped bare names when Scope.Name is an entity (e.g. Python
	// @helper.setter inside class Box → Box.helper). Method-body scopes are
	// Class.method; Class.method.leaf is not an entity so this stays fail-closed.
	if target == "" && u.Scope != "" && fe != nil {
		if t, ok := entityInFile(allEntities, u.Scope+"."+u.Name, fe.Path); ok {
			target = t
		}
	}
	// 6. Star imports (`from mod import *`): bind bare names to top-level entities
	// in the imported module (skip private `_` names; last star wins on collision).
	if target == "" && len(starBases) > 0 && u.Name != "" && !strings.HasPrefix(u.Name, "_") {
		for i := len(starBases) - 1; i >= 0; i-- {
			baseRef := ParseReference(starBases[i])
			file := strings.TrimPrefix(baseRef.Path, "./")
			if file == "" {
				continue
			}
			// Only flat top-level names (not Type.method) are introduced by import *.
			if t, ok := entityInFile(allEntities, u.Name, file); ok {
				target = t
				break
			}
		}
	}

	if target != "" {
		res.Uses = append(res.Uses, Use{
			Reference:      scopeRef,
			StartByte:      u.StartByte,
			EndByte:        u.EndByte,
			Target:         target,
			ViaImportAlias: viaImportAlias,
		})
	}
}
