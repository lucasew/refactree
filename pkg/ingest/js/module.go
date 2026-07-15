package js

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/tsx"
	_ "github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar/typescript"
)

// ECMA family: language id remains "javascript" for the current surface bundle
// (JS/TS/TSX/JSX). Grammars differ by extension; extract/move/resolve are shared.
// Svelte is a separate surface id under FamilyECMA (see pkg/ingest/svelte).
// Prefer FamilyECMA for lattice sharing. Vue/Astro are out of scope.
// Future: split honest surface ids (typescript, tsx) under the same family.
var ecmaExtensions = []string{".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx"}

func init() {
	ingest.RegisterLanguageDriver("javascript", languageDriver{})
	ingest.RegisterLanguageRules("javascript", ingest.LanguageRules{
		Extensions:      append([]string(nil), ecmaExtensions...),
		DirectoryModule: false,
		Family:          ingest.FamilyECMA,
	})
	ingest.RegisterReferenceProvider("node", referenceProvider{})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "javascript" }

// TreeSitterGrammar selects the ECMA surface grammar for filename.
// .js/.mjs/.cjs → javascript; .ts → typescript; .tsx/.jsx → tsx.
func (languageDriver) TreeSitterGrammar(filename string) (grammar.Language, bool) {
	return grammarForECMAFile(filename)
}

func grammarForECMAFile(filename string) (grammar.Language, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".ts":
		return grammar.Get("typescript")
	case ".tsx", ".jsx":
		// tsx grammar covers JSX; no separate jsx grammar in the pin.
		return grammar.Get("tsx")
	case ".js", ".mjs", ".cjs":
		return grammar.Get("javascript")
	default:
		if lang, ok := grammar.GetByExtension(filename); ok {
			return lang, true
		}
		return grammar.Get("javascript")
	}
}

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	// Shared ECMA extract (JS + TS/TSX node shapes we care about).
	return extractECMA(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	if ref, ok := resolvePathImport(sourcePath, ctx); ok {
		return ref
	}
	if ref, ok := resolveNodeImport(sourcePath, ctx); ok {
		return ref
	}
	return nodeSymbolicRef(sourcePath)
}

func (languageDriver) AllowListSymbol(string, ingest.SymbolListOptions) bool { return true }

func (languageDriver) DestinationFileInDirectory(dstDirRel string, _ ingest.Reference) string {
	_ = dstDirRel
	return ""
}

