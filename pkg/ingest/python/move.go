package python

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
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
	lineStart := pythonLeadingIndentStart(source, start)
	declText := string(source[lineStart:end])
	// Nested methods/classes carry class-body indent; normalize to column 0 for insert.
	nested := lineStart < start
	if nested {
		declText = dedentPythonBlock(declText)
	}

	removeStart, removeEnd := pythonTrailingNewlineEnd(source, lineStart, end)

	// Qualified entity names (Class.method) need a class shell when inserted into a
	// new module; stash the outer class in Preamble for InsertDecl.
	preamble := ""
	if className := pythonOuterClass(ingest.ParseReference(entity.Reference).Symbol); className != "" && nested {
		preamble = className
		// Last method (or only body stmt) would leave `class C:` with no body —
		// a SyntaxError. Drop the whole empty class instead.
		if classNode := pythonEnclosingClass(root, declNode); classNode != nil {
			if body := ingest.ChildByField(classNode, "body"); body != nil && pythonBodyEmptyAfterRemove(body, declNode) {
				removeStart, removeEnd = pythonTrailingNewlineEnd(source, pythonLeadingIndentStart(source, classNode.StartByte()), classNode.EndByte())
			}
		}
	}

	return ingest.DeclExtract{
		Preamble:    preamble,
		DeclText:    declText,
		RemoveStart: removeStart,
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
			indent := pythonDetectIndentUnit(decl.DeclText)
			text = "class " + className + ":\n" + indentPythonBlock(decl.DeclText, indent)
			if dstContent == nil {
				return ingest.Edit{
					File:      dstRelPath,
					StartByte: 0,
					EndByte:   0,
					NewText:   text + "\n",
				}
			}
		} else if edit, ok := pythonInsertIntoClassBody(dstRelPath, dstContent, className, decl.DeclText); ok {
			return edit
		} else {
			// Class present but body boundary not found: append a second class block.
			indent := pythonDetectIndentUnit(decl.DeclText)
			text = "class " + className + ":\n" + indentPythonBlock(decl.DeclText, indent)
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

// FinishCrossFileMove adds imports when residual same-file uses remain after a
// move, and when the moved declaration still depends on names left in the
// source module (mirrors JS FinishCrossFileMove).
func (moveDriver) FinishCrossFileMove(rootDir string, result *ingest.Result, src, dst ingest.Reference, decl ingest.DeclExtract) ([]ingest.Edit, error) {
	srcRel := strings.TrimPrefix(src.Path, "./")
	dstRel := strings.TrimPrefix(dst.Path, "./")
	if srcRel == dstRel {
		return nil, nil
	}
	// Nested symbols (Class.method) are not expressible as a simple
	// "from mod import leaf" for residual uses; leave those for method extract.
	if strings.Contains(src.Symbol, ".") {
		return nil, nil
	}
	leaf := src.Symbol
	if leaf == "" {
		return nil, nil
	}

	var edits []ingest.Edit
	srcRef := src.String()

	// 1. Source file still references the moved symbol → import it from dest.
	if pythonFileUsesTargetOutside(result, srcRel, srcRef, decl.RemoveStart, decl.RemoveEnd) {
		if srcContent, err := os.ReadFile(path.Join(rootDir, srcRel)); err == nil {
			stmt := pythonFromImportStmt(srcRel, dstRel, leaf)
			edits = append(edits, pythonImportInsertEdits(srcRel, srcContent, []string{stmt})...)
		}
	}

	// 2. Moved declaration references other same-file entities → import them at dest.
	localDeps := pythonLocalDepsForDecl(result, src, decl)
	if len(localDeps) > 0 {
		var stmts []string
		for _, dep := range localDeps {
			stmts = append(stmts, pythonFromImportStmt(dstRel, srcRel, dep))
		}
		dstPath := path.Join(rootDir, dstRel)
		if dstContent, err := os.ReadFile(dstPath); err == nil {
			edits = append(edits, pythonImportInsertEdits(dstRel, dstContent, stmts)...)
		} else if os.IsNotExist(err) {
			// New destination file: plan insert at byte 0. Applied after the
			// InsertDecl edit (also at 0) so imports end up above the decl.
			block := strings.Join(stmts, "\n") + "\n\n"
			edits = append(edits, ingest.Edit{
				File:      dstRel,
				StartByte: 0,
				EndByte:   0,
				NewText:   block,
			})
		}
	}

	return edits, nil
}

// pythonFileUsesTargetOutside reports whether fileRel has a relation to targetRef
// whose span is outside [removeStart, removeEnd].
func pythonFileUsesTargetOutside(result *ingest.Result, fileRel, targetRef string, removeStart, removeEnd uint32) bool {
	if result == nil {
		return false
	}
	for _, rel := range result.Relations {
		if rel.Target != targetRef {
			continue
		}
		ref := ingest.ParseReference(rel.Reference)
		if strings.TrimPrefix(ref.Path, "./") != fileRel {
			continue
		}
		if rel.StartByte >= removeStart && rel.EndByte <= removeEnd {
			continue
		}
		return true
	}
	return false
}

// pythonLocalDepsForDecl returns top-level symbol names defined in the same file
// as src that the moved declaration body references.
func pythonLocalDepsForDecl(result *ingest.Result, src ingest.Reference, decl ingest.DeclExtract) []string {
	if result == nil {
		return nil
	}
	srcRef := src.String()
	srcPath := src.Path
	localEntities := map[string]bool{}
	for _, ent := range result.Entities {
		ref := ingest.ParseReference(ent.Reference)
		if ref.Path != srcPath || ent.Reference == srcRef {
			continue
		}
		// Only top-level names (no Class.method) for simple imports.
		if ref.Symbol == "" || strings.Contains(ref.Symbol, ".") {
			continue
		}
		localEntities[ref.Symbol] = true
	}
	if len(localEntities) == 0 {
		return nil
	}
	var deps []string
	seen := map[string]bool{}
	for _, rel := range result.Relations {
		ref := ingest.ParseReference(rel.Reference)
		if ref.Path != srcPath {
			continue
		}
		if rel.StartByte < decl.RemoveStart || rel.EndByte > decl.RemoveEnd {
			continue
		}
		targetRef := ingest.ParseReference(rel.Target)
		if targetRef.Path != srcPath {
			continue
		}
		sym := targetRef.Symbol
		if sym == "" || seen[sym] || sym == src.Symbol || strings.Contains(sym, ".") {
			continue
		}
		if localEntities[sym] {
			seen[sym] = true
			deps = append(deps, sym)
		}
	}
	return deps
}

// pythonFromImportStmt builds "from <module> import <symbol>" for a consumer
// file importing a name defined in toFile.
func pythonFromImportStmt(fromFile, toFile, symbol string) string {
	toMod := pythonModuleFromPath(toFile)
	fromDir := pythonDirOf(fromFile)
	modSpec := toMod
	if fromDir != "" {
		if rel := makePythonRelativeSpec(fromDir, toMod); rel != "" {
			modSpec = rel
		}
	}
	return fmt.Sprintf("from %s import %s", modSpec, symbol)
}

// pythonImportInsertEdits inserts missing "from … import …" lines after any
// existing import block (or at the top of the file).
func pythonImportInsertEdits(file string, content []byte, stmts []string) []ingest.Edit {
	if len(stmts) == 0 {
		return nil
	}
	text := string(content)
	var missing []string
	for _, s := range stmts {
		if s == "" || strings.Contains(text, s) {
			continue
		}
		// Also skip if the symbol is already imported from somewhere
		// ("import X" / "from Y import X" / "from Y import X as Z").
		missing = append(missing, s)
	}
	if len(missing) == 0 {
		return nil
	}

	insertPos := pythonImportInsertPos(text)
	block := strings.Join(missing, "\n") + "\n"
	// Blank line after import block when inserting before non-import code.
	if insertPos < len(text) {
		rest := strings.TrimLeft(text[insertPos:], " \t")
		if rest != "" && !strings.HasPrefix(rest, "import ") && !strings.HasPrefix(rest, "from ") && !strings.HasPrefix(rest, "\n") {
			block += "\n"
		}
	}
	return []ingest.Edit{{
		File:      file,
		StartByte: uint32(insertPos),
		EndByte:   uint32(insertPos),
		NewText:   block,
	}}
}

// pythonImportInsertPos returns the byte offset after the last top-level
// import/from-import line (and after a leading module docstring if present).
func pythonImportInsertPos(text string) int {
	offset := 0
	insertPos := 0
	// Skip UTF-8 BOM.
	if strings.HasPrefix(text, "\ufeff") {
		offset = 3
		insertPos = 3
	}
	// Skip leading module docstring.
	if rest := text[offset:]; len(rest) > 0 {
		if q := pythonLeadingDocstringEnd(rest); q > 0 {
			offset += q
			insertPos = offset
		}
	}
	for offset < len(text) {
		// Find end of current line.
		nl := strings.IndexByte(text[offset:], '\n')
		var line string
		var lineEnd int
		if nl < 0 {
			line = text[offset:]
			lineEnd = len(text)
		} else {
			line = text[offset : offset+nl]
			lineEnd = offset + nl + 1
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "from ") {
			insertPos = lineEnd
			offset = lineEnd
			if nl < 0 {
				break
			}
			continue
		}
		// Parenthesized multi-line from-import: keep scanning until ")".
		if strings.HasPrefix(trimmed, "from ") && strings.Contains(line, "(") && !strings.Contains(line, ")") {
			offset = lineEnd
			for offset < len(text) {
				nl2 := strings.IndexByte(text[offset:], '\n')
				if nl2 < 0 {
					insertPos = len(text)
					return insertPos
				}
				line2 := text[offset : offset+nl2]
				offset = offset + nl2 + 1
				insertPos = offset
				if strings.Contains(line2, ")") {
					break
				}
			}
			continue
		}
		break
	}
	return insertPos
}

// pythonLeadingDocstringEnd returns the end offset (in s) of a leading
// module docstring, or 0 if none. Offset is relative to s.
func pythonLeadingDocstringEnd(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	if i >= len(s) {
		return 0
	}
	var quote string
	if strings.HasPrefix(s[i:], `"""`) {
		quote = `"""`
	} else if strings.HasPrefix(s[i:], `'''`) {
		quote = `'''`
	} else {
		return 0
	}
	start := i + len(quote)
	idx := strings.Index(s[start:], quote)
	if idx < 0 {
		return 0
	}
	end := start + idx + len(quote)
	if end < len(s) && s[end] == '\n' {
		end++
	} else if end+1 < len(s) && s[end] == '\r' && s[end+1] == '\n' {
		end += 2
	}
	return end
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
		if abs > 0 && ingest.IsIdentChar(text[abs-1]) {
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

// rewritePythonSymbolImport rewrites a Python import statement from the old
// module to the new module. It uses the Result's alias data to find the
// exact import module strings used in this file to refer to the source module,
// then updates the "from <module> import <names>" statement.
//
// When the import lists multiple names and only some of them are the moved
// symbol, the statement is split so remaining names keep the old module:
//
//	from pkg.mod import helper, stay  →  from pkg.mod import stay
//	                                    from pkg.utils import helper
//
// A sole import of the moved symbol still rewrites only the module path.
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

	// Scan for "from <module> import" statements and rewrite when the module
	// matches a known import module string AND the imported names include the
	// symbol being moved.
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
		if importStart < len(text) {
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
			importedNames := text[importStart:importEnd]
			if movedSymbol == "" {
				// Module-level / unknown symbol: fall back to path rewrite.
				modStart := afterFrom + strings.Index(text[afterFrom:afterFrom+importIdx], modStr)
				modEnd := modStart + len(modStr)
				edits = append(edits, ingest.Edit{
					File:      fileRelPath,
					StartByte: uint32(modStart),
					EndByte:   uint32(modEnd),
					NewText:   buildReplacementModule(modStr, oldMod, newMod),
				})
			} else if items := parsePythonImportItems(importedNames); len(items) > 0 {
				var moved, stayed []pythonImportItem
				for _, it := range items {
					if it.name == movedSymbol {
						moved = append(moved, it)
					} else {
						stayed = append(stayed, it)
					}
				}
				if len(moved) == 0 {
					// Substring match legacy path (e.g. unusual formatting): module rewrite.
					if strings.Contains(importedNames, movedSymbol) {
						modStart := afterFrom + strings.Index(text[afterFrom:afterFrom+importIdx], modStr)
						modEnd := modStart + len(modStr)
						edits = append(edits, ingest.Edit{
							File:      fileRelPath,
							StartByte: uint32(modStart),
							EndByte:   uint32(modEnd),
							NewText:   buildReplacementModule(modStr, oldMod, newMod),
						})
					}
				} else if len(stayed) == 0 {
					// Only the moved symbol(s): rewrite module path in place.
					modStart := afterFrom + strings.Index(text[afterFrom:afterFrom+importIdx], modStr)
					modEnd := modStart + len(modStr)
					edits = append(edits, ingest.Edit{
						File:      fileRelPath,
						StartByte: uint32(modStart),
						EndByte:   uint32(modEnd),
						NewText:   buildReplacementModule(modStr, oldMod, newMod),
					})
				} else {
					// Split: keep remaining names on the old module; add a new
					// import for the moved symbol from the destination module.
					replacement := buildReplacementModule(modStr, oldMod, newMod)
					newText := formatPythonFromImport(modStr, stayed) + "\n" + formatPythonFromImport(replacement, moved)
					edits = append(edits, ingest.Edit{
						File:      fileRelPath,
						StartByte: uint32(fromStart),
						EndByte:   uint32(importEnd),
						NewText:   newText,
					})
				}
			} else if strings.Contains(importedNames, movedSymbol) {
				// Unparsed import list (e.g. star): rewrite module only.
				modStart := afterFrom + strings.Index(text[afterFrom:afterFrom+importIdx], modStr)
				modEnd := modStart + len(modStr)
				edits = append(edits, ingest.Edit{
					File:      fileRelPath,
					StartByte: uint32(modStart),
					EndByte:   uint32(modEnd),
					NewText:   buildReplacementModule(modStr, oldMod, newMod),
				})
			}
		}

		off = importEnd
	}

	return edits
}

// pythonImportItem is one entry in a "from … import a, b as c" name list.
type pythonImportItem struct {
	name  string // imported name (before "as")
	alias string // local alias, empty if none
}

// parsePythonImportItems splits a Python import name list into items.
// Handles parenthesized multi-line lists and "name as alias" forms.
// Returns nil for star imports or unparseable lists.
func parsePythonImportItems(raw string) []pythonImportItem {
	s := strings.TrimSpace(raw)
	if s == "" || s == "*" || strings.HasPrefix(s, "*") {
		return nil
	}
	if strings.HasPrefix(s, "(") {
		end := strings.LastIndex(s, ")")
		if end < 0 {
			return nil
		}
		s = s[1:end]
	}
	// Drop comments and flatten newlines inside parenthesized groups.
	var cleaned strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" || line == "\\" {
			continue
		}
		if cleaned.Len() > 0 {
			cleaned.WriteByte(' ')
		}
		cleaned.WriteString(line)
	}
	s = strings.TrimSpace(cleaned.String())
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var items []pythonImportItem
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := part
		alias := ""
		if i := strings.Index(part, " as "); i >= 0 {
			name = strings.TrimSpace(part[:i])
			alias = strings.TrimSpace(part[i+4:])
		}
		if name == "" || name == "*" || !isPythonIdent(name) {
			return nil
		}
		if alias != "" && !isPythonIdent(alias) {
			return nil
		}
		items = append(items, pythonImportItem{name: name, alias: alias})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func isPythonIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
				return false
			}
			continue
		}
		if !ingest.IsIdentChar(c) {
			return false
		}
	}
	return true
}

