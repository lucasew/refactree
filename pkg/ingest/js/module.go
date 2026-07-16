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

func extractTSNamedType(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	nameNode := ingest.ChildByField(n, "name")
	if nameNode == nil {
		return
	}
	typeName := ingest.NodeText(nameNode, source)
	if typeName == "" {
		return
	}
	fe.Entities = append(fe.Entities, ingest.EntityDef{
		Name:      typeName,
		StartByte: nameNode.StartByte(),
		EndByte:   nameNode.EndByte(),
		Exported:  true,
	})
	switch n.Type() {
	case "interface_declaration":
		if body := ingest.ChildByField(n, "body"); body != nil {
			extractTSObjectTypeMembers(fe, body, source, typeName)
		}
	case "type_alias_declaration":
		// type Worker = { helper(): number; stay: number }
		if value := ingest.ChildByField(n, "value"); value != nil && value.Type() == "object_type" {
			extractTSObjectTypeMembers(fe, value, source, typeName)
		}
	case "enum_declaration":
		if body := ingest.ChildByField(n, "body"); body != nil {
			extractTSEnumMembers(fe, body, source, typeName)
		}
	}
}

// extractTSObjectTypeMembers records method_signature / property_signature
// members under an interface_body or object_type as Type.member entities.

// extractTSObjectTypeMembers records method_signature / property_signature
// members under an interface_body or object_type as Type.member entities.
func extractTSObjectTypeMembers(fe *ingest.FileExtract, body *grammar.Node, source []byte, typeName string) {
	if body == nil || typeName == "" {
		return
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		member := body.Child(i)
		switch member.Type() {
		case "method_signature", "property_signature", "construct_signature", "call_signature":
			nameNode := ingest.ChildByField(member, "name")
			if nameNode == nil || nameNode.Type() == "computed_property_name" {
				continue
			}
			short := ingest.NodeText(nameNode, source)
			if short == "" {
				continue
			}
			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      typeName + "." + short,
				StartByte: nameNode.StartByte(),
				EndByte:   nameNode.EndByte(),
				Exported:  true,
			})
		}
	}
}

// extractTSEnumMembers records enum constants as Enum.Member entities.
// Members may be bare property_identifier children or enum_assignment nodes.

// extractTSEnumMembers records enum constants as Enum.Member entities.
// Members may be bare property_identifier children or enum_assignment nodes.
func extractTSEnumMembers(fe *ingest.FileExtract, body *grammar.Node, source []byte, enumName string) {
	if body == nil || enumName == "" {
		return
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		child := body.Child(i)
		var nameNode *grammar.Node
		switch child.Type() {
		case "enum_assignment":
			nameNode = ingest.ChildByField(child, "name")
		case "property_identifier":
			// Bare member: `enum Color { Helper, Stay }`
			nameNode = child
		default:
			continue
		}
		if nameNode == nil {
			continue
		}
		short := ingest.NodeText(nameNode, source)
		if short == "" {
			continue
		}
		fe.Entities = append(fe.Entities, ingest.EntityDef{
			Name:      enumName + "." + short,
			StartByte: nameNode.StartByte(),
			EndByte:   nameNode.EndByte(),
			Exported:  true,
		})
	}
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
			// export default someName — record as primary export and as a usage so
			// renaming someName rewrites this reference (not only the definition).
			if isDefault {
				fe.DefaultExport = ingest.NodeText(inner, source)
				fe.Usages = append(fe.Usages, ingest.UsageDef{
					Name:      fe.DefaultExport,
					StartByte: inner.StartByte(),
					EndByte:   inner.EndByte(),
				})
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
		// Local binding reference in `export { name }` / `export { name as alias }` —
		// rename must rewrite this span when the binding is renamed.
		fe.Usages = append(fe.Usages, ingest.UsageDef{
			Name:      sourceName,
			StartByte: nameNode.StartByte(),
			EndByte:   nameNode.EndByte(),
		})
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
		// Span of the imported/source name token — the site rename must rewrite
		// (export { helper } from / export { helper as h } from).
		fe.Reexports = append(fe.Reexports, ingest.ReexportDef{
			ExportName:      exportName,
			SourceName:      sourceName,
			SourcePath:      sourcePath,
			SourceStartByte: nameNode.StartByte(),
			SourceEndByte:   nameNode.EndByte(),
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
		value := ingest.ChildByField(child, "value")
		var localName string
		if nameNode != nil && nameNode.Type() == "identifier" {
			localName = ingest.NodeText(nameNode, source)
			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      localName,
				StartByte: nameNode.StartByte(),
				EndByte:   nameNode.EndByte(),
				Exported:  true,
			})
			// Dynamic import namespace: `const m = await import("./box.js")` or
			// `const m = import("./box.js")` — treat like `import * as m from …`
			// so m.helper property accesses resolve to the module export.
			if value != nil {
				if dynPath, ok := jsDynamicImportPath(value, source); ok {
					fe.Imports = append(fe.Imports, ingest.ImportDef{
						LocalName:  localName,
						SourcePath: dynPath,
						StartByte:  nameNode.StartByte(),
						EndByte:    nameNode.EndByte(),
					})
				}
			}
		}
		if value != nil {
			// CJS: const m = require('./x') / const { a, b: c } = require('./x')
			// Bind like ESM import so m.a and destructured names resolve to the module.
			if reqPath, ok := jsRequireCallPath(value, source); ok {
				bindJSRequireImports(fe, nameNode, source, reqPath)
				if nameNode != nil {
					// Defaults inside patterns (rare): { a = helper() } = require(...)
					walkJSBindingPatternDefaults(fe, nameNode, source, scope)
				}
				continue
			}
			// const Box = class { m() {} } / const api = { m() {} } — methods are
			// path entities under the binding name (class expressions have no
			// declaration path of their own).
			switch value.Type() {
			case "class", "class_expression", "abstract_class_declaration":
				if localName != "" {
					extractJSClassMembers(fe, value, source, localName)
					continue
				}
			case "object":
				if localName != "" {
					extractJSObjectMembers(fe, value, source, localName, scope)
					continue
				}
			}
			walkJSUsages(fe, value, source, scope)
		}
	}
}