// ResolveDirectoryModule maps a JS/Node package directory to package.json main
// or index.{js,mjs,cjs} (same convention as import resolution).
func (languageDriver) ResolveDirectoryModule(absDir string) (string, bool) {
	resolved, ok := resolveJSFileOnDisk(absDir, true)
	if !ok {
		return "", false
	}
	st, err := os.Stat(absDir)
	if err != nil || !st.IsDir() {
		return "", false
	}
	rel, err := filepath.Rel(absDir, resolved)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

type referenceProvider struct{}

func (referenceProvider) Name() string { return "node" }

func (referenceProvider) Resolve(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	return resolveNodeImport(spec, ctx)
}

func resolvePathImport(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if !(strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/")) {
		return "", false
	}

	rel := ingest.RelImportPath(ctx.ImporterPath, spec)
	if rel == "" {
		return "", false
	}

	if ref, ok := resolveKnownJSPath(rel, ctx.KnownFiles); ok {
		return ref, true
	}

	rootAbs, err := filepath.Abs(ctx.RootDir)
	if err != nil {
		return ingest.FileRef("./" + rel), true
	}
	candidate := filepath.Join(rootAbs, filepath.FromSlash(rel))
	resolved, ok := resolveJSFileOnDisk(candidate, false)
	if ok {
		return ingest.PathRefForAbs(rootAbs, resolved), true
	}

	return ingest.FileRef("./" + rel), true
}

// nodeSymbolicRef builds a node-provider reference for an unresolved import.
// Always prefixes with the provider ("node:"). Specifiers that already use the
// Node builtin protocol (e.g. "node:url") therefore become "node:node:url"
// (provider=node, path=node:url), not "node:url" (provider=node, path=url).
func nodeSymbolicRef(spec string) string {
	return "node:" + spec
}

func resolveNodeImport(spec string, ctx ingest.ImportResolveContext) (string, bool) {
	if strings.HasPrefix(spec, "node:") {
		// Node builtin / explicit protocol — keep full "node:..." as the path.
		return nodeSymbolicRef(spec), true
	}
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || strings.HasPrefix(spec, "/") {
		return "", false
	}

	pkgName, subpath := splitNodePackageSpecifier(spec)
	if pkgName == "" {
		return nodeSymbolicRef(spec), true
	}

	rootAbs, err := filepath.Abs(ctx.RootDir)
	if err != nil {
		rootAbs = ctx.RootDir
	}
	importerAbs := filepath.Join(rootAbs, filepath.FromSlash(filepath.Dir(ctx.ImporterPath)))

	for _, pkgRoot := range nodeModuleCandidates(importerAbs, pkgName) {
		if st, err := os.Stat(pkgRoot); err != nil || !st.IsDir() {
			continue
		}

		// Prefer package.json entrypoints (exports / module / main) — same order Node uses.
		if resolved, ok := resolvePackageEntrypoint(pkgRoot, subpath); ok {
			return ingest.PathRefForAbs(rootAbs, resolved), true
		}

		// Fallback when there is no package.json (or no usable entry): treat as a plain path.
		targetBase := pkgRoot
		if subpath != "" {
			targetBase = filepath.Join(pkgRoot, filepath.FromSlash(subpath))
		}
		if resolved, ok := resolveJSFileOnDisk(targetBase, false); ok {
			return ingest.PathRefForAbs(rootAbs, resolved), true
		}
		if st, err := os.Stat(targetBase); err == nil && st.IsDir() {
			return ingest.PathRefForAbs(rootAbs, targetBase), true
		}
	}

	return nodeSymbolicRef(spec), true
}

func extractECMA(root *grammar.Node, source []byte, path string) *ingest.FileExtract {
	fe := &ingest.FileExtract{Language: "javascript", Path: path}

	for i := uint32(0); i < root.ChildCount(); i++ {
		extractECMATopLevel(fe, root.Child(i), source)
	}

	return fe
}

func extractECMATopLevel(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	if n == nil {
		return
	}
	switch n.Type() {
	case "function_declaration", "generator_function_declaration":
		extractJSFunc(fe, n, source)
	case "class_declaration", "abstract_class_declaration":
		extractJSClass(fe, n, source)
	case "interface_declaration", "type_alias_declaration", "enum_declaration":
		extractTSNamedType(fe, n, source)
	case "lexical_declaration", "variable_declaration":
		extractJSVarDecl(fe, n, source, "")
	case "expression_statement":
		walkJSUsages(fe, n, source, "")
	case "export_statement":
		extractJSExport(fe, n, source)
	case "import_statement":
		extractJSImport(fe, n, source)
	}
}

// extractTSNamedType records TypeScript interface / type alias / enum names.
func extractTSNamedType(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      ingest.NodeText(nameNode, source),
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  true,
	})
}
func extractJSExport(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	// export … from "…" / export * from "…" — barrel hop; source field is on the export_statement.
	if srcNode := ingest.ChildByField(n, "source"); srcNode != nil {
		if fragment := ingest.ChildByType(srcNode, "string_fragment"); fragment != nil {
			sourcePath := ingest.NodeText(fragment, source)
			extractJSReexport(fe, n, source, sourcePath)
		}
		return
	}

	// Local export list (JS-specific shapes, including compiled `export { x as default }`).
	if clause := ingest.ChildByType(n, "export_clause"); clause != nil {
		extractJSLocalExportClause(fe, clause, source)
		return
	}

	isDefault := false
	for j := uint32(0); j < n.ChildCount(); j++ {
		if n.Child(j).Type() == "default" {
			isDefault = true
			break
		}
	}

	for j := uint32(0); j < n.ChildCount(); j++ {
		inner := n.Child(j)
		switch inner.Type() {
		case "function_declaration", "generator_function_declaration":
			extractJSFunc(fe, inner, source)
			if isDefault {
				if nameNode := ingest.ChildByField(inner, "name"); nameNode != nil {
					fe.DefaultExport = ingest.NodeText(nameNode, source)
				}
			}
		case "class_declaration", "abstract_class_declaration":
			extractJSClass(fe, inner, source)
			if isDefault {
				if nameNode := ingest.ChildByField(inner, "name"); nameNode != nil {
					fe.DefaultExport = ingest.NodeText(nameNode, source)
				}
			}
		case "interface_declaration", "type_alias_declaration", "enum_declaration":
			extractTSNamedType(fe, inner, source)
		case "lexical_declaration", "variable_declaration":
			extractJSVarDecl(fe, inner, source, "")
		case "identifier":
			// export default someName
			if isDefault {
				fe.DefaultExport = ingest.NodeText(inner, source)
			}
		default:
			// export default defineConfig({...}) — usages inside the expression.
			if inner.IsNamed() {
				walkJSUsages(fe, inner, source, "")
			}
		}
	}
}