// formatPythonFromImport builds a single-line "from <mod> import <names>" statement.
func formatPythonFromImport(mod string, items []pythonImportItem) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if it.alias != "" {
			parts = append(parts, it.name+" as "+it.alias)
		} else {
			parts = append(parts, it.name)
		}
	}
	return "from " + mod + " import " + strings.Join(parts, ", ")
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
			// When the ingest root is the package directory (CLI scopes symbol
			// ops to the source file's parent), both consumerDir and targetDir
			// are "" — still same-directory for a single-dot relative import.
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

// pythonLeadingIndentStart walks left from pos through spaces/tabs on the same line.
func pythonLeadingIndentStart(source []byte, pos uint32) uint32 {
	for pos > 0 && source[pos-1] != '\n' && source[pos-1] != '\r' {
		if source[pos-1] != ' ' && source[pos-1] != '\t' {
			break
		}
		pos--
	}
	return pos
}

// pythonTrailingNewlineEnd extends end through up to two trailing newlines.
func pythonTrailingNewlineEnd(source []byte, start, end uint32) (uint32, uint32) {
	removeEnd := end
	for removeEnd < uint32(len(source)) && (source[removeEnd] == '\n' || source[removeEnd] == '\r') {
		removeEnd++
		if removeEnd-end >= 2 {
			break
		}
	}
	return start, removeEnd
}