// jsDynamicImportPath returns the string specifier for `import("…")` or
// `await import("…")`. Empty when the RHS is not a dynamic import expression.
func jsDynamicImportPath(n *grammar.Node, source []byte) (string, bool) {
	if n == nil {
		return "", false
	}
	// Unwrap await_expression.
	if n.Type() == "await_expression" {
		for i := uint32(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c.Type() == "await" {
				continue
			}
			return jsDynamicImportPath(c, source)
		}
		return "", false
	}
	return jsImportCallPath(n, source)
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

	// Default parameter values (n = helper()) are value references, not bindings.
	if params := ingest.ChildByField(n, "parameters"); params != nil {
		walkJSUsages(fe, params, source, name)
	}
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
	extractJSClassMembers(fe, n, source, className)
}

// extractJSClassMembers records methods/fields of a class_declaration or class
// expression under ownerName (declaration name or the const/let binding that
// holds the class).
func extractJSClassMembers(fe *ingest.FileExtract, n *grammar.Node, source []byte, ownerName string) {
	if ownerName == "" {
		return
	}
	// `class Sub extends Box` — heritage is not under body.
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child.Type() == "class_heritage" {
			walkJSUsages(fe, child, source, ownerName)
		}
	}

	body := ingest.ChildByField(n, "body")
	if body == nil {
		return
	}

	for i := uint32(0); i < body.ChildCount(); i++ {
		member := body.Child(i)
		switch member.Type() {
		case "field_definition":
			// Class fields (instance/static/private/arrow): `helper = 1`, `static stay = 2`,
			// `#priv = 3`, `helper = () => 1` → entities Class.helper so rename rewrites
			// definitions and ExtraRename covers this.helper / Box.helper call sites.
			if prop := ingest.ChildByField(member, "property"); prop != nil {
				if prop.Type() != "computed_property_name" {
					short := ingest.NodeText(prop, source)
					if short != "" {
						fieldName := ownerName + "." + short
						fe.Entities = append(fe.Entities, ingest.EntityDef{
							Name:      fieldName,
							StartByte: prop.StartByte(),
							EndByte:   prop.EndByte(),
							Exported:  true,
						})
					}
				}
			}
			// Class field initializers: `b = new Box(1)` — walk value usages.
			if value := ingest.ChildByField(member, "value"); value != nil {
				walkJSUsages(fe, value, source, ownerName)
			}
		case "method_definition":
			methodNameNode := ingest.ChildByField(member, "name")
			if methodNameNode == nil {
				continue
			}
			// Skip computed property names (e.g. [Symbol.asyncDispose]) — they are
			// runtime-determined and cannot be statically refactored.
			if methodNameNode.Type() == "computed_property_name" {
				if params := ingest.ChildByField(member, "parameters"); params != nil {
					walkJSUsages(fe, params, source, ownerName)
				}
				if methodBody := ingest.ChildByField(member, "body"); methodBody != nil {
					walkJSUsages(fe, methodBody, source, ownerName)
				}
				continue
			}

			methodShort := ingest.NodeText(methodNameNode, source)
			methodName := ownerName + "." + methodShort
			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      methodName,
				StartByte: methodNameNode.StartByte(),
				EndByte:   methodNameNode.EndByte(),
				Exported:  true,
			})

			if params := ingest.ChildByField(member, "parameters"); params != nil {
				walkJSUsages(fe, params, source, methodName)
			}
			if methodBody := ingest.ChildByField(member, "body"); methodBody != nil {
				walkJSUsages(fe, methodBody, source, methodName)
			}
		case "class_static_block":
			// `static { helper() }` — body is a statement_block of free references.
			if blockBody := ingest.ChildByField(member, "body"); blockBody != nil {
				walkJSUsages(fe, blockBody, source, ownerName)
			}
		}
	}
}