// extractJSLocalExportClause is JS-only: `export { a, b as c }` and the common
// bundler form `export { createIntegration as default }` (sets DefaultExport for core).
func extractJSLocalExportClause(fe *ingest.FileExtract, clause *grammar.Node, source []byte) {
	for i := uint32(0); i < clause.ChildCount(); i++ {
		spec := clause.Child(i)
		if spec.Type() != "export_specifier" {
			continue
		}
		nameNode := ingest.ChildByField(spec, "name")
		if nameNode == nil {
			continue
		}
		sourceName := ingest.NodeText(nameNode, source)
		exportName := sourceName
		if aliasNode := ingest.ChildByField(spec, "alias"); aliasNode != nil {
			exportName = ingest.NodeText(aliasNode, source)
		}
		// ESM default export via alias — only this branch sets the neutral DefaultExport field.
		if exportName == "default" {
			fe.DefaultExport = sourceName
		}
	}
}

func extractJSReexport(fe *ingest.FileExtract, n *grammar.Node, source []byte, sourcePath string) {
	// export * from "mod"
	for j := uint32(0); j < n.ChildCount(); j++ {
		if n.Child(j).Type() == "*" {
			fe.Reexports = append(fe.Reexports, ingest.ReexportDef{
				SourcePath: sourcePath,
				Star:       true,
			})
			return
		}
	}

	// export { a, b as c } from "mod"
	clause := ingest.ChildByType(n, "export_clause")
	if clause == nil {
		// export default from is rare; treat whole default as star-like hop not supported.
		return
	}
	for i := uint32(0); i < clause.ChildCount(); i++ {
		spec := clause.Child(i)
		if spec.Type() != "export_specifier" {
			continue
		}
		nameNode := ingest.ChildByField(spec, "name")
		if nameNode == nil {
			continue
		}
		sourceName := ingest.NodeText(nameNode, source)
		exportName := sourceName
		if aliasNode := ingest.ChildByField(spec, "alias"); aliasNode != nil {
			exportName = ingest.NodeText(aliasNode, source)
		}
		fe.Reexports = append(fe.Reexports, ingest.ReexportDef{
			ExportName: exportName,
			SourceName: sourceName,
			SourcePath: sourcePath,
		})
	}
}

func extractJSVarDecl(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() != "variable_declarator" {
			continue
		}
		nameNode := ingest.ChildByField(child, "name")
		if nameNode != nil && nameNode.Type() == "identifier" {
			name := ingest.NodeText(nameNode, source)
			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      name,
				StartByte: nameNode.StartByte(),
				EndByte:   nameNode.EndByte(),
				Exported:  true,
			})
		}
		if value := ingest.ChildByField(child, "value"); value != nil {
			walkJSUsages(fe, value, source, scope)
		}
	}
}

