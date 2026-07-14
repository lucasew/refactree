package python

import (
	"fmt"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

func init() {
	ingest.RegisterMoveDriver("python", moveDriver{})
}

type moveDriver struct{}

func (moveDriver) Language() string { return "python" }

func (moveDriver) ExtractDecl(filePath string, entity ingest.Entity) (ingest.DeclExtract, error) {
	pf, err := ingest.ParseSourceFile(filePath, "")
	if err != nil {
		return ingest.DeclExtract{}, err
	}
	defer pf.Close()
	source, root := pf.Source, pf.Root

	declNode := findPythonDecl(root, entity.StartByte)
	if declNode == nil {
		return ingest.DeclExtract{}, fmt.Errorf("declaration not found in %s", filePath)
	}

	start := declNode.StartByte()
	end := declNode.EndByte()
	// Include leading indentation on the first line so nested methods remove cleanly.
	lineStart := start
	for lineStart > 0 && source[lineStart-1] != '\n' && source[lineStart-1] != '\r' {
		if source[lineStart-1] != ' ' && source[lineStart-1] != '\t' {
			break
		}
		lineStart--
	}
	declText := string(source[lineStart:end])
	// Nested methods/classes carry class-body indent; normalize to column 0 for insert.
	if lineStart < start {
		declText = dedentPythonBlock(declText)
	}

	// Remove up to two trailing newlines.
	removeEnd := end
	for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
		removeEnd++
		if removeEnd-end >= 2 {
			break
		}
	}

	// Qualified entity names (Class.method) need a class shell when inserted into a
	// new module; stash the outer class in Preamble for InsertDecl.
	preamble := ""
	if className := pythonOuterClass(ingest.ParseReference(entity.Reference).Symbol); className != "" && lineStart < start {
		preamble = className
	}

	return ingest.DeclExtract{
		Preamble:    preamble,
		DeclText:    declText,
		RemoveStart: lineStart,
		RemoveEnd:   removeEnd,
	}, nil
}

func (moveDriver) InsertDecl(dstRelPath string, dstContent []byte, decl ingest.DeclExtract) ingest.Edit {
	text := decl.DeclText
	insertAt := uint32(0)

	// Nested method/class: re-home under its outer class when the destination is
	// a new file or does not already define that class.
	if decl.Preamble != "" {
		className := decl.Preamble
		if dstContent == nil || !pythonSourceHasClass(dstContent, className) {
			text = "class " + className + ":\n" + indentPythonBlock(decl.DeclText, "    ")
			if dstContent == nil {
				return ingest.Edit{
					File:      dstRelPath,
					StartByte: 0,
					EndByte:   0,
					NewText:   text + "\n",
				}
			}
		} else if at, ok := pythonClassBodyInsertAt(dstContent, className); ok {
			// Insert indented method into the existing class body.
			insertAt = at
			insertText := indentPythonBlock(decl.DeclText, "    ")
			if at > 0 && dstContent[at-1] != '\n' {
				insertText = "\n" + insertText
			}
			if !strings.HasSuffix(insertText, "\n") {
				insertText += "\n"
			}
			return ingest.Edit{
				File:      dstRelPath,
				StartByte: insertAt,
				EndByte:   insertAt,
				NewText:   insertText,
			}
		} else {
			// Class present but body boundary not found: append a second class block.
			text = "class " + className + ":\n" + indentPythonBlock(decl.DeclText, "    ")
		}
	}

	insertText := ""
	if dstContent != nil {
		insertAt = uint32(len(dstContent))
		if len(dstContent) > 0 && dstContent[len(dstContent)-1] != '\n' {
			insertText += "\n"
		}
		if len(dstContent) > 0 {
			insertText += "\n"
		}
		insertText += text + "\n"
	} else {
		insertText = text + "\n"
	}

	return ingest.Edit{
		File:      dstRelPath,
		StartByte: insertAt,
		EndByte:   insertAt,
		NewText:   insertText,
	}
}