// extractJSObjectMembers records non-computed object methods / function-valued
// properties as owner.method entities (const api = { helper() {}, stay: fn }).
// Shorthand / value properties still produce usages of free identifiers under
// enclosingScope (not ownerName — they are not members of the object binding).
func extractJSObjectMembers(fe *ingest.FileExtract, n *grammar.Node, source []byte, ownerName, enclosingScope string) {
	if n == nil || ownerName == "" {
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		switch child.Type() {
		case "method_definition":
			methodNameNode := ingest.ChildByField(child, "name")
			if methodNameNode == nil || methodNameNode.Type() == "computed_property_name" {
				if params := ingest.ChildByField(child, "parameters"); params != nil {
					walkJSUsages(fe, params, source, enclosingScope)
				}
				if methodBody := ingest.ChildByField(child, "body"); methodBody != nil {
					walkJSUsages(fe, methodBody, source, enclosingScope)
				}
				continue
			}
			methodShort := ingest.NodeText(methodNameNode, source)
			methodName := ownerName + "." + methodShort
			fe.Entities = append(fe.Entities, ingest.EntityDef{
				Name:      methodName,
				StartByte: methodNameNode.StartByte(),
				EndByte:   methodNameNode.EndByte(),
				Exported:  true,
			})
			if params := ingest.ChildByField(child, "parameters"); params != nil {
				walkJSUsages(fe, params, source, methodName)
			}
			if methodBody := ingest.ChildByField(child, "body"); methodBody != nil {
				walkJSUsages(fe, methodBody, source, methodName)
			}
		case "pair":
			key := ingest.ChildByField(child, "key")
			val := ingest.ChildByField(child, "value")
			if key == nil || val == nil || key.Type() == "computed_property_name" {
				if val != nil {
					walkJSUsages(fe, val, source, enclosingScope)
				}
				continue
			}
			switch val.Type() {
			case "function_expression", "function", "arrow_function", "generator_function":
				methodShort := ingest.NodeText(key, source)
				methodName := ownerName + "." + methodShort
				fe.Entities = append(fe.Entities, ingest.EntityDef{
					Name:      methodName,
					StartByte: key.StartByte(),
					EndByte:   key.EndByte(),
					Exported:  true,
				})
				if params := ingest.ChildByField(val, "parameters"); params != nil {
					walkJSUsages(fe, params, source, methodName)
				}
				if methodBody := ingest.ChildByField(val, "body"); methodBody != nil {
					walkJSUsages(fe, methodBody, source, methodName)
				}
			default:
				// `{ k: helper }` — free value reference under enclosing scope.
				walkJSUsages(fe, val, source, enclosingScope)
			}
		case "spread_element", "shorthand_property_identifier":
			// `{ ...other }` / `{ helper }` shorthand — free identifier usages.
			walkJSUsages(fe, child, source, enclosingScope)
		default:
			if child.IsNamed() {
				walkJSUsages(fe, child, source, enclosingScope)
			}
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

// jsImportCallPath returns the string specifier for a bare `import("…")` call.
// Does not unwrap await (used when import() is the object of `.then`).
func jsImportCallPath(n *grammar.Node, source []byte) (string, bool) {
	return jsCallStringArg(n, source, "import")
}

// jsCallStringArg returns the first string argument of call_expression callee `name`.
func jsCallStringArg(n *grammar.Node, source []byte, name string) (string, bool) {
	if n == nil || n.Type() != "call_expression" {
		return "", false
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return "", false
	}
	fnText := ingest.NodeText(fn, source)
	if fnText != name && fn.Type() != name {
		return "", false
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return "", false
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		arg := args.Child(i)
		if arg.Type() != "string" {
			continue
		}
		if frag := ingest.ChildByType(arg, "string_fragment"); frag != nil {
			return ingest.NodeText(frag, source), true
		}
	}
	return "", false
}

// bindJSRequireImports records CJS require bindings as ImportDefs (namespace or named).

// jsRequireCallPath returns the string specifier for `require("…")`.
func jsRequireCallPath(n *grammar.Node, source []byte) (string, bool) {
	return jsCallStringArg(n, source, "require")
}

// jsCallStringArg returns the first string argument of call_expression callee `name`.

// bindJSRequireImports records CJS require bindings as ImportDefs (namespace or named).
func bindJSRequireImports(fe *ingest.FileExtract, nameNode *grammar.Node, source []byte, sourcePath string) {
	if fe == nil || nameNode == nil || sourcePath == "" {
		return
	}
	switch nameNode.Type() {
	case "identifier":
		// const m = require('./mod')
		fe.Imports = append(fe.Imports, ingest.ImportDef{
			LocalName:  ingest.NodeText(nameNode, source),
			SourcePath: sourcePath,
			StartByte:  nameNode.StartByte(),
			EndByte:    nameNode.EndByte(),
		})
	case "object_pattern":
		// const { helper, stay: s } = require('./mod')
		for i := uint32(0); i < nameNode.ChildCount(); i++ {
			ch := nameNode.Child(i)
			switch ch.Type() {
			case "shorthand_property_identifier_pattern":
				member := ingest.NodeText(ch, source)
				fe.Imports = append(fe.Imports, ingest.ImportDef{
					LocalName:       member,
					SourcePath:      sourcePath,
					MemberName:      member,
					StartByte:       ch.StartByte(),
					EndByte:         ch.EndByte(),
					TargetStartByte: ch.StartByte(),
					TargetEndByte:   ch.EndByte(),
				})
			case "pair_pattern":
				key := ingest.ChildByField(ch, "key")
				val := ingest.ChildByField(ch, "value")
				if key == nil || val == nil {
					continue
				}
				member := ingest.NodeText(key, source)
				// value may be identifier or assignment_pattern (default).
				localNode := val
				if val.Type() == "assignment_pattern" {
					if left := ingest.ChildByField(val, "left"); left != nil {
						localNode = left
					}
				}
				if localNode.Type() != "identifier" {
					continue
				}
				local := ingest.NodeText(localNode, source)
				fe.Imports = append(fe.Imports, ingest.ImportDef{
					LocalName:       local,
					SourcePath:      sourcePath,
					MemberName:      member,
					StartByte:       localNode.StartByte(),
					EndByte:         localNode.EndByte(),
					TargetStartByte: key.StartByte(),
					TargetEndByte:   key.EndByte(),
					HasAliasBinding: local != member,
				})
			}
		}
	}
}

// jsIsCJSExportsTarget reports whether n is `exports` or `module.exports`.

// jsIsCJSExportsTarget reports whether n is `exports` or `module.exports`.
func jsIsCJSExportsTarget(n *grammar.Node, source []byte) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "identifier":
		return ingest.NodeText(n, source) == "exports"
	case "member_expression":
		obj := ingest.ChildByField(n, "object")
		prop := ingest.ChildByField(n, "property")
		if obj == nil || prop == nil {
			return false
		}
		return obj.Type() == "identifier" &&
			ingest.NodeText(obj, source) == "module" &&
			ingest.NodeText(prop, source) == "exports"
	}
	return false
}

// emitJSCJSExportSurfaceUsages records property keys on CJS export assignments so
// renaming a symbol rewrites exports.helper / module.exports = { helper: … } keys.

// emitJSCJSExportSurfaceUsages records property keys on CJS export assignments so
// renaming a symbol rewrites exports.helper / module.exports = { helper: … } keys.
func emitJSCJSExportSurfaceUsages(fe *ingest.FileExtract, left, right *grammar.Node, source []byte, scope string) {
	if left == nil {
		return
	}
	// exports.helper = … / module.exports.helper = …
	if left.Type() == "member_expression" {
		obj := ingest.ChildByField(left, "object")
		prop := ingest.ChildByField(left, "property")
		if prop != nil && jsIsCJSExportsTarget(obj, source) {
			// Bare usage so resolve links the key to a same-file entity of that name.
			fe.Usages = append(fe.Usages, ingest.UsageDef{
				Scope:     scope,
				Name:      ingest.NodeText(prop, source),
				StartByte: prop.StartByte(),
				EndByte:   prop.EndByte(),
			})
		}
	}
	// module.exports = { helper, stay: … } / exports = { … }
	if right != nil && right.Type() == "object" && jsIsCJSExportsTarget(left, source) {
		for i := uint32(0); i < right.ChildCount(); i++ {
			ch := right.Child(i)
			switch ch.Type() {
			case "shorthand_property_identifier":
				// Already emitted as bare usage when walking the object; skip double.
			case "pair":
				if key := ingest.ChildByField(ch, "key"); key != nil &&
					(key.Type() == "property_identifier" || key.Type() == "identifier") {
					fe.Usages = append(fe.Usages, ingest.UsageDef{
						Scope:     scope,
						Name:      ingest.NodeText(key, source),
						StartByte: key.StartByte(),
						EndByte:   key.EndByte(),
					})
				}
			case "method_definition":
				if name := ingest.ChildByField(ch, "name"); name != nil &&
					(name.Type() == "property_identifier" || name.Type() == "identifier") {
					fe.Usages = append(fe.Usages, ingest.UsageDef{
						Scope:     scope,
						Name:      ingest.NodeText(name, source),
						StartByte: name.StartByte(),
						EndByte:   name.EndByte(),
					})
				}
			}
		}
	}
}

// bindJSDynamicImportThen records a namespace-style import for the first
// parameter of `import("…").then(m => …)` / `.then(function (m) { … })`.

// bindJSDynamicImportThen records a namespace-style import for the first
// parameter of `import("…").then(m => …)` / `.then(function (m) { … })`.
func bindJSDynamicImportThen(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	if fe == nil || n == nil || n.Type() != "call_expression" {
		return
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "member_expression" {
		return
	}
	prop := ingest.ChildByField(fn, "property")
	if prop == nil || ingest.NodeText(prop, source) != "then" {
		return
	}
	obj := ingest.ChildByField(fn, "object")
	dynPath, ok := jsImportCallPath(obj, source)
	if !ok || dynPath == "" {
		return
	}
	args := ingest.ChildByField(n, "arguments")
	if args == nil {
		return
	}
	var cb *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		arg := args.Child(i)
		switch arg.Type() {
		case "arrow_function", "function_expression", "generator_function":
			cb = arg
		}
		if cb != nil {
			break
		}
	}
	if cb == nil {
		return
	}
	// First formal parameter: namespace ident or destructured named imports.
	params := ingest.ChildByField(cb, "parameters")
	var param *grammar.Node
	if params != nil {
		for i := uint32(0); i < params.ChildCount(); i++ {
			p := params.Child(i)
			switch p.Type() {
			case "identifier", "object_pattern":
				param = p
			}
			if param != nil {
				break
			}
		}
	} else {
		// Single-param arrow without parens: `m => m.helper()` — parameter field.
		if p := ingest.ChildByField(cb, "parameter"); p != nil {
			switch p.Type() {
			case "identifier", "object_pattern":
				param = p
			}
		}
	}
	if param == nil {
		return
	}
	if param.Type() == "object_pattern" {
		bindJSObjectPatternNamedImports(fe, param, source, dynPath)
		return
	}
	local := ingest.NodeText(param, source)
	if local == "" {
		return
	}
	fe.Imports = append(fe.Imports, ingest.ImportDef{
		LocalName:  local,
		SourcePath: dynPath,
		StartByte:  param.StartByte(),
		EndByte:    param.EndByte(),
	})
}

// bindJSObjectPatternNamedImports records named imports for each shorthand /
// pair in an object pattern bound from a dynamic import module namespace.
// e.g. `{ helper, stay: s }` from `import("./box.js")` → member imports.
func bindJSObjectPatternNamedImports(fe *ingest.FileExtract, pattern *grammar.Node, source []byte, dynPath string) {
	if fe == nil || pattern == nil || pattern.Type() != "object_pattern" || dynPath == "" {
		return
	}
	for i := uint32(0); i < pattern.ChildCount(); i++ {
		ch := pattern.Child(i)
		switch ch.Type() {
		case "shorthand_property_identifier_pattern":
			name := ingest.NodeText(ch, source)
			if name == "" {
				continue
			}
			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:       name,
				SourcePath:      dynPath,
				MemberName:      name,
				StartByte:       ch.StartByte(),
				EndByte:         ch.EndByte(),
				TargetStartByte: ch.StartByte(),
				TargetEndByte:   ch.EndByte(),
			})
		case "pair_pattern":
			// `{ helper: h }` — key is export name, value is local binding.
			key := ingest.ChildByField(ch, "key")
			val := ingest.ChildByField(ch, "value")
			if key == nil || val == nil {
				continue
			}
			memberName := ingest.NodeText(key, source)
			var localNode *grammar.Node
			switch val.Type() {
			case "identifier", "shorthand_property_identifier_pattern":
				localNode = val
			case "assignment_pattern":
				if left := ingest.ChildByField(val, "left"); left != nil {
					localNode = left
				}
			}
			if memberName == "" || localNode == nil {
				continue
			}
			local := ingest.NodeText(localNode, source)
			fe.Imports = append(fe.Imports, ingest.ImportDef{
				LocalName:       local,
				SourcePath:      dynPath,
				MemberName:      memberName,
				StartByte:       localNode.StartByte(),
				EndByte:         localNode.EndByte(),
				TargetStartByte: key.StartByte(),
				TargetEndByte:   key.EndByte(),
				HasAliasBinding: local != memberName,
			})
		case "object_assignment_pattern":
			// `{ helper = fallback }` treated as shorthand with default.
			if left := ingest.ChildByField(ch, "left"); left != nil {
				if left.Type() == "shorthand_property_identifier_pattern" || left.Type() == "identifier" {
					name := ingest.NodeText(left, source)
					if name == "" {
						continue
					}
					fe.Imports = append(fe.Imports, ingest.ImportDef{
						LocalName:       name,
						SourcePath:      dynPath,
						MemberName:      name,
						StartByte:       left.StartByte(),
						EndByte:         left.EndByte(),
						TargetStartByte: left.StartByte(),
						TargetEndByte:   left.EndByte(),
					})
				}
			}
		}
	}
}