// pythonEnclosingClass returns the innermost class_definition whose body
// contains decl (by byte range).
func pythonEnclosingClass(root, decl *grammar.Node) *grammar.Node {
	if root == nil || decl == nil {
		return nil
	}
	var found *grammar.Node
	var walk func(*grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil {
			return
		}
		if n.Type() == "class_definition" {
			if body := ingest.ChildByField(n, "body"); body != nil {
				if decl.StartByte() >= body.StartByte() && decl.EndByte() <= body.EndByte() {
					found = n
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return found
}

// pythonBodyEmptyAfterRemove reports whether body has no named statements left
// once remove is taken out.
func pythonBodyEmptyAfterRemove(body, remove *grammar.Node) bool {
	if body == nil {
		return true
	}
	for i := uint32(0); i < body.NamedChildCount(); i++ {
		c := body.NamedChild(i)
		if c == nil || c.IsNull() {
			continue
		}
		// Exact match or fully covered by the removed span.
		if c.StartByte() == remove.StartByte() && c.EndByte() == remove.EndByte() {
			continue
		}
		if c.StartByte() >= remove.StartByte() && c.EndByte() <= remove.EndByte() {
			continue
		}
		return false
	}
	return true
}

// pythonDetectIndentUnit picks a class-body indent from a column-0 declaration
// block (first nested line's leading whitespace), defaulting to four spaces.
func pythonDetectIndentUnit(block string) string {
	for _, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if indent != "" {
			return indent
		}
	}
	return "    "
}

// pythonLineIndent returns the leading spaces/tabs on the line containing pos.
func pythonLineIndent(source []byte, pos uint32) string {
	lineStart := pos
	for lineStart > 0 && source[lineStart-1] != '\n' && source[lineStart-1] != '\r' {
		lineStart--
	}
	i := lineStart
	for i < uint32(len(source)) && (source[i] == ' ' || source[i] == '\t') {
		i++
	}
	if i <= pos {
		return string(source[lineStart:i])
	}
	return string(source[lineStart:pos])
}

// pythonClassBodyIndent returns the indent used by statements in the named
// class body, or "" if it cannot be determined.
func pythonClassBodyIndent(source []byte, classNode *grammar.Node) string {
	body := ingest.ChildByField(classNode, "body")
	if body == nil {
		return ""
	}
	for i := uint32(0); i < body.NamedChildCount(); i++ {
		c := body.NamedChild(i)
		if c == nil || c.IsNull() {
			continue
		}
		indent := pythonLineIndent(source, c.StartByte())
		if indent != "" {
			return indent
		}
	}
	return ""
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

// pythonInsertIntoClassBody inserts declText into an existing class body,
// matching the destination class's indent style. If the body is only `pass`,
// the pass is replaced.
func pythonInsertIntoClassBody(dstRelPath string, dstContent []byte, className, declText string) (ingest.Edit, bool) {
	pf, err := ingest.ParseSource(dstContent, "memory.py", "python")
	if err != nil {
		return ingest.Edit{}, false
	}
	defer pf.Close()
	classNode := pythonFindClass(pf.Root, pf.Source, className)
	if classNode == nil {
		return ingest.Edit{}, false
	}
	body := ingest.ChildByField(classNode, "body")
	if body == nil {
		return ingest.Edit{}, false
	}

	indent := pythonClassBodyIndent(pf.Source, classNode)
	if indent == "" {
		indent = pythonDetectIndentUnit(declText)
	}
	insertText := indentPythonBlock(declText, indent)
	if !strings.HasSuffix(insertText, "\n") {
		insertText += "\n"
	}

	// Body is only `pass`: replace it with the method (cosmetic cleanup).
	if body.NamedChildCount() == 1 {
		only := body.NamedChild(0)
		if only != nil && only.Type() == "pass_statement" {
			passStart := pythonLeadingIndentStart(pf.Source, only.StartByte())
			return ingest.Edit{
				File:      dstRelPath,
				StartByte: passStart,
				EndByte:   body.EndByte(),
				NewText:   insertText,
			}, true
		}
	}

	at := body.EndByte()
	if at > 0 && at <= uint32(len(dstContent)) && dstContent[at-1] != '\n' {
		insertText = "\n" + insertText
	}
	return ingest.Edit{
		File:      dstRelPath,
		StartByte: at,
		EndByte:   at,
		NewText:   insertText,
	}, true
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

// ExtraRenameEdits rewrites attribute call sites when renaming a method
// (Class.method → Class.new_name). Relation-based rename only covers class-
// qualified calls (Box.get_value) because instance receivers (self/params)
// are not entities. Mirror Go ExtraRenameEdits for Python attributes.
func (moveDriver) ExtraRenameEdits(rootDir string, result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) []ingest.Edit {
	if oldLeaf == "" || oldLeaf == newLeaf || len(sourceRefs) == 0 || rootDir == "" || result == nil {
		return nil
	}
	src := ingest.ParseReference(sourceRefs[0])
	if !strings.Contains(src.Symbol, ".") {
		return nil // only methods / nested symbols
	}

	ourReceivers := map[string]bool{}
	ourTypes := map[string]bool{}
	sourceSet := map[string]bool{}
	for _, s := range sourceRefs {
		sourceSet[s] = true
		ref := ingest.ParseReference(s)
		if recv, ok := pythonMethodReceiver(ref.Symbol); ok {
			ourReceivers[recv] = true
			ourTypes[recv] = true
		}
	}
	if len(ourReceivers) == 0 {
		return nil
	}

	// Inheritance edges (bases) for Protocol/ABC expansion: when renaming an
	// abstract/stub method on Worker, also rename same-leaf methods on subclasses
	// that list Worker as a base (class Box(Worker)). Unlike Java, Python keeps
	// concrete Base/Child override pairs as distinct symbols — do NOT expand
	// parents when renaming a child, and only expand implementors for stub sources.
	baseEdges := pythonBaseEdges(rootDir, result)
	alsoTypes := map[string]bool{}
	if pythonSourcesAreMethodStubs(rootDir, result, sourceSet) {
		for t := range ourTypes {
			for impl, parents := range baseEdges {
				if parents[t] {
					alsoTypes[impl] = true
				}
			}
		}
		for t := range alsoTypes {
			ourReceivers[t] = true
		}
	}

	// Other classes that define the same method leaf — do not rewrite their calls.
	// Hierarchy-expanded types are ours, not foreign.
	foreignReceivers := map[string]bool{}
	for _, ent := range result.Entities {
		if sourceSet[ent.Reference] {
			continue
		}
		ref := ingest.ParseReference(ent.Reference)
		if ingest.SymbolLeaf(ref.Symbol) != oldLeaf {
			continue
		}
		if recv, ok := pythonMethodReceiver(ref.Symbol); ok && !ourReceivers[recv] {
			foreignReceivers[recv] = true
		}
	}

	occupied := ingest.MarkEntityRelationSpans(result, sourceSet)
	markOccupied := func(file string, start, end uint32) {
		file = strings.TrimPrefix(file, "./")
		if occupied[file] == nil {
			occupied[file] = map[[2]uint32]bool{}
		}
		occupied[file][[2]uint32{start, end}] = true
	}

	var edits []ingest.Edit

	// Rename override / related-type method declarations (Protocol/ABC ↔ implementor).
	if len(alsoTypes) > 0 {
		for _, ent := range result.Entities {
			if sourceSet[ent.Reference] {
				continue
			}
			ref := ingest.ParseReference(ent.Reference)
			recv, ok := pythonMethodReceiver(ref.Symbol)
			if !ok || ingest.SymbolLeaf(ref.Symbol) != oldLeaf {
				continue
			}
			if !alsoTypes[recv] && !ourTypes[recv] {
				continue
			}
			file := strings.TrimPrefix(ref.Path, "./")
			if ingest.SpanOccupied(occupied[file], ent.StartByte, ent.EndByte) {
				continue
			}
			edits = append(edits, ingest.Edit{
				File:      file,
				StartByte: ent.StartByte,
				EndByte:   ent.EndByte,
				NewText:   newLeaf,
			})
			markOccupied(file, ent.StartByte, ent.EndByte)
		}
	}

	for _, f := range result.Files {
		if f.Language != "python" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		occ := occupied[rel]
		for _, e := range pythonMethodAttrEdits(rel, content, oldLeaf, newLeaf, ourReceivers, foreignReceivers) {
			if ingest.SpanOccupied(occ, e.StartByte, e.EndByte) {
				continue
			}
			edits = append(edits, e)
		}
	}
	return edits
}

// pythonSourcesAreMethodStubs reports whether every source entity looks like a
// Protocol/ABC abstract method (body is `...` / pass, or @abstractmethod).
// Concrete methods with real bodies must not expand to subclass overrides.
func pythonSourcesAreMethodStubs(rootDir string, result *ingest.Result, sourceSet map[string]bool) bool {
	if result == nil || len(sourceSet) == 0 {
		return false
	}
	saw := false
	for _, ent := range result.Entities {
		if !sourceSet[ent.Reference] {
			continue
		}
		ref := ingest.ParseReference(ent.Reference)
		if !strings.Contains(ref.Symbol, ".") {
			continue
		}
		saw = true
		file := strings.TrimPrefix(ref.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(file)))
		if err != nil {
			return false
		}
		if !pythonMethodEntityIsStub(content, ent.StartByte, ent.EndByte) {
			return false
		}
	}
	return saw
}

// pythonMethodEntityIsStub checks the function_definition around [start,end) name span.
func pythonMethodEntityIsStub(content []byte, start, end uint32) bool {
	pf, err := ingest.ParseSource(content, ".py", "")
	if err != nil {
		return false
	}
	defer pf.Close()
	var fn *grammar.Node
	var find func(*grammar.Node)
	find = func(n *grammar.Node) {
		if n == nil || n.IsNull() || fn != nil {
			return
		}
		if n.Type() == "function_definition" || n.Type() == "async_function_definition" {
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.StartByte() == start && nameN.EndByte() == end {
				fn = n
				return
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			find(n.Child(i))
		}
	}
	find(pf.Root)
	if fn == nil {
		return false
	}
	// @abstractmethod counts as abstract even with a real body.
	var walkDeco func(*grammar.Node) bool
	walkDeco = func(n *grammar.Node) bool {
		if n == nil || n.IsNull() {
			return false
		}
		if n.Type() == "decorated_definition" {
			// decorated_definition children: decorator+ … + function_definition
			hasFn := false
			hasAbs := false
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == fn || (ch.Type() == "function_definition" || ch.Type() == "async_function_definition") {
					nameN := ingest.ChildByField(ch, "name")
					if nameN != nil && nameN.StartByte() == start {
						hasFn = true
					}
				}
				if ch.Type() == "decorator" && strings.Contains(ingest.NodeText(ch, content), "abstractmethod") {
					hasAbs = true
				}
			}
			if hasFn && hasAbs {
				return true
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			if walkDeco(n.Child(i)) {
				return true
			}
		}
		return false
	}
	if walkDeco(pf.Root) {
		return true
	}
	body := ingest.ChildByField(fn, "body")
	if body == nil {
		return false
	}
	// Protocol-style: body is only `...` / `pass` (optional leading docstring).
	// tree-sitter-python often puts bare ellipsis as a direct block child.
	hasStub := false
	for i := uint32(0); i < body.ChildCount(); i++ {
		ch := body.Child(i)
		switch ch.Type() {
		case "ellipsis":
			hasStub = true
		case "pass_statement":
			hasStub = true
		case "comment":
			continue
		case "expression_statement":
			inner := strings.TrimSpace(ingest.NodeText(ch, content))
			if inner == "..." {
				hasStub = true
				continue
			}
			if ch.ChildCount() > 0 {
				c0 := ch.Child(0)
				if c0.Type() == "string" {
					continue // docstring
				}
				if c0.Type() == "ellipsis" {
					hasStub = true
					continue
				}
			}
			return false
		default:
			txt := strings.TrimSpace(ingest.NodeText(ch, content))
			if txt == "" || txt == "..." {
				if txt == "..." {
					hasStub = true
				}
				continue
			}
			return false
		}
	}
	return hasStub
}

// pythonBaseEdges returns map[classSimpleName]set[baseSimpleName] from class
// superclasses lists (class Box(Worker, ABC): …).
func pythonBaseEdges(rootDir string, result *ingest.Result) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if result == nil {
		return out
	}
	for _, f := range result.Files {
		if f.Language != "python" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		content, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		pf, err := ingest.ParseSource(content, rel, "python")
		if err != nil {
			continue
		}
		var walk func(n *grammar.Node)
		walk = func(n *grammar.Node) {
			if n == nil || n.IsNull() {
				return
			}
			if n.Type() == "class_definition" {
				nameN := ingest.ChildByField(n, "name")
				bases := ingest.ChildByField(n, "superclasses")
				if nameN != nil && bases != nil {
					className := ingest.NodeText(nameN, content)
					if out[className] == nil {
						out[className] = map[string]bool{}
					}
					for i := uint32(0); i < bases.ChildCount(); i++ {
						ch := bases.Child(i)
						// argument_list children: identifier, attribute, keyword_argument (metaclass=)
						switch ch.Type() {
						case "identifier":
							out[className][ingest.NodeText(ch, content)] = true
						case "attribute":
							// typing.Protocol / abc.ABC — use attribute leaf
							if attr := ingest.ChildByField(ch, "attribute"); attr != nil {
								out[className][ingest.NodeText(attr, content)] = true
							}
						case "keyword_argument":
							// metaclass=Helper — not a base for method inheritance
							continue
						default:
							// subscript: Protocol[T] etc.
							if ch.Type() == "subscript" {
								if val := ingest.ChildByField(ch, "value"); val != nil {
									if val.Type() == "identifier" {
										out[className][ingest.NodeText(val, content)] = true
									} else if val.Type() == "attribute" {
										if attr := ingest.ChildByField(val, "attribute"); attr != nil {
											out[className][ingest.NodeText(attr, content)] = true
										}
									}
								}
							}
						}
					}
				}
			}
			for i := uint32(0); i < n.ChildCount(); i++ {
				walk(n.Child(i))
			}
		}
		walk(pf.Root)
		pf.Close()
	}
	return out
}

// pythonMethodReceiver returns the class name for "Class.method" symbols.
func pythonMethodReceiver(symbol string) (string, bool) {
	if symbol == "" || !strings.Contains(symbol, ".") {
		return "", false
	}
	// Nested: Outer.Inner.method → treat last parent as receiver class name segment.
	// For Class.method, receiver is Class.
	parts := strings.Split(symbol, ".")
	if len(parts) < 2 {
		return "", false
	}
	// method leaf is last; receiver for Class.method is Class; for A.B.m is A.B
	recv := strings.Join(parts[:len(parts)-1], ".")
	return recv, recv != ""
}

// pythonMethodAttrEdits finds obj.oldLeaf attribute nodes to rewrite, plus
// TypedDict-style string key loads: b["oldLeaf"] / b.get("oldLeaf").
func pythonMethodAttrEdits(fileRel string, content []byte, oldLeaf, newLeaf string, ourReceivers, foreignReceivers map[string]bool) []ingest.Edit {
	pf, err := ingest.ParseSource(content, ".py", "")
	if err != nil {
		return nil
	}
	defer pf.Close()

	// Locals whose static type we can attribute to a class (annotations / Class()).
	typedLocals := pythonTypedLocals(pf.Root, content, ourReceivers)

	var edits []ingest.Edit
	var walk func(n *grammar.Node, enclosingClass string)
	walk = func(n *grammar.Node, enclosingClass string) {
		if n == nil || n.IsNull() {
			return
		}
		classHere := enclosingClass
		if n.Type() == "class_definition" {
			if nameN := ingest.ChildByField(n, "name"); nameN != nil {
				classHere = ingest.NodeText(nameN, content)
			}
		}
		switch n.Type() {
		case "attribute":
			obj := ingest.ChildByField(n, "object")
			attr := ingest.ChildByField(n, "attribute")
			if obj != nil && attr != nil && ingest.NodeText(attr, content) == oldLeaf {
				if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals) {
					edits = append(edits, ingest.Edit{
						File:      fileRel,
						StartByte: attr.StartByte(),
						EndByte:   attr.EndByte(),
						NewText:   newLeaf,
					})
				}
			}
		case "subscript":
			// b["helper"] / b['helper'] — TypedDict key loads.
			obj := ingest.ChildByField(n, "value")
			if obj == nil {
				obj = ingest.ChildByField(n, "object")
			}
			sub := ingest.ChildByField(n, "subscript")
			if obj != nil && sub != nil && sub.Type() == "string" {
				if contentN, text := pythonStringContent(sub, content); contentN != nil && text == oldLeaf {
					if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals) {
						edits = append(edits, ingest.Edit{
							File:      fileRel,
							StartByte: contentN.StartByte(),
							EndByte:   contentN.EndByte(),
							NewText:   newLeaf,
						})
					}
				}
			}
		case "call":
			// b.get("helper", default) — TypedDict .get key.
			fn := ingest.ChildByField(n, "function")
			args := ingest.ChildByField(n, "arguments")
			if fn != nil && fn.Type() == "attribute" && args != nil {
				obj := ingest.ChildByField(fn, "object")
				attr := ingest.ChildByField(fn, "attribute")
				if obj != nil && attr != nil && ingest.NodeText(attr, content) == "get" {
					if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals) {
						if key := pythonFirstStringArg(args); key != nil {
							if contentN, text := pythonStringContent(key, content); contentN != nil && text == oldLeaf {
								edits = append(edits, ingest.Edit{
									File:      fileRel,
									StartByte: contentN.StartByte(),
									EndByte:   contentN.EndByte(),
									NewText:   newLeaf,
								})
							}
						}
					}
				}
			}
			// dataclasses.replace(b, field=…) / replace(b, field=…): keyword names are
			// field sites when the first positional target is one of our receivers.
			if kwEdits := pythonReplaceKeywordEdits(fileRel, n, content, oldLeaf, newLeaf, ourReceivers, foreignReceivers, typedLocals); len(kwEdits) > 0 {
				edits = append(edits, kwEdits...)
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i), classHere)
		}
	}
	walk(pf.Root, "")
	return edits
}