func (moveDriver) RewriteImports(fileRelPath string, content []byte, result *ingest.Result, oldRef, newRef ingest.Reference) []ingest.Edit {
	oldPath := strings.TrimPrefix(oldRef.Path, "./")
	newPath := strings.TrimPrefix(newRef.Path, "./")

	// For symbol-level moves, find the import statement in this consumer file
	// that references the source module and rewrite it to point to the
	// destination module.
	if oldRef.Symbol != "" {
		return rewritePythonSymbolImport(fileRelPath, content, result, oldRef, oldPath, newPath)
	}

	// Module file renames (*.py → *.py): Python imports use the module stem
	// (namedutils), never the filename (namedutils.py). Rewriting the file
	// basename would no-op; rewrite the importable stem instead so
	// `from pkg.namedutils import X`, `from .namedutils import X`, and
	// `import namedutils` all track the move.
	if strings.HasSuffix(oldPath, ".py") && strings.HasSuffix(newPath, ".py") {
		return rewritePythonModuleFile(fileRelPath, content, oldPath, newPath)
	}

	// For package directory moves, use word-boundary-aware replacement to avoid
	// corrupting identifiers that happen to contain the package name as substring.
	oldDir := oldPath
	newDir := newPath
	if oldDir == "" || newDir == "" || oldDir == newDir {
		return nil
	}
	oldBase := ingest.LastPathComponent(oldDir)
	newBase := ingest.LastPathComponent(newDir)
	if oldBase == newBase {
		return nil
	}
	return ingest.FindAllWholeWordOccurrences(fileRelPath, content, oldBase, newBase)
}

// rewritePythonModuleFile rewrites import module specs after a .py file is
// relocated. It scans import statements only (not bare whole-word stems) so:
//   - package path + stem both update (pkg.a.mod → pkg.b.mod_fuzz)
//   - same-leaf unrelated modules stay put (pkg.a.utils vs pkg.b.utils)
//   - relative imports (from .mod / from ..a.mod) resolve and track the move
func rewritePythonModuleFile(fileRelPath string, content []byte, oldPath, newPath string) []ingest.Edit {
	oldMod := pythonModuleFromPath(oldPath)
	newMod := pythonModuleFromPath(newPath)
	if oldMod == "" || newMod == "" || oldMod == newMod {
		return nil
	}
	consumerDir := pythonDirOf(strings.TrimPrefix(fileRelPath, "./"))
	text := string(content)
	var edits []ingest.Edit

	// "from <module> import ..."
	for off := 0; off < len(text); {
		idx := strings.Index(text[off:], "from ")
		if idx < 0 {
			break
		}
		afterFrom := off + idx + 5
		importIdx := strings.Index(text[afterFrom:], " import")
		if importIdx < 0 {
			off = afterFrom
			continue
		}
		modRaw := text[afterFrom : afterFrom+importIdx]
		modStr := strings.TrimSpace(modRaw)
		if modStr == "" || modStr == "..." {
			off = afterFrom + importIdx + 7
			continue
		}
		if !pythonImportMatchesModule(modStr, oldMod, consumerDir) {
			off = afterFrom + importIdx + 7
			continue
		}
		modStart := afterFrom + strings.Index(modRaw, modStr)
		modEnd := modStart + len(modStr)
		edits = append(edits, ingest.Edit{
			File:      fileRelPath,
			StartByte: uint32(modStart),
			EndByte:   uint32(modEnd),
			NewText:   pythonReplacementModuleSpec(modStr, oldMod, newMod, consumerDir),
		})
		off = afterFrom + importIdx + 7
	}

	// "import <module>" / "import <module> as alias" (possibly comma-separated)
	for off := 0; off < len(text); {
		idx := strings.Index(text[off:], "import ")
		if idx < 0 {
			break
		}
		// Skip the "import" that is part of "from X import".
		abs := off + idx
		if abs >= 5 && text[abs-5:abs] == "from " {
			off = abs + 7
			continue
		}
		// Also skip if preceded by non-boundary (e.g. "fromx import" is fine; word start).
		if abs > 0 && isPythonIdentChar(text[abs-1]) {
			off = abs + 7
			continue
		}
		start := abs + 7
		// End of statement: newline (not inside parens) or comment.
		end := start
		for end < len(text) && text[end] != '\n' && text[end] != '#' {
			end++
		}
		segment := text[start:end]
		// Split on commas for "import a, b as c".
		partOff := start
		for _, part := range strings.Split(segment, ",") {
			raw := part
			// Strip " as alias".
			asIdx := strings.Index(raw, " as ")
			modPart := raw
			if asIdx >= 0 {
				modPart = raw[:asIdx]
			}
			modStr := strings.TrimSpace(modPart)
			if modStr == "" {
				partOff += len(part) + 1
				continue
			}
			if pythonImportMatchesModule(modStr, oldMod, consumerDir) {
				// Locate modStr within this part relative to partOff.
				inner := strings.Index(raw, modStr)
				if inner >= 0 {
					modStart := partOff + inner
					edits = append(edits, ingest.Edit{
						File:      fileRelPath,
						StartByte: uint32(modStart),
						EndByte:   uint32(modStart + len(modStr)),
						NewText:   pythonReplacementModuleSpec(modStr, oldMod, newMod, consumerDir),
					})
				}
			}
			partOff += len(part) + 1 // +1 for comma
		}
		off = end
	}

	return edits
}