// bindJSDynamicImportDeclarator records named imports for
// `const { helper } = await import("./x")` / `const { helper } = import("./x")`.
func bindJSDynamicImportDeclarator(fe *ingest.FileExtract, n *grammar.Node, source []byte) {
	if fe == nil || n == nil || n.Type() != "variable_declarator" {
		return
	}
	name := ingest.ChildByField(n, "name")
	value := ingest.ChildByField(n, "value")
	if name == nil || value == nil || name.Type() != "object_pattern" {
		return
	}
	// Unwrap await: await import("…")
	call := value
	if value.Type() == "await_expression" {
		// await_expression argument / first non-keyword child
		if arg := ingest.ChildByField(value, "argument"); arg != nil {
			call = arg
		} else {
			for i := uint32(0); i < value.ChildCount(); i++ {
				ch := value.Child(i)
				if ch.Type() == "await" {
					continue
				}
				call = ch
				break
			}
		}
	}
	dynPath, ok := jsImportCallPath(call, source)
	if !ok || dynPath == "" {
		return
	}
	bindJSObjectPatternNamedImports(fe, name, source, dynPath)
}

func walkJSUsages(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if n == nil || n.IsNull() {
		return
	}
	switch n.Type() {
	case "assignment_expression":
		// CJS export surface: exports.helper = … / module.exports = { helper: … }
		// so rename rewrites the public property key with the local symbol.
		left := ingest.ChildByField(n, "left")
		right := ingest.ChildByField(n, "right")
		emitJSCJSExportSurfaceUsages(fe, left, right, source, scope)
		if left != nil {
			// Avoid double-recording exports.helper as qualified usage of helper;
			// the bare key usage from emitJSCJSExportSurfaceUsages is the rename site.
			if left.Type() == "member_expression" {
				obj := ingest.ChildByField(left, "object")
				if !jsIsCJSExportsTarget(obj, source) {
					walkJSUsages(fe, left, source, scope)
				}
			} else {
				walkJSUsages(fe, left, source, scope)
			}
		}
		if right != nil {
			walkJSUsages(fe, right, source, scope)
		}
		return
	case "call_expression":
		// import("./x").then(m => m.helper()) — bind callback param as namespace import
		// so property accesses resolve like `import * as m from "./x"`.
		bindJSDynamicImportThen(fe, n, source)
		funcNode := ingest.ChildByField(n, "function")
		if funcNode != nil {
			emitJSIdentifierUsage(fe, funcNode, source, scope)
		}
		if args := ingest.ChildByField(n, "arguments"); args != nil {
			walkJSUsages(fe, args, source, scope)
		}
		return
	case "new_expression":
		// `new Box(1)` — constructor field is the type/value being constructed.
		if ctor := ingest.ChildByField(n, "constructor"); ctor != nil {
			emitJSIdentifierUsage(fe, ctor, source, scope)
		}
		if args := ingest.ChildByField(n, "arguments"); args != nil {
			walkJSUsages(fe, args, source, scope)
		}
		return
	case "class_heritage":
		// `extends Box` / `extends pkg.Box` — expression after extends.
		for i := uint32(0); i < n.ChildCount(); i++ {
			child := n.Child(i)
			if child.Type() == "extends" {
				continue
			}
			emitJSIdentifierUsage(fe, child, source, scope)
		}
		return
	case "binary_expression":
		// `x instanceof Box` — right operand is a type/value reference.
		if op := ingest.ChildByField(n, "operator"); op != nil && ingest.NodeText(op, source) == "instanceof" {
			if right := ingest.ChildByField(n, "right"); right != nil {
				emitJSIdentifierUsage(fe, right, source, scope)
			}
			if left := ingest.ChildByField(n, "left"); left != nil {
				walkJSUsages(fe, left, source, scope)
			}
			return
		}
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
	case "identifier", "shorthand_property_identifier":
		// Bare value references: assignment RHS, call args, array elements,
		// object values, object shorthand ({ helper }), ternary/binary operands.
		// Matches Go/Python walk*Usages for residual rename correctness.
		// Object-pattern shorthands use shorthand_property_identifier_pattern
		// (skipped via object_pattern below), so this is literal-only.
		fe.Usages = append(fe.Usages, ingest.UsageDef{
			Scope:     scope,
			Name:      ingest.NodeText(n, source),
			StartByte: n.StartByte(),
			EndByte:   n.EndByte(),
		})
		return
	case "member_expression":
		// obj.prop value/call-function already handled via emit; record leaf + qualifier.
		emitJSIdentifierUsage(fe, n, source, scope)
		return
	case "object_pattern", "array_pattern", "formal_parameters":
		// Binding sites for pattern ids — walk default-value expressions only.
		walkJSBindingPatternDefaults(fe, n, source, scope)
		return
	case "assignment_pattern", "object_assignment_pattern":
		// `n = helper()` / `{ n = helper() }` defaults: right-hand side is a value ref.
		if right := ingest.ChildByField(n, "right"); right != nil {
			walkJSUsages(fe, right, source, scope)
		}
		if left := ingest.ChildByField(n, "left"); left != nil {
			walkJSBindingPatternDefaults(fe, left, source, scope)
		}
		return
	case "function_declaration", "generator_function_declaration",
		"class_declaration", "abstract_class_declaration",
		"method_definition", "variable_declarator":
		// Nested declarations: walk bodies/values only so the declared name is
		// not recorded as a usage of a same-named outer entity.
		if n.Type() == "variable_declarator" {
			// const { helper } = await import("./x") / const m = await import("./x")
			bindJSDynamicImportDeclarator(fe, n, source)
			if nameNode := ingest.ChildByField(n, "name"); nameNode != nil && nameNode.Type() == "identifier" {
				if value := ingest.ChildByField(n, "value"); value != nil {
					if dynPath, ok := jsDynamicImportPath(value, source); ok {
						// Namespace form not handled by object_pattern declarator helper.
						fe.Imports = append(fe.Imports, ingest.ImportDef{
							LocalName:  ingest.NodeText(nameNode, source),
							SourcePath: dynPath,
							StartByte:  nameNode.StartByte(),
							EndByte:    nameNode.EndByte(),
						})
					}
				}
			}
		}
		if value := ingest.ChildByField(n, "value"); value != nil {
			walkJSUsages(fe, value, source, scope)
		}
		if body := ingest.ChildByField(n, "body"); body != nil {
			walkJSUsages(fe, body, source, scope)
		}
		// method_definition / function may have params with default values.
		if params := ingest.ChildByField(n, "parameters"); params != nil {
			walkJSUsages(fe, params, source, scope)
		}
		return
	case "template_string":
		// Static template text is not a reference. Substitutions (`${helper()}`)
		// contain live expressions that must rename with the callee.
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "template_substitution" {
				walkJSUsages(fe, ch, source, scope)
			}
		}
		return
	case "property_identifier", "private_property_identifier",
		"shorthand_property_identifier_pattern",
		"string", "number", "true", "false", "null", "undefined",
		"comment", "this", "super":
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		walkJSUsages(fe, n.Child(i), source, scope)
	}
}