func extractJSFunc(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	name := ingest.NodeText(nameNode, source)

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      name,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  true,
	})

	if body := ingest.ChildByField(n, "body"); body != nil {
		walkJSUsages(fe, body, source, name)
	}
}

func extractJSClass(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	className := ingest.NodeText(nameNode, source)

	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      className,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  true,
	})

	body := ingest.ChildByField(n, "body")
	if body == nil {
		return
	}

	for i := uint32(0); i < body.ChildCount(); i++ {
		member := body.Child(i)
		if member.Type() != "method_definition" {
			continue
		}
		methodNameNode := ingest.ChildByField(member, "name")
		if methodNameNode == nil {
			continue
		}
		// Skip computed property names (e.g. [Symbol.asyncDispose]) — they are
		// runtime-determined and cannot be statically refactored.
		if methodNameNode.Type() == "computed_property_name" {
			continue
		}

		methodShort := ingest.NodeText(methodNameNode, source)
		methodName := className + "." + methodShort
		fe.Entities = append(fe.Entities, ingest.EntityDef{
			Name:      methodName,
			StartByte: methodNameNode.StartByte(),
			EndByte:   methodNameNode.EndByte(),
			Exported:  true,
		})

		if methodBody := ingest.ChildByField(member, "body"); methodBody != nil {
			walkJSUsages(fe, methodBody, source, methodName)
		}
	}
}

func extractJSImport(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	sourceNode := ingest.ChildByField(n, "source")
	if sourceNode == nil {
		return
	}
	fragment := ingest.ChildByType(sourceNode, "string_fragment")
	if fragment == nil {
		return
	}
	sourcePath := ingest.NodeText(fragment, source)

	clause := ingest.ChildByType(n, "import_clause")
	if clause == nil {
		return
	}

	for i := uint32(0); i < clause.ChildCount(); i++ {
		child := clause.Child(i)
		switch child.Type() {
		case "named_imports":
			extractJSNamedImports(fe, child, source, sourcePath)
		case "namespace_import":
			extractJSNamespaceImport(fe, child, source, sourcePath)
		case "identifier":
			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:  ingest.NodeText(child, source),
				SourcePath: sourcePath,
				StartByte:  child.StartByte(),
				EndByte:    child.EndByte(),
			})
		}
	}
}

func extractJSNamedImports(fe *ingest.FileExtract, n *grammar.Node, source []byte, sourcePath string) {
	for i := uint32(0); i < n.ChildCount(); i++ {
		spec := n.Child(i)
		if spec.Type() != "import_specifier" {
			continue
		}
		nameNode := ingest.ChildByField(spec, "name")
		if nameNode == nil {
			continue
		}
		memberName := ingest.NodeText(nameNode, source)

		aliasNode := ingest.ChildByField(spec, "alias")
		localName := memberName
		startByte := nameNode.StartByte()
		endByte := nameNode.EndByte()
		if aliasNode != nil {
			localName = ingest.NodeText(aliasNode, source)
			startByte = aliasNode.StartByte()
			endByte = aliasNode.EndByte()
		}

		fe.Imports = append(fe.Imports, ingest.ImportDef{
			LocalName:       localName,
			SourcePath:      sourcePath,
			MemberName:      memberName,
			StartByte:       startByte,
			EndByte:         endByte,
			TargetStartByte: nameNode.StartByte(),
			TargetEndByte:   nameNode.EndByte(),
			HasAliasBinding: aliasNode != nil,
		})
	}
}

func extractJSNamespaceImport(fe *ingest.FileExtract, n *grammar.Node, source []byte, sourcePath string) {
	var nameNode *grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Type() == "identifier" {
			nameNode = c
		}
	}
	if nameNode == nil {
		return
	}
	fe.Imports = append(fe.Imports, ingest.ImportDef{
		LocalName:  ingest.NodeText(nameNode, source),
		SourcePath: sourcePath,
		StartByte:  nameNode.StartByte(),
		EndByte:    nameNode.EndByte(),
	})
}