// pythonFirstStringArg returns the first string argument node in an argument_list.
func pythonFirstStringArg(args *grammar.Node) *grammar.Node {
	if args == nil {
		return nil
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() == "string" {
			return ch
		}
	}
	return nil
}

// pythonReplaceKeywordEdits rewrites field keywords on replace(obj, oldLeaf=…).
// Only when obj is a typed local / class name of ourReceivers (fail closed).
func pythonReplaceKeywordEdits(fileRel string, call *grammar.Node, content []byte, oldLeaf, newLeaf string, ourReceivers, foreignReceivers, typedLocals map[string]bool) []ingest.Edit {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return nil
	}
	// replace(...) or dataclasses.replace(...) / dc.replace(...)
	leaf := pythonSimpleCalleeName(fn, content)
	if leaf != "replace" {
		return nil
	}
	args := ingest.ChildByField(call, "arguments")
	if args == nil {
		return nil
	}
	// First positional argument is the dataclass instance.
	var target *grammar.Node
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment", "keyword_argument":
			continue
		default:
			target = ch
		}
		if target != nil {
			break
		}
	}
	if target == nil || target.Type() != "identifier" {
		return nil
	}
	name := ingest.NodeText(target, content)
	if !typedLocals[name] && !ourReceivers[name] {
		return nil
	}
	if foreignReceivers[name] {
		return nil
	}
	var edits []ingest.Edit
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() != "keyword_argument" {
			continue
		}
		nameN := ingest.ChildByField(ch, "name")
		if nameN == nil || ingest.NodeText(nameN, content) != oldLeaf {
			continue
		}
		edits = append(edits, ingest.Edit{
			File:      fileRel,
			StartByte: nameN.StartByte(),
			EndByte:   nameN.EndByte(),
			NewText:   newLeaf,
		})
	}
	return edits
}