// walkJSBindingPatternDefaults walks default-value expressions inside binding
// patterns (formal parameters, object/array destructuring) without treating
// the bound identifiers themselves as value references.
func walkJSBindingPatternDefaults(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if n == nil || n.IsNull() {
		return
	}
	switch n.Type() {
	case "assignment_pattern", "object_assignment_pattern":
		// `n = helper()` / `{ n = helper() }` — right-hand side is a value ref.
		if right := ingest.ChildByField(n, "right"); right != nil {
			walkJSUsages(fe, right, source, scope)
		}
		if left := ingest.ChildByField(n, "left"); left != nil {
			walkJSBindingPatternDefaults(fe, left, source, scope)
		}
		return
	case "required_parameter", "optional_parameter":
		// TS: (n: T = expr) — default is value field; pattern may nest further.
		if value := ingest.ChildByField(n, "value"); value != nil {
			walkJSUsages(fe, value, source, scope)
		}
		if pattern := ingest.ChildByField(n, "pattern"); pattern != nil {
			walkJSBindingPatternDefaults(fe, pattern, source, scope)
		}
		return
	case "object_pattern", "array_pattern", "formal_parameters",
		"pair_pattern", "rest_pattern":
		for i := uint32(0); i < n.ChildCount(); i++ {
			walkJSBindingPatternDefaults(fe, n.Child(i), source, scope)
		}
		return
	}
}