func walkJSUsages(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	switch n.Type() {
	case "call_expression":
		funcNode := ingest.ChildByField(n, "function")
		if funcNode != nil {
			emitJSIdentifierUsage(fe, funcNode, source, scope)
		}
		if args := ingest.ChildByField(n, "arguments"); args != nil {
			walkJSUsages(fe, args, source, scope)
		}
		return
	case "jsx_self_closing_element", "jsx_opening_element":
		// JSX elements are component invocations: <Component /> or <Component>.
		// Emit a usage for the component name so renames and moves track them.
		if nameNode := ingest.ChildByField(n, "name"); nameNode != nil {
			emitJSIdentifierUsage(fe, nameNode, source, scope)
		}
		// Walk attribute values for nested usages (e.g. onClick={handler}).
		for i := uint32(0); i < n.ChildCount(); i++ {
			child := n.Child(i)
			if child.Type() == "jsx_attribute" {
				walkJSUsages(fe, child, source, scope)
			}
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		walkJSUsages(fe, n.Child(i), source, scope)
	}
}

// emitJSIdentifierUsage records a usage for a function/component reference node.
// Handles both plain identifiers and member expressions (pkg.Component).
// Nested chains like Box.getValue.call emit leaf usages only (Box.getValue with
// Qualifier "Box"), never a qualifier span covering a nested member expression —
// resolveQualifiedUsage would otherwise treat the whole "Box.getValue" span as a
// relation target and rename would replace it with the leaf alone.
func emitJSIdentifierUsage(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	switch n.Type() {
	case "identifier":
		fe.Usages = append(fe.Usages, ingest.UsageDef{
			Scope:     scope,
			Name:      ingest.NodeText(n, source),
			StartByte: n.StartByte(),
			EndByte:   n.EndByte(),
		})
	case "member_expression", "member_expression_optional", "optional_chain":
		obj := ingest.ChildByField(n, "object")
		prop := ingest.ChildByField(n, "property")
		if obj == nil || prop == nil {
			return
		}
		// Recurse so Box.getValue.call also records Box.getValue (leaf getValue).
		switch obj.Type() {
		case "member_expression", "member_expression_optional", "optional_chain":
			emitJSIdentifierUsage(fe, obj, source, scope)
			return
		case "identifier", "this", "super":
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:         scope,
				Name:          ingest.NodeText(prop, source),
				StartByte:     prop.StartByte(),
				EndByte:       prop.EndByte(),
				Qualifier:     ingest.NodeText(obj, source),
				QualStartByte: obj.StartByte(),
				QualEndByte:   obj.EndByte(),
			})
		}
	}
}

func resolveKnownJSPath(rel string, knownFiles map[string]bool) (string, bool) {
	if knownFiles[rel] {
		return ingest.FileRef("./" + rel), true
	}
	for _, ext := range ecmaResolveExtensions {
		if knownFiles[rel+ext] {
			return ingest.FileRef("./" + rel + ext), true
		}
	}
	for _, indexName := range ecmaIndexNames {
		p := filepath.ToSlash(filepath.Join(rel, indexName))
		if knownFiles[p] {
			return ingest.FileRef("./" + p), true
		}
	}
	return "", false
}

// ecmaResolveExtensions is the order ECMA/TS-style resolution tries for bare paths.
var ecmaResolveExtensions = []string{".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx"}

// ecmaIndexNames are directory entry candidates (JS then TS).
var ecmaIndexNames = []string{
	"index.js", "index.mjs", "index.cjs",
	"index.ts", "index.tsx", "index.jsx",
}