// pythonShouldRenameAttr decides whether obj.oldLeaf is a call on one of our receivers.
func pythonShouldRenameAttr(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool) bool {
	if obj == nil {
		return false
	}
	// Parenthesized expressions share logic with the inner receiver.
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil && obj.ChildCount() > 0 {
			for i := uint32(0); i < obj.ChildCount(); i++ {
				ch := obj.Child(i)
				if ch.Type() == "(" || ch.Type() == ")" {
					continue
				}
				inner = ch
				break
			}
		}
		if inner == nil {
			break
		}
		obj = inner
	}
	// super().method() targets a parent implementation, not the enclosing class's
	// own override. When renaming Base.m, rewrite super().m in Child even if Child
	// also defines m; when renaming Child.m, leave super().m alone.
	if pythonIsSuperCall(obj, content) {
		if enclosingClass != "" && ourReceivers[enclosingClass] {
			return false
		}
		return true
	}
	// Box().method / Box(1).method — temporary constructor receiver (mirror Java new).
	if obj.Type() == "call" {
		fn := ingest.ChildByField(obj, "function")
		if fn != nil && fn.Type() == "identifier" {
			name := ingest.NodeText(fn, content)
			if ourReceivers[name] {
				return true
			}
			if foreignReceivers[name] {
				return false
			}
			// make().method — unknown return type: only when leaf is unique.
			return len(foreignReceivers) == 0
		}
		// Nested call / attribute callee: fail closed unless leaf is unique.
		return len(foreignReceivers) == 0
	}
	// Simple identifiers: self.x, cls.x, box.x, Box.x
	if obj.Type() == "identifier" {
		name := ingest.NodeText(obj, content)
		switch name {
		case "self", "cls":
			// Inside our class body: rewrite. If foreign classes share the leaf, only
			// rewrite when enclosing class is one of ours.
			if enclosingClass == "" {
				return len(foreignReceivers) == 0
			}
			if ourReceivers[enclosingClass] {
				return true
			}
			if foreignReceivers[enclosingClass] {
				return false
			}
			// Nested / unknown class: fail closed if collisions exist.
			return len(foreignReceivers) == 0
		}
		// Class-qualified: Box.method
		if ourReceivers[name] {
			return true
		}
		if foreignReceivers[name] {
			return false
		}
		// Local with known type matching our receiver.
		if typedLocals[name] {
			return true
		}
		// No foreign same-leaf methods: rewrite all simple attribute loads of the leaf
		// (unique method name in the project graph).
		return len(foreignReceivers) == 0
	}
	// Complex receivers: xs[0].m, obj.box.m, (a if c else b).m — only when the
	// method leaf is unique project-wide (no static type on the operand).
	switch obj.Type() {
	case "subscript", "attribute", "conditional_expression",
		"binary_operator", "boolean_operator", "await":
		return len(foreignReceivers) == 0
	}
	return false
}