// emitJSIdentifierUsage records a usage for a function/component reference node.
// Handles both plain identifiers and member expressions (pkg.Component).
// Nested chains like Box.getValue.call emit leaf usages only (Box.getValue with
// Qualifier "Box"), never a qualifier span covering a nested member expression —
// resolveQualifiedUsage would otherwise treat the whole "Box.getValue" span as a
// relation target and rename would replace it with the leaf alone.
//
// Complex nodes (parenthesized_expression, ternary, binary, sequence, …) fall
// through to walkJSUsages so callees like `(helper)()`, `(c ? a : b)()`,
// `(f || helper)()`, and constructors like `new (c ? Helper : Stay)()` still
// record identifier usages.
func emitJSIdentifierUsage(fe *ingest.FileExtract, n *grammar.Node, source []byte, scope string) {
	if n == nil || n.IsNull() {
		return
	}
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
		default:
			// Complex receivers: `new Box(1).n`, `factory().x`, `(cond ? A : B).m`.
			// Walk the object so constructor/callee identifiers inside are usages
			// (plain `new Box(1)` is handled by walkJSUsages new_expression; chaining
			// wraps it in member_expression and used to drop the constructor ref).
			walkJSUsages(fe, obj, source, scope)
		}
	default:
		// call_expression / new_expression / class_heritage function|constructor
		// fields that are not bare identifiers or members (e.g. parenthesized
		// ternary or logical callees).
		walkJSUsages(fe, n, source, scope)
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
		case "template_string":
			// Walk `${…}` substitutions; leave static template chunks alone.
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "template_substitution" {
					walk(ch, false)
				}
			}
			return
		case "string", "number", "true", "false", "null", "undefined",
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