func resolveJSFileOnDisk(baseAbs string, preferPackageMain bool) (string, bool) {
	if st, err := os.Stat(baseAbs); err == nil {
		if !st.IsDir() {
			return baseAbs, true
		}

		if preferPackageMain {
			if resolved, ok := resolvePackageEntrypoint(baseAbs, ""); ok {
				return resolved, true
			}
		}

		for _, indexName := range ecmaIndexNames {
			candidate := filepath.Join(baseAbs, indexName)
			if st2, err := os.Stat(candidate); err == nil && !st2.IsDir() {
				return candidate, true
			}
		}
	}

	for _, ext := range ecmaResolveExtensions {
		candidate := baseAbs + ext
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, true
		}
	}

	return "", false
}

// packageJSON mirrors the fields Node consults when resolving a package entrypoint.
type packageJSON struct {
	Main    string          `json:"main"`
	Module  string          `json:"module"`
	Browser json.RawMessage `json:"browser"`
	Exports json.RawMessage `json:"exports"`
}

// resolvePackageEntrypoint resolves a Node package root (or subpath within it) using
// package.json conventions: exports (preferred), then module, then main, then index.*.
// subpath is without a leading "./" (e.g. "" for the package root, "config" for astro/config).
func resolvePackageEntrypoint(pkgRoot, subpath string) (string, bool) {
	pkgPath := filepath.Join(pkgRoot, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return "", false
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", false
	}

	// 1. exports field — modern Node resolution (takes precedence when present; main/module ignored).
	if len(pkg.Exports) > 0 {
		if entry, ok := resolveExportsEntry(pkg.Exports, subpath); ok {
			return resolvePackageRelative(pkgRoot, entry)
		}
		// exports is authoritative: unexported subpaths (and missing ".") fail, no main/module fallback.
		return "", false
	}

	// Subpaths without exports: fall back to filesystem (legacy packages).
	if subpath != "" {
		target := filepath.Join(pkgRoot, filepath.FromSlash(subpath))
		return resolveJSFileOnDisk(target, false)
	}

	// 2. module (ESM entry; common in browser/bundler packages).
	if pkg.Module != "" {
		if resolved, ok := resolvePackageRelative(pkgRoot, pkg.Module); ok {
			return resolved, true
		}
	}

	// 3. main (CommonJS / Node classic entry).
	if pkg.Main != "" {
		if resolved, ok := resolvePackageRelative(pkgRoot, pkg.Main); ok {
			return resolved, true
		}
	}

	// 4. browser field as a string (simple redirect; object form is ignored).
	if len(pkg.Browser) > 0 {
		var browserStr string
		if err := json.Unmarshal(pkg.Browser, &browserStr); err == nil && browserStr != "" {
			if resolved, ok := resolvePackageRelative(pkgRoot, browserStr); ok {
				return resolved, true
			}
		}
	}

	// 5. index.* convention (JS then TS).
	for _, indexName := range ecmaIndexNames {
		candidate := filepath.Join(pkgRoot, indexName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, true
		}
	}

	return "", false
}

func resolvePackageRelative(pkgRoot, entry string) (string, bool) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", false
	}
	// package.json entries are package-relative; strip optional "./".
	entry = strings.TrimPrefix(entry, "./")
	abs := filepath.Join(pkgRoot, filepath.FromSlash(entry))
	return resolveJSFileOnDisk(abs, false)
}

// resolveExportsEntry picks a target string from package.json "exports" for the given
// import subpath ("" means package root / ".").
func resolveExportsEntry(exportsRaw json.RawMessage, subpath string) (string, bool) {
	// exports can be a string: "." only.
	var asString string
	if err := json.Unmarshal(exportsRaw, &asString); err == nil {
		if subpath == "" && asString != "" {
			return asString, true
		}
		return "", false
	}

	// exports as a map keyed by subpath (".", "./config", ...) or condition object for root.
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(exportsRaw, &asMap); err != nil {
		return "", false
	}

	key := "."
	if subpath != "" {
		key = "./" + subpath
	}

	// Direct key match.
	if raw, ok := asMap[key]; ok {
		return pickExportTarget(raw)
	}

	// Root request with a condition-only exports object (no "." key, keys are conditions).
	if subpath == "" {
		if target, ok := pickExportTarget(exportsRaw); ok {
			return target, true
		}
	}

	// Pattern keys like "./tsconfigs/*.json" — support a single "*" segment.
	for pattern, raw := range asMap {
		if !strings.Contains(pattern, "*") {
			continue
		}
		matched, ok := matchExportPattern(pattern, key)
		if !ok {
			continue
		}
		target, ok := pickExportTarget(raw)
		if !ok {
			continue
		}
		target = strings.Replace(target, "*", matched, 1)
		return target, true
	}

	return "", false
}