// pythonImportMatchesModule reports whether an import module string refers to oldMod
// (exact, external-prefix suffix, or relative resolved against the consumer package).
func pythonImportMatchesModule(modStr, oldMod, consumerDir string) bool {
	if modStr == oldMod {
		return true
	}
	if strings.HasSuffix(modStr, "."+oldMod) {
		// External package prefix: boltons.namedutils when oldMod is namedutils.
		// Require a boundary so namedutils does not match foo.xnamedutils.
		prefixLen := len(modStr) - len(oldMod)
		if prefixLen > 0 && modStr[prefixLen-1] == '.' {
			return true
		}
	}
	if strings.HasPrefix(modStr, ".") {
		return resolvePythonRelative(consumerDir, modStr) == oldMod
	}
	return false
}

// pythonReplacementModuleSpec builds the replacement import module string.
func pythonReplacementModuleSpec(modStr, oldMod, newMod, consumerDir string) string {
	if strings.HasPrefix(modStr, ".") {
		if rel := makePythonRelativeSpec(consumerDir, newMod); rel != "" {
			return rel
		}
		return newMod
	}
	return buildReplacementModule(modStr, oldMod, newMod)
}

// resolvePythonRelative maps a relative import module (leading dots) to an absolute
// dotted module path from the consumer's directory.
func resolvePythonRelative(consumerDir, relSpec string) string {
	if !strings.HasPrefix(relSpec, ".") {
		return ""
	}
	dots := 0
	for dots < len(relSpec) && relSpec[dots] == '.' {
		dots++
	}
	rest := relSpec[dots:]
	// dots=1 → current package; dots=2 → parent; …
	up := dots - 1
	dir := consumerDir
	for i := 0; i < up; i++ {
		if dir == "" {
			return ""
		}
		if j := strings.LastIndex(dir, "/"); j >= 0 {
			dir = dir[:j]
		} else {
			dir = ""
		}
	}
	base := strings.ReplaceAll(dir, "/", ".")
	if rest == "" {
		return base
	}
	if base == "" {
		return rest
	}
	return base + "." + rest
}

// makePythonRelativeSpec builds a relative import module string from consumerDir
// to absMod (dotted). Returns "" if a relative form cannot be formed cleanly.
func makePythonRelativeSpec(consumerDir, absMod string) string {
	if absMod == "" {
		return ""
	}
	consumerPkg := strings.ReplaceAll(consumerDir, "/", ".")
	if consumerPkg == absMod {
		return "."
	}
	if consumerPkg != "" && strings.HasPrefix(absMod, consumerPkg+".") {
		return "." + strings.TrimPrefix(absMod, consumerPkg+".")
	}
	// Walk up from the consumer package until absMod is under the parent.
	parts := []string{}
	if consumerPkg != "" {
		parts = strings.Split(consumerPkg, ".")
	}
	for up := 1; up <= len(parts); up++ {
		parentParts := parts[:len(parts)-up]
		parent := strings.Join(parentParts, ".")
		var rest string
		if parent == "" {
			rest = absMod
		} else if absMod == parent {
			rest = ""
		} else if strings.HasPrefix(absMod, parent+".") {
			rest = strings.TrimPrefix(absMod, parent+".")
		} else {
			continue
		}
		// up parents ⇒ (up+1) leading dots in Python relative import syntax.
		return strings.Repeat(".", up+1) + rest
	}
	return ""
}

func pythonDirOf(p string) string {
	p = strings.TrimPrefix(p, "./")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return ""
}

func isPythonIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// rewritePythonSymbolImport rewrites a Python import statement from the old
// module to the new module. It uses the Result's alias data to find the
// exact import module strings used in this file to refer to the source module,
// then replaces the module path in the "from <module> import" statement.
func rewritePythonSymbolImport(fileRelPath string, content []byte, result *ingest.Result, oldRef ingest.Reference, oldPath, newPath string) []ingest.Edit {
	oldMod := pythonModuleFromPath(oldPath)
	newMod := pythonModuleFromPath(newPath)
	if oldMod == "" || newMod == "" || oldMod == newMod {
		return nil
	}

	// Build the set of import module strings used in this consumer file that
	// actually point at the source module. We find these by looking at the
	// alias targets and finding the corresponding import source specs.
	importModules := findImportModulesForFile(fileRelPath, result, oldPath)

	text := string(content)
	var edits []ingest.Edit

	movedSymbol := oldRef.Symbol

	// Scan for "from <module> import" statements and replace the module path
	// only when it matches one of the known import module strings AND the
	// imported symbols include the one being moved.
	for off := 0; off < len(text); {
		idx := strings.Index(text[off:], "from ")
		if idx < 0 {
			break
		}
		fromStart := off + idx
		afterFrom := fromStart + 5

		importIdx := strings.Index(text[afterFrom:], " import")
		if importIdx < 0 {
			off = afterFrom
			continue
		}

		modRaw := text[afterFrom : afterFrom+importIdx]
		modStr := strings.TrimSpace(modRaw)

		// Find end of the import statement (end of line or multi-line paren group).
		importStart := afterFrom + importIdx + 7 // skip " import"
		importEnd := importStart
		if importStart < len(text) && text[importStart] == ' ' {
			importEnd = importStart
			// Check for parenthesized multi-line imports.
			rest := strings.TrimLeft(text[importStart:], " \t")
			if len(rest) > 0 && rest[0] == '(' {
				parenClose := strings.Index(text[importStart:], ")")
				if parenClose >= 0 {
					importEnd = importStart + parenClose + 1
				}
			} else {
				// Single-line: find end of line.
				nlIdx := strings.IndexByte(text[importStart:], '\n')
				if nlIdx >= 0 {
					importEnd = importStart + nlIdx
				} else {
					importEnd = len(text)
				}
			}
		}

		if importModules[modStr] {
			// Check if the moved symbol appears in the imported names.
			importedNames := text[importStart:importEnd]
			if movedSymbol == "" || strings.Contains(importedNames, movedSymbol) {
				modStart := afterFrom + strings.Index(text[afterFrom:afterFrom+importIdx], modStr)
				modEnd := modStart + len(modStr)

				replacement := buildReplacementModule(modStr, oldMod, newMod)

				edits = append(edits, ingest.Edit{
					File:      fileRelPath,
					StartByte: uint32(modStart),
					EndByte:   uint32(modEnd),
					NewText:   replacement,
				})
			}
		}

		off = importEnd
	}

	return edits
}

// findImportModulesForFile finds the Python import module strings used in
// a consumer file that reference the given source path. It does this by
// looking at the aliases in the ingest result and extracting the dotted
// module path from the target reference.
func findImportModulesForFile(consumerFile string, result *ingest.Result, oldPath string) map[string]bool {
	modules := map[string]bool{}
	consumerRel := strings.TrimPrefix(consumerFile, "./")
	oldMod := pythonModuleFromPath(oldPath)
	if oldMod == "" {
		return modules
	}

	// Also compute what relative import forms would look like for this consumer.
	consumerDir := ""
	if i := strings.LastIndex(consumerRel, "/"); i >= 0 {
		consumerDir = consumerRel[:i]
	}

	for _, alias := range result.Aliases {
		ref := ingest.ParseReference(alias.Reference)
		aliasFile := strings.TrimPrefix(ref.Path, "./")
		if aliasFile != consumerRel {
			continue
		}
		targetRef := ingest.ParseReference(alias.Target)
		if targetRef.Symbol == "" {
			continue
		}

		// Convert the target to a dotted module path.
		var targetMod string
		if targetRef.Provider == "path" {
			targetMod = pythonModuleFromPath(strings.TrimPrefix(targetRef.Path, "./"))
		} else {
			targetMod = targetRef.Path
		}

		if targetMod == oldMod || strings.HasSuffix(targetMod, "."+oldMod) {
			modules[targetMod] = true

			// Also add the relative import form (e.g. ".helpers" for same-dir).
			if consumerDir != "" {
				targetDir := ""
				if targetRef.Provider == "path" {
					tp := strings.TrimPrefix(targetRef.Path, "./")
					if i := strings.LastIndex(tp, "/"); i >= 0 {
						targetDir = tp[:i]
					}
				}
				if targetDir == consumerDir {
					stem := pythonFileStem(strings.TrimPrefix(targetRef.Path, "./"))
					if stem != "" {
						modules["."+stem] = true
					}
				}
			}
		}
	}

	return modules
}

// buildReplacementModule constructs the replacement module string.
// If the original import module has a prefix beyond what the ingest root sees
// (e.g. "fastapi.utils" when oldMod is "utils"), preserve that prefix.
func buildReplacementModule(importMod, oldMod, newMod string) string {
	if importMod == oldMod {
		return newMod
	}
	if strings.HasSuffix(importMod, "."+oldMod) {
		prefix := importMod[:len(importMod)-len(oldMod)]
		return prefix + newMod
	}
	// Relative import: replace stem
	if strings.HasPrefix(importMod, ".") {
		oldStem := ingest.LastPathComponent(strings.ReplaceAll(oldMod, ".", "/"))
		newStem := ingest.LastPathComponent(strings.ReplaceAll(newMod, ".", "/"))
		if strings.HasSuffix(importMod, oldStem) {
			return importMod[:len(importMod)-len(oldStem)] + newStem
		}
	}
	return newMod
}