// pythonIsSuperCall reports whether n is a call to super (super() / super(C, self)).
func pythonIsSuperCall(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "call" {
		return false
	}
	fn := ingest.ChildByField(n, "function")
	return fn != nil && fn.Type() == "identifier" && ingest.NodeText(fn, content) == "super"
}

// pythonTypedLocals maps local names that are annotated or assigned as ourReceivers.
// Covers: `b: Box`, `b = Box()`, `b: Box = ...`.
func pythonTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) map[string]bool {
	out := map[string]bool{}
	if root == nil || len(ourReceivers) == 0 {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "parameters", "lambda_parameters":
			// function params: (self, b: Box) / (b: "Box")
			for i := uint32(0); i < n.ChildCount(); i++ {
				p := n.Child(i)
				switch p.Type() {
				case "typed_parameter", "typed_default_parameter":
					nameN := ingest.ChildByField(p, "name")
					typeN := ingest.ChildByField(p, "type")
					if nameN == nil {
						// grammar may put identifier as first named child
						nameN = ingest.ChildByType(p, "identifier")
					}
					if nameN != nil && typeN != nil {
						if tn := pythonTypeName(typeN, content); ourReceivers[tn] {
							out[ingest.NodeText(nameN, content)] = true
						}
					}
				}
			}
		case "assignment":
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			typeN := ingest.ChildByField(n, "type")
			if left != nil && left.Type() == "identifier" {
				lname := ingest.NodeText(left, content)
				if typeN != nil {
					if tn := pythonTypeName(typeN, content); ourReceivers[tn] {
						out[lname] = true
					}
				}
				if right != nil && right.Type() == "call" {
					fn := ingest.ChildByField(right, "function")
					if fn != nil && fn.Type() == "identifier" {
						if ourReceivers[ingest.NodeText(fn, content)] {
							out[lname] = true
						}
					}
				}
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out
}

// pythonTypeName extracts a simple class name from a type annotation node.
func pythonTypeName(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	// type may be type node wrapping identifier / attribute / string
	if typeN.Type() == "type" && typeN.ChildCount() > 0 {
		typeN = typeN.Child(0)
	}
	switch typeN.Type() {
	case "identifier":
		return ingest.NodeText(typeN, content)
	case "string":
		// from __future__ annotations / quoted: "Box"
		s := ingest.NodeText(typeN, content)
		return strings.Trim(s, `"'`)
	case "attribute":
		// pkg.Box — use leaf
		if attr := ingest.ChildByField(typeN, "attribute"); attr != nil {
			return ingest.NodeText(attr, content)
		}
	}
	return ""
}