// pickExportTarget resolves an exports value which may be a string or a conditions object.
// Prefers ESM-oriented conditions for static import analysis.
func pickExportTarget(raw json.RawMessage) (string, bool) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil && asString != "" {
		return asString, true
	}

	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		return "", false
	}

	// Condition priority for import/static analysis (skip types — we want runtime JS).
	for _, cond := range []string{"import", "default", "node", "require", "module", "browser", "worker"} {
		if nested, ok := asMap[cond]; ok {
			if target, ok := pickExportTarget(nested); ok {
				return target, true
			}
		}
	}

	// Last resort: any non-types string value in the map (depth-1).
	for k, nested := range asMap {
		if k == "types" {
			continue
		}
		var s string
		if err := json.Unmarshal(nested, &s); err == nil && s != "" {
			return s, true
		}
	}

	return "", false
}

// matchExportPattern matches an exports key pattern (single "*") against a request key.
// Returns the substring captured by "*" when it matches.
func matchExportPattern(pattern, request string) (string, bool) {
	star := strings.Index(pattern, "*")
	if star < 0 {
		return "", false
	}
	prefix := pattern[:star]
	suffix := pattern[star+1:]
	if !strings.HasPrefix(request, prefix) || !strings.HasSuffix(request, suffix) {
		return "", false
	}
	mid := request[len(prefix) : len(request)-len(suffix)]
	// Empty capture is only valid when the pattern explicitly allows it (prefix ends with "/").
	if mid == "" && !strings.HasSuffix(prefix, "/") {
		return "", false
	}
	return mid, true
}

func splitNodePackageSpecifier(spec string) (pkgName, subpath string) {
	parts := strings.Split(spec, "/")
	if spec == "" {
		return "", ""
	}
	if strings.HasPrefix(spec, "@") {
		if len(parts) < 2 {
			return spec, ""
		}
		pkgName = parts[0] + "/" + parts[1]
		if len(parts) > 2 {
			subpath = strings.Join(parts[2:], "/")
		}
		return pkgName, subpath
	}
	pkgName = parts[0]
	if len(parts) > 1 {
		subpath = strings.Join(parts[1:], "/")
	}
	return pkgName, subpath
}