// pythonPathWithoutSuffix strips .py and package __init__ suffixes from a file path.
func pythonPathWithoutSuffix(p string) string {
	p = strings.TrimSuffix(p, ".py")
	return strings.TrimSuffix(p, "/__init__")
}

// pythonModuleFromPath converts a file path like "pkga/helpers.py" to
// a Python module spec like "pkga.helpers".
func pythonModuleFromPath(p string) string {
	return strings.ReplaceAll(pythonPathWithoutSuffix(p), "/", ".")
}

// pythonFileStem returns the bare module name from a path (e.g. "helpers" from "pkg/helpers.py").
func pythonFileStem(p string) string {
	return ingest.LastPathComponent(pythonPathWithoutSuffix(p))
}

// findPythonDecl returns the declaration node whose name starts at nameStart.
// It walks nested class/function bodies so methods (Class.method) extract for
// cross-file moves — top-level-only search left those as "declaration not found".
func findPythonDecl(root *grammar.Node, nameStart uint32) *grammar.Node {
	return findPythonDeclNode(root, nameStart)
}

func findPythonDeclNode(n *grammar.Node, nameStart uint32) *grammar.Node {
	if n == nil {
		return nil
	}
	switch n.Type() {
	case "function_definition", "class_definition":
		if name := ingest.ChildByField(n, "name"); name != nil && name.StartByte() == nameStart {
			return n
		}
	case "assignment", "augmented_assignment":
		if left := ingest.ChildByField(n, "left"); left != nil && left.StartByte() == nameStart {
			return n
		}
	case "expression_statement":
		if n.ChildCount() > 0 {
			inner := n.Child(0)
			if inner.Type() == "assignment" || inner.Type() == "augmented_assignment" {
				if left := ingest.ChildByField(inner, "left"); left != nil && left.StartByte() == nameStart {
					return n
				}
			}
		}
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		if found := findPythonDeclNode(n.Child(i), nameStart); found != nil {
			return found
		}
	}
	return nil
}

// pythonOuterClass returns the outer class name for a qualified entity like
// "FunctionBuilder.remove_arg", or "" for module-level symbols.
func pythonOuterClass(entityName string) string {
	entityName = strings.TrimSpace(entityName)
	if i := strings.LastIndex(entityName, "."); i > 0 {
		return entityName[:i]
	}
	return ""
}

// dedentPythonBlock strips the common leading indent from every line.
func dedentPythonBlock(block string) string {
	lines := strings.Split(block, "\n")
	prefix := ""
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if prefix == "" || len(indent) < len(prefix) {
			prefix = indent
		}
	}
	if prefix == "" {
		return block
	}
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(line, prefix)
	}
	return strings.Join(lines, "\n")
}

// indentPythonBlock prefixes every non-empty line with indent.
func indentPythonBlock(block, indent string) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

// pythonSourceHasClass reports whether content defines a class with the given name.
func pythonSourceHasClass(content []byte, className string) bool {
	pf, err := ingest.ParseSource(content, "memory.py", "python")
	if err != nil {
		// Fallback: naive scan.
		return strings.Contains(string(content), "class "+className+":") ||
			strings.Contains(string(content), "class "+className+"(")
	}
	defer pf.Close()
	return pythonFindClass(pf.Root, pf.Source, className) != nil
}

// pythonClassBodyInsertAt returns the byte offset just before the end of the
// named class's body (suitable for inserting a nested method).
func pythonClassBodyInsertAt(content []byte, className string) (uint32, bool) {
	pf, err := ingest.ParseSource(content, "memory.py", "python")
	if err != nil {
		return 0, false
	}
	defer pf.Close()
	classNode := pythonFindClass(pf.Root, pf.Source, className)
	if classNode == nil {
		return 0, false
	}
	body := ingest.ChildByField(classNode, "body")
	if body == nil {
		return 0, false
	}
	return body.EndByte(), true
}

func pythonFindClass(n *grammar.Node, source []byte, className string) *grammar.Node {
	if n == nil {
		return nil
	}
	if n.Type() == "class_definition" {
		if name := ingest.ChildByField(n, "name"); name != nil && ingest.NodeText(name, source) == className {
			return n
		}
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		if found := pythonFindClass(n.Child(i), source, className); found != nil {
			return found
		}
	}
	return nil
}