func nodeModuleCandidates(importerAbsDir, pkgName string) []string {
	seen := map[string]bool{}
	out := []string{}

	for dir := importerAbsDir; ; {
		candidate := filepath.Join(dir, "node_modules", filepath.FromSlash(pkgName))
		if !seen[candidate] {
			seen[candidate] = true
			out = append(out, candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if nodePath := os.Getenv("NODE_PATH"); nodePath != "" {
		for _, entry := range filepath.SplitList(nodePath) {
			if entry == "" {
				continue
			}
			candidate := filepath.Join(entry, filepath.FromSlash(pkgName))
			if !seen[candidate] {
				seen[candidate] = true
				out = append(out, candidate)
			}
		}
	}

	return out
}

// GrammarNameForScriptLang maps a <script lang="..."> attribute to a tree-sitter grammar name.
// Empty or "js" → javascript; "ts" / "typescript" → typescript.
func GrammarNameForScriptLang(langAttr string) string {
	switch strings.ToLower(strings.TrimSpace(langAttr)) {
	case "", "js", "javascript":
		return "javascript"
	case "ts", "typescript":
		return "typescript"
	default:
		// Unknown (e.g. coffee): fall back to javascript parse attempt.
		return "javascript"
	}
}

// ExtractECMAScript parses script source with the named grammar ("javascript" or
// "typescript") and returns a FileExtract whose byte offsets are relative to
// script (not a parent SFC). Language on the extract is "javascript" (ECMA family).
func ExtractECMAScript(script []byte, grammarName, relPath string) (*ingest.FileExtract, error) {
	if grammarName == "" {
		grammarName = "javascript"
	}
	lang, ok := grammar.Get(grammarName)
	if !ok {
		return nil, fmt.Errorf("unknown grammar %q", grammarName)
	}
	pf, err := ingest.ParseSourceLanguage(script, lang, grammarName)
	if err != nil {
		return nil, err
	}
	defer pf.Close()
	fe := extractECMA(pf.Root, script, relPath)
	return fe, nil
}

// ResolveECMAImport resolves a path or node module specifier using ECMA rules.
func ResolveECMAImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	return (languageDriver{}).ResolveImport(sourcePath, ctx)
}

// ExtractECMAExpressionUsages parses a short expression (markup {…}, attribute
// values, #each/#if heads) and returns identifier/member usages with offsets
// relative to the expression source.
func ExtractECMAExpressionUsages(expr []byte, grammarName string) ([]ingest.UsageDef, error) {
	if len(bytesTrimSpace(expr)) == 0 {
		return nil, nil
	}
	if grammarName == "" {
		grammarName = "javascript"
	}
	lang, ok := grammar.Get(grammarName)
	if !ok {
		return nil, fmt.Errorf("unknown grammar %q", grammarName)
	}
	pf, err := ingest.ParseSourceLanguage(expr, lang, grammarName)
	if err != nil {
		return nil, err
	}
	defer pf.Close()
	var usages []ingest.UsageDef
	var walk func(n *grammar.Node, skipIdent bool)
	walk = func(n *grammar.Node, skipIdent bool) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "member_expression", "optional_chain", "member_expression_optional":
			// tree-sitter-typescript uses member_expression; property may be property_identifier.
			obj := ingest.ChildByField(n, "object")
			prop := ingest.ChildByField(n, "property")
			if obj != nil {
				walk(obj, false)
			}
			if prop != nil {
				switch prop.Type() {
				case "property_identifier", "identifier", "private_property_identifier":
					usages = append(usages, ingest.UsageDef{
						Name:          ingest.NodeText(prop, expr),
						StartByte:     prop.StartByte(),
						EndByte:       prop.EndByte(),
						Qualifier:     ingest.NodeText(obj, expr),
						QualStartByte: obj.StartByte(),
						QualEndByte:   obj.EndByte(),
					})
				default:
					walk(prop, false)
				}
			}
			return
		case "identifier":
			if skipIdent {
				return
			}
			name := ingest.NodeText(n, expr)
			if name == "" || isECMAKeyword(name) {
				return
			}
			usages = append(usages, ingest.UsageDef{
				Name:      name,
				StartByte: n.StartByte(),
				EndByte:   n.EndByte(),
			})
			return
		case "string", "template_string", "number", "true", "false", "null", "undefined",
			"comment", "regex":
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), false)
		}
	}
	walk(pf.Root, false)
	return usages, nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func isECMAKeyword(name string) bool {
	switch name {
	case "true", "false", "null", "undefined", "this", "super", "new", "typeof", "void",
		"delete", "in", "instanceof", "await", "yield", "as", "satisfies", "is", "keyof",
		"readonly", "infer", "any", "never", "unknown", "string", "number", "boolean",
		"object", "symbol", "bigint", "const", "let", "var", "function", "class", "return",
		"if", "else", "for", "while", "do", "switch", "case", "break", "continue", "try",
		"catch", "finally", "throw", "import", "export", "default", "from", "of", "async":
		return true
	default:
		return false
	}
}
