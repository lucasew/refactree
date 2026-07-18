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
	// fieldOf maps "local.field" → field type leaf for class/dataclass field access
	// (box.a.run() / xa = box.a under foreign same-leaf methods).
	typedLocals, fieldOf := pythonTypedLocals(pf.Root, content, ourReceivers)

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
				if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf) {
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
					if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf) {
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
					if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf) {
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

// pythonRenameByTypeMaps: our → rename; foreign → skip; typedLocals → rename; else unique-leaf only.
func pythonRenameByTypeMaps(name string, ourReceivers, foreignReceivers, typedLocals map[string]bool) bool {
	if ourReceivers[name] {
		return true
	}
	if foreignReceivers[name] {
		return false
	}
	if typedLocals != nil && typedLocals[name] {
		return true
	}
	return len(foreignReceivers) == 0
}

// pythonShouldRenameAttr decides whether obj.oldLeaf is a call on one of our receivers.
// fieldOf maps "local.field" → field type leaf for dataclass/class field access
// (box.a.run() under foreign same-leaf methods).
func pythonShouldRenameAttr(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, fieldOf map[string]string) bool {
	if obj == nil {
		return false
	}
	for obj != nil && !obj.IsNull() && obj.Type() == "parenthesized_expression" {
		inner := ingest.ChildByField(obj, "expression")
		if inner == nil {
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
	// super().method(): rewrite when renaming Base.m in Child; leave alone when renaming Child.m.
	if pythonIsSuperCall(obj, content) {
		return enclosingClass == "" || !ourReceivers[enclosingClass]
	}
	// Box().method / make().method / nested call — ctor name via maps, else unique-leaf.
	if obj.Type() == "call" {
		name := ""
		if fn := ingest.ChildByField(obj, "function"); fn != nil && fn.Type() == "identifier" {
			name = ingest.NodeText(fn, content)
		}
		return pythonRenameByTypeMaps(name, ourReceivers, foreignReceivers, nil)
	}
	if obj.Type() == "identifier" {
		name := ingest.NodeText(obj, content)
		if name == "self" || name == "cls" {
			return pythonRenameByTypeMaps(enclosingClass, ourReceivers, foreignReceivers, nil)
		}
		return pythonRenameByTypeMaps(name, ourReceivers, foreignReceivers, typedLocals)
	}
	// box.a.run() — dataclass/class field access when box is a typed local.
	if obj.Type() == "attribute" {
		if ft := pythonFieldAccessType(obj, content, fieldOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		return len(foreignReceivers) == 0
	}
	// Complex receivers without static type: unique-leaf only.
	switch obj.Type() {
	case "subscript", "conditional_expression",
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
// Covers: `b: Box`, `b = Box()`, `b: Box = ...`, Optional/Union/`|` annotations
// (`b: Optional[Box]`, `b: Box | None`, `b: Union[Box, None]`), `b = cast(Box, x)`,
// `a = next(iter(items))` / `a = next(items)` / `a = next(x for x in items)`
// (element type of the iterable arg / identity genexp),
// `a = min(items)` / `a = max(items)` / `a = min(items, key=...)` (same element type),
// `pair = min(pairs)` / `a, b = max(list(zip(...)))` when pairs is a pair-iter
// (pairSlots + shared elemOf; same path as next(pairs) / pairs.pop()),
// `a = choice(items)` / `a = random.choice(items)` (same element type),
// `pair = choice(pairs)` / `a, b = random.choice(list(zip(...)))` when pairs is a
// pair-iter (pairSlots + shared elemOf; same path as min(pairs)),
// `a = heappop(items)` / `a = heapq.heappop(items)` (heap element type; same as next),
// `pair = heappop(pairs)` / `a, b = heapq.heappop(list(zip(...)))` when pairs is a
// pair-iter (pairSlots + shared elemOf; same path as min(pairs)),
// `a = heappushpop(items, x)` / `a = heapreplace(items, x)` / heapq.* (same heap elem),
// `pair = heappushpop(pairs, x)` / `pair = heapreplace(pairs, x)` (pair-iter same),
// `a = itemgetter(0)(items)` / `a = operator.itemgetter(0)(items)` (collection element),
// `pair = itemgetter(0)(pairs)` when pairs is a pair-iter (pairSlots + shared elemOf),
// `a = items[0]` / `a = d[k]` / `a = items.copy()[0]` (element/value of a known collection),
// `a = items.pop()` / `a = items.pop(0)` / `a = d.pop(k)` (same element/value type),
// `a = items.popleft()` (collections.deque; same element type as pop),
// `a = d.get(k)` / `a = d.get(k, default)` (dict value type; default ignored like next),
// `a = d.setdefault(k)` / `a = d.setdefault(k, default)` (same dict value type),
// `k, a = d.popitem()` (dict value leaf on 2nd unpack slot; pair itself untyped),
// `a = d.popitem()[1]` / `a = (d.popitem())[1]` (pair value slot; [0]/other fail closed),
// `it1, it2 = tee(items)` / `it1, it2 = itertools.tee(items[, n])` —
// each target is an iterator of items elements (elemOf; not an element itself),
// `xs = items.copy()` / `xs = items or []` (elemOf preserved for later index/for),
// `a = it.__next__()` when `it = iter(items)` (or other known iterable) has element type,
// as-bindings (`case A() as a`, `with A() as a`, `except A as e`),
// match sequence captures from a known collection subject
// (`match items: case [a]:` / `case [a, *rest]:` with `items: list[A]` —
// fixed slots are elements; *rest is a sequence of the same element type),
// match mapping value captures from a known dict subject
// (`match d: case {"k": a}:` / `case {"k": a as x}:` with `d: dict[K, A]` —
// value slots are the dict value leaf; **rest fails closed),
// walrus (`a := A()`, `a := next(items)`, `a := next(x for x in items)`,
// `a := min(items)`, `a := choice(items)`, `a := random.choice(items)`,
// `a := heappop(items)`, `a := heapq.heappop(items)`,
// `a := heappushpop(items, x)`, `a := heapreplace(items, x)` / heapq.*,
// `a := items.pop()`, `a := items.popleft()`, `a := d.get(k)`,
// `a := d.setdefault(k)`, `a := it.__next__()`, `a := items[0]`,
// `a := d.popitem()[1]` — same RHS typing as plain assignment),
// for/comprehension targets over known collections
// (`for a in [A()]`, `for a in items` with `items: list[A]`,
// `for a in items.copy()` / `for a in items or []`,
// `for a in d.values()` / `for k, a in d.items()` with `d: dict[K, A]`,
// `for i, a in enumerate(items)` / `for a, b in zip(xs, ys)`,
// `for a, b in zip(*[xs, ys])` / `for a, b in zip(*(xs, ys))`,
// `for a, b in zip_longest(xs, ys)` / `for a, b in itertools.zip_longest(xs, ys)`,
// `for a, b in pairwise(xs)` / `for a, b in itertools.pairwise(xs)`,
// `for a, b in batched(xs, n)` / `for a, b in itertools.batched(xs, n)` /
// `for batch in batched(xs, n): for a in batch` (batch → elemOf; n/strict ignored),
// `for combo in combinations(xs, r): for a in combo` /
// `for combo in permutations/combinations_with_replacement(...)` (combo → elemOf),
// `for combo in product(xs, ys): for a in combo` /
// `for combo in itertools.product(...)` (combo → elemOf when all args share type),
// `for pair in zip/zip_longest/pairwise(...): for a in pair` (pair → elemOf when shared),
// `for a, b in list/tuple/iter/reversed/sorted/filter(...zip...)` (identity wrappers),
// `for pair in list(zip(...)): for a in pair` (wrapper + nested shared elemOf),
// `pairs = zip/zip_longest/product/pairwise(...); for a, b in pairs` /
// `for pair in pairs: for a in pair` (assigned pair-iter; shared → elemOf),
// `combos = combinations/permutations/batched(...); for a, b in combos` /
// `for combo in combos: for a in combo` (assigned; literal r/n → pair slots),
// `for item in enumerate(xs): a = item[1]` (pair-slot subscript; [0] fails closed),
// `a, b = next(zip(xs, ys))` / `a, b = next(pairs)` when pairs = zip/... /
// `a, b = pair` when pair = next(pairs) / for-pair (pairSlots unpack),
// `a, b = pairs[0]` / `a, b = list(zip(...))[0]` (pair-iter index → pair),
// `a = pairs[0][0]` / `a = list(zip(...))[0][0]` (double subscript slot),
// `pair = pairs[0]; a = pair[0]` (index binds pairSlots),
// `i, a = next(enumerate(xs))` (index slot untyped; value is element),
// `for k, g in groupby(xs)` / `for k, g in itertools.groupby(xs)` —
// group g is an iterable of xs elements (key untyped; key= ignored),
// `for a in reversed/sorted/list/iter(items)`,
// `for a in set(items)` / `for a in frozenset(items)`,
// `for a in filter(pred, items)` / `for a in map(A, names)`,
// `for a in chain(xs, ys)` / `for a in itertools.chain(xs, ys)`,
// `for a in merge(xs, ys)` / `for a in heapq.merge(xs, ys)` (key/reverse ignored),
// `for a in islice(xs, n)` / `for a in itertools.islice(xs, n)`,
// `for a in accumulate(xs)` / `for a in itertools.accumulate(xs)`,
// `for a in cycle(xs)` / `for a in itertools.cycle(xs)`,
// `for a in repeat(item)` / `for a in itertools.repeat(item[, times])`
// (object type of 1st arg — typed local / Class(); times ignored),
// `for a in starmap(A, pairs)` / `for a in itertools.starmap(A, pairs)`,
// `for a in takewhile/dropwhile/filterfalse(pred, xs)` /
// `for a in itertools.takewhile/dropwhile/filterfalse(pred, xs)`,
// `for a in compress(xs, selectors)` / `for a in itertools.compress(xs, selectors)`,
// `for a in choices(xs)` / `for a in random.choices(xs, k=n)` /
// `for a in sample(xs, k)` / `for a in random.sample(xs, k)`,
// `for a in Counter(items)` / `for a in collections.Counter(items)`,
// `for a in Counter(items).elements()`,
// `for a in dict.fromkeys(items)` / `d = dict.fromkeys(keys, A()); d.values()`),
// tuple/list unpack (`a, b = A(), B()`, `[a, b] = [A(), B()]`,
// `a, *rest = items` / `*rest, a = items` / `a, = items` from list[A],
// `for a, b in [(A(), B())]`), and
// `except* A as e` → `for a in e.exceptions` (ExceptionGroup element type),
// `xa = box.a` / `box.a.run()` when box is a typed local of a class/dataclass
// with annotated field a: A (fieldOf; under foreign same-leaf methods),
// or a collections.namedtuple with field types recovered from same-file
// constructors (`Box = namedtuple("Box", ["a","b"]); box = Box(A(), B())`).
// fieldOf maps "local.field" → field type leaf for class field access.
func pythonTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) (map[string]bool, map[string]string) {
	out := map[string]bool{}
	// Collection locals → element type leaf (list[A] / [A()] / dict value → "A").
	elemOf := map[string]string{}
	// except* Type as name → ExceptionGroup local; .exceptions elements are Type.
	egElems := map[string]string{}
	// Class-typed object locals → type leaf (item: A / x = A() → "A").
	// Includes foreign types so repeat(item) can shadow same-leaf B correctly.
	typeOf := map[string]string{}
	// Pair/tuple locals → per-slot types (for item in enumerate/zip(...); item[i]).
	// Empty slot ("") fails closed on subscript/unpack of that index.
	pairSlots := map[string][]string{}
	// Assigned pair-iterators → per-slot types of each yielded tuple
	// (pairs = zip/enumerate/product/...; combos = combinations/batched(xs, n);
	// for a, b in pairs / for pair in pairs).
	pairIterSlots := map[string][]string{}
	// Class field access: "box.a" → "A" when box is typed as a class with field a: A.
	fieldOf := map[string]string{}
	if root == nil || len(ourReceivers) == 0 {
		return out, fieldOf
	}
	fieldIndex := pythonClassFieldIndex(root, content)
	// namedtuple factory fields have no annotations — recover from same-file ctors.
	pythonMergeNamedtupleFields(root, content, fieldIndex)
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "parameters", "lambda_parameters":
			// function params: (self, b: Box) / (b: "Box") / (items: list[Box])
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
						lname := ingest.NodeText(nameN, content)
						if tn := pythonTypeName(typeN, content); tn != "" {
							// Object annotation (item: A / item: B) — foreign too for shadowing.
							typeOf[lname] = tn
							pythonBindClassLocalFields(lname, tn, fieldIndex, fieldOf)
							if ourReceivers[tn] {
								out[lname] = true
							}
						}
						// Record even foreign element types so a later `items: list[B]`
						// shadows a prior `items: list[A]` (file-global map).
						if et := pythonContainerElemType(typeN, content); et != "" {
							elemOf[lname] = et
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
					if tn := pythonTypeName(typeN, content); tn != "" {
						// x: A / x: B = ... — foreign too for shadowing.
						typeOf[lname] = tn
						pythonBindClassLocalFields(lname, tn, fieldIndex, fieldOf)
						if ourReceivers[tn] {
							out[lname] = true
						}
					}
					// Foreign element types too — shadow prior same-name collections.
					if et := pythonContainerElemType(typeN, content); et != "" {
						elemOf[lname] = et
					}
				}
				if right != nil && right.Type() == "call" {
					fn := ingest.ChildByField(right, "function")
					if fn != nil && fn.Type() == "identifier" {
						fname := ingest.NodeText(fn, content)
						if ourReceivers[fname] {
							// x = A() — Class() ctor of our receiver.
							out[lname] = true
							typeOf[lname] = fname
							pythonBindClassLocalFields(lname, fname, fieldIndex, fieldOf)
						} else if len(fieldIndex[fname]) > 0 {
							// x = Box(...) — namedtuple/class with known fields (not our
							// receiver); bind fieldOf so box.a.run() / xa = box.a work.
							typeOf[lname] = fname
							pythonBindClassLocalFields(lname, fname, fieldIndex, fieldOf)
						}
						// a = cast(A, x) / cast("A", x)
						if fname == "cast" {
							if tn := pythonCastTypeArg(right, content); ourReceivers[tn] {
								out[lname] = true
							}
						}
						// a = next(iter(items)) / next(items) / next(x for x in items) /
						// next(reversed(items)) — result type is the element type of
						// the iterable arg (identity genexp preserves that type).
						// pair = next(pairs) when pairs = zip/enumerate(...) — pair is a
						// tuple (pairSlots + shared elemOf), not an element; use pair[i] /
						// unpack / nested for a in pair.
						if fname == "next" {
							if types := pythonNextPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonNextElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						}
						// a = min(items) / max(items) / min(items, key=...) —
						// single-iterable form yields an element of that iterable.
						// Multi-arg min(a, b, ...) fails closed (not an iterable fold).
						// pair = min(pairs) when pairs is a pair-iter (list(zip(...)), …) —
						// pair is a tuple (pairSlots + shared elemOf), not an element.
						if fname == "min" || fname == "max" {
							if types := pythonMinMaxPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonMinMaxElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						}
						// a = choice(items) — from random import choice; element of seq.
						// pair = choice(pairs) when pairs is a pair-iter (pairSlots + shared
						// elemOf), not an element; use pair[i] / unpack / nested for.
						if fname == "choice" {
							if types := pythonChoicePairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonRandomChoiceElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						}
						// a = heappop(items) / heappushpop(items, x) / heapreplace(items, x)
						// — from heapq import …; element of heap (1st arg).
						// pair = heappop(pairs) when pairs is a pair-iter (pairSlots + shared
						// elemOf), not an element; use pair[i] / unpack / nested for.
						if fname == "heappop" || fname == "heappushpop" || fname == "heapreplace" {
							if types := pythonHeappopPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonHeappopElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						}
						// a = reduce(fn, items) / reduce(fn, items, init) — from functools
						// import reduce; result type is the iterable element (fold of same
						// leaf). Multi-arg without iterable fails closed inside helper.
						if fname == "reduce" {
							if et := pythonReduceElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					} else if fn != nil && fn.Type() == "attribute" {
						if attr := ingest.ChildByField(fn, "attribute"); attr != nil {
							switch ingest.NodeText(attr, content) {
							case "cast":
								// typing.cast(A, x)
								if tn := pythonCastTypeArg(right, content); ourReceivers[tn] {
									out[lname] = true
								}
							case "pop", "popleft", "get", "setdefault":
								// a = items.pop() / items.pop(0) / d.pop(k) / list(items).pop()
								// a = items.popleft() (deque) — element type of receiver.
								// a = d.get(k) / d.get(k, default) — element/value type of
								// the receiver collection (dict value leaf via elemOf).
								// a = d.setdefault(k) / d.setdefault(k, default) — same.
								// Default arg on get/setdefault is ignored (same as next's default).
								// pair = pairs.pop() / pairs.pop(0) when pairs is a pair-iter
								// (list(zip(...)), …) — pair is a tuple (pairSlots + shared
								// elemOf), not an element; use pair[i] / unpack / nested for.
								// popitem() yields a (key, value) pair — single-name bind fails
								// closed (pair, not value); use unpack `k, a = d.popitem()` or
								// pair subscript `a = d.popitem()[1]` (via pythonSubscriptElemType).
								// Other methods fail closed.
								obj := ingest.ChildByField(fn, "object")
								if ingest.NodeText(attr, content) == "pop" {
									if types := pythonPopPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
										pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
										break
									}
								}
								if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
									out[lname] = true
								}
							case "heappop", "heappushpop", "heapreplace":
								// a = heapq.heappop(items) / heapq.heappushpop(items, x) /
								// heapq.heapreplace(items, x) — element of 1st arg (heap).
								// pair = heapq.heappop(pairs) when pairs is a pair-iter.
								if types := pythonHeappopPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
									pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
								} else if et := pythonHeappopElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
									out[lname] = true
								}
							case "choice":
								// a = random.choice(items) — module-qualified; element of seq.
								// pair = random.choice(pairs) when pairs is a pair-iter.
								if types := pythonChoicePairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
									pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
								} else if et := pythonRandomChoiceElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
									out[lname] = true
								}
							case "reduce":
								// a = functools.reduce(fn, items[, init]) — module-qualified.
								if et := pythonReduceElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
									out[lname] = true
								}
							case "__next__":
								// a = it.__next__() — element type of iterator/collection
								// receiver (it = iter(items) preserves items' element type).
								// Other methods fail closed.
								obj := ingest.ChildByField(fn, "object")
								if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
									out[lname] = true
								}
							}
						}
					}
					// a = itemgetter(0)(items) / operator.itemgetter(0)(items) —
					// single-index getter applied to a collection yields an element
					// (same as items[0]). Multi-index / other callables fail closed.
					// pair = itemgetter(0)(pairs) when pairs is a pair-iter (pairSlots +
					// shared elemOf), not an element; use pair[i] / unpack / nested for.
					if types := pythonItemgetterPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
						pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
					} else if et := pythonItemgetterElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
						out[lname] = true
					}
				}
				// a = items[0] / a = d[k] / a = list(items)[0] — element/value of collection.
				// a = item[1] when item from enumerate/zip pair (pairSlots).
				// a = pairs[0][0] / a = list(zip(...))[0][0] — double subscript slot.
				// pair = pairs[0] / pair = list(zip(...))[0] — index into pair-iter binds
				// pairSlots (+ elemOf when slots share type, for nested for/next).
				// Slices (items[1:3]) fail closed (sequence, not element).
				if right != nil && right.Type() == "subscript" {
					if types := pythonPairSlotsOf(right, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots); len(types) > 0 {
						// Foreign slots too — shadow prior same-name pair locals.
						pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
					} else if et := pythonSubscriptElemType(right, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots); ourReceivers[et] {
						out[lname] = true
					}
				}
				// xa = box.a — dataclass/class field access when box is a typed local
				// with annotated field a: A (under foreign same-leaf methods).
				if right != nil && right.Type() == "attribute" {
					if ft := pythonFieldAccessType(right, content, fieldOf); ft != "" {
						typeOf[lname] = ft
						pythonBindClassLocalFields(lname, ft, fieldIndex, fieldOf)
						if ourReceivers[ft] {
							out[lname] = true
						}
					}
				}
				// xs = [A()] / (A(),) / [B()] — track element type for later for-loops.
				// xs = list(items) / filter(...) — preserve element type via wrappers.
				// d = dict.fromkeys(keys, A()) — value leaf is A (for .values/.get).
				// pairs = zip/enumerate/product/pairwise(...) /
				// pairs = list/tuple/iter/reversed/sorted/filter(...zip...) —
				// combos = combinations/permutations/batched(...) (literal r/n) — pair-iter slots.
				if right != nil {
					if types := pythonPairIterSlotsOf(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
						// Foreign slots too — shadow prior same-name pair-iters.
						pairIterSlots[lname] = types
					} else if et := pythonDictFromkeysValueType(right, content); et != "" {
						elemOf[lname] = et
					} else if et := pythonHomogeneousCtorElem(right, content); et != "" {
						elemOf[lname] = et
					} else if et := pythonIterableElemType(right, content, elemOf, egElems, typeOf); et != "" {
						elemOf[lname] = et
					}
				}
			}
			// a, b = A(), B() / (a, b) = A(), B() / a, b = (A(), B()) /
			// [a, b] = [A(), B()] /
			// a, b = next(zip(xs, ys)) / a, b = next(pairs) / a, b = pair /
			// a, b = pairs[0] / a, b = list(zip(...))[0]
			// (pair-slot unpack; see pythonAssignPairUnpackTypes) /
			// k, a = d.popitem() (value leaf on 2nd slot; same as for k, a in d.items()) /
			// it1, it2 = tee(items) / itertools.tee(items[, n]) (each → elemOf) /
			// a, *rest = items / *rest, a = items / a, = items (items: list[A])
			if left != nil && right != nil {
				if targets := pythonPatternIdents(left, content); len(targets) > 0 {
					if types := pythonCtorListTypes(right, content); len(types) > 0 {
						for i, name := range targets {
							if i < len(types) && ourReceivers[types[i]] {
								out[name] = true
							}
						}
					} else if types := pythonAssignPairUnpackTypes(right, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots); len(types) > 0 {
						// a, b = next(zip(...)) / next(pairs) / pair / pairs[0] /
						// list(zip(...))[0] when pair/pair-iter slots known.
						// Per-slot: ourReceivers only; untyped slots (enumerate index) skip.
						for i, name := range targets {
							if i < len(types) && ourReceivers[types[i]] {
								out[name] = true
							}
						}
					} else if vt := pythonDictPopitemValueType(right, content, elemOf); vt != "" {
						// k, a = d.popitem() — value type is elemOf[d] (dict value leaf).
						if len(targets) >= 2 && ourReceivers[vt] {
							out[targets[1]] = true
						}
					} else if et := pythonTeeElemType(right, content, elemOf, egElems, typeOf); et != "" {
						// it1, it2 = tee(items) / itertools.tee(items[, n]) —
						// each target is an iterator of items elements (like groupby's g).
						// Do not put targets into out (iterators, not elements).
						// Foreign element types too — shadow prior same-name collections.
						for _, name := range targets {
							elemOf[name] = et
						}
					} else if et := pythonIterableElemType(right, content, elemOf, egElems, typeOf); et != "" {
						// a, b = items / a, = items — homogeneous collection elements.
						for _, name := range targets {
							if ourReceivers[et] {
								out[name] = true
							}
						}
					}
				} else if fixed, star, ok := pythonUnpackFixedAndStar(left, content); ok {
					// a, *rest = items / *rest, a = items — fixed slots are elements;
					// *rest is a sequence of the same element type (elemOf[rest]).
					if et := pythonIterableElemType(right, content, elemOf, egElems, typeOf); et != "" {
						for _, name := range fixed {
							if ourReceivers[et] {
								out[name] = true
							}
						}
						if star != "" {
							// Foreign element types too — shadow prior same-name collections.
							elemOf[star] = et
						}
					}
				}
			}
		case "named_expression":
			// Walrus: (a := A()) / (a := next(items)) / (a := min(items)) /
			// (a := heappop(items)) / (a := heapq.heappop(items)) /
			// (a := heappushpop(items, x)) / (a := heapreplace(items, x)) /
			// (a := reduce(...)) / (a := itemgetter(0)(items)) /
			// (a := items.pop()) / (a := d.get(k)) / (a := d.setdefault(k)) /
			// (a := items[0]) — mirror assignment RHS typing. Without this,
			// a.m() is skipped under foreign same-leaf.
			nameN := ingest.ChildByField(n, "name")
			valueN := ingest.ChildByField(n, "value")
			if nameN == nil || valueN == nil {
				break
			}
			lname := ingest.NodeText(nameN, content)
			if valueN.Type() == "call" {
				fn := ingest.ChildByField(valueN, "function")
				if fn != nil && fn.Type() == "identifier" {
					fname := ingest.NodeText(fn, content)
					if ourReceivers[fname] {
						// a := A() — Class() ctor of our receiver.
						out[lname] = true
						typeOf[lname] = fname
					}
					// a := cast(A, x)
					if fname == "cast" {
						if tn := pythonCastTypeArg(valueN, content); ourReceivers[tn] {
							out[lname] = true
						}
					}
					// a := next(iter(items)) / next(items) / next(x for x in items) /
					// next(reversed(items)) /
					// pair := next(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "next" {
						if types := pythonNextPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonNextElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						}
					}
					// a := min(items) / max(items) / min(items, key=...) /
					// pair := min(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "min" || fname == "max" {
						if types := pythonMinMaxPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonMinMaxElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						}
					}
					// a := choice(items) — from random import choice /
					// pair := choice(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "choice" {
						if types := pythonChoicePairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonRandomChoiceElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						}
					}
					// a := heappop(items) / heappushpop(items, x) / heapreplace(items, x) /
					// pair := heappop(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "heappop" || fname == "heappushpop" || fname == "heapreplace" {
						if types := pythonHeappopPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonHeappopElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						}
					}
					// a := reduce(fn, items) / reduce(fn, items, init)
					if fname == "reduce" {
						if et := pythonReduceElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						}
					}
				} else if fn != nil && fn.Type() == "attribute" {
					if attr := ingest.ChildByField(fn, "attribute"); attr != nil {
						switch ingest.NodeText(attr, content) {
						case "cast":
							// typing.cast(A, x)
							if tn := pythonCastTypeArg(valueN, content); ourReceivers[tn] {
								out[lname] = true
							}
						case "pop", "popleft", "get", "setdefault":
							// a := items.pop() / items.pop(0) / d.pop(k)
							// a := items.popleft() (deque)
							// a := d.get(k) / d.get(k, default)
							// a := d.setdefault(k) / d.setdefault(k, default)
							// pair := pairs.pop() when pairs is a pair-iter (pairSlots + shared elemOf).
							obj := ingest.ChildByField(fn, "object")
							if ingest.NodeText(attr, content) == "pop" {
								if types := pythonPopPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
									pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
									break
								}
							}
							if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						case "heappop", "heappushpop", "heapreplace":
							// a := heapq.heappop(items) / heapq.heappushpop /
							// heapq.heapreplace — element of heap (1st arg).
							// pair := heapq.heappop(pairs) when pairs is a pair-iter.
							if types := pythonHeappopPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonHeappopElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						case "choice":
							// a := random.choice(items) /
							// pair := random.choice(pairs) when pairs is a pair-iter.
							if types := pythonChoicePairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonRandomChoiceElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						case "reduce":
							// a := functools.reduce(fn, items[, init])
							if et := pythonReduceElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						case "__next__":
							// a := it.__next__() — element type of iterator receiver
							obj := ingest.ChildByField(fn, "object")
							if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					}
				}
				// a := itemgetter(0)(items) / operator.itemgetter(0)(items) /
				// pair := itemgetter(0)(pairs) when pairs is a pair-iter.
				if types := pythonItemgetterPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
					pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
				} else if et := pythonItemgetterElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
					out[lname] = true
				}
			}
			// a := items[0] / a := d[k] — element/value of known collection.
			// pair := pairs[0] / pair := list(zip(...))[0] — pairSlots (+ shared elemOf).
			// a := pairs[0][0] — double subscript slot.
			// Slices fail closed (sequence, not element).
			if valueN.Type() == "subscript" {
				if types := pythonPairSlotsOf(valueN, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots); len(types) > 0 {
					pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
				} else if et := pythonSubscriptElemType(valueN, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots); ourReceivers[et] {
					out[lname] = true
				}
			}
			// xa := box.a — dataclass/class field access (same as plain assignment).
			if valueN.Type() == "attribute" {
				if ft := pythonFieldAccessType(valueN, content, fieldOf); ft != "" {
					typeOf[lname] = ft
					pythonBindClassLocalFields(lname, ft, fieldIndex, fieldOf)
					if ourReceivers[ft] {
						out[lname] = true
					}
				}
			}
		case "except_clause":
			// except* A as e: e is ExceptionGroup, not A. Skip as_pattern typing of e
			// and record that e.exceptions carries A (foreign too, for shadowing).
			if pythonExceptClauseIsStar(n) {
				if asPat := ingest.ChildByType(n, "as_pattern"); asPat != nil {
					if name, typ := pythonAsPatternBinding(asPat, content); name != "" && typ != "" {
						egElems[name] = typ
					}
				}
				// Walk body without re-processing as_pattern as a plain except binding.
				for i := uint32(0); i < n.ChildCount(); i++ {
					ch := n.Child(i)
					if ch.Type() == "as_pattern" {
						continue
					}
					walk(ch)
				}
				return
			}
		case "for_statement", "for_in_clause":
			// for a in items / for a in [A()] / for a in d.values() /
			// for k, a in d.items() / for a, b in [(A(), B())] /
			// for i, a in enumerate(items) / for a, b in zip(xs, ys) /
			// for a, b in zip(*[xs, ys]) / zip(*(xs, ys)) /
			// for a, b in zip_longest / itertools.zip_longest /
			// for a, b in pairwise / itertools.pairwise /
			// for a, b in product / itertools.product /
			// for combo/pair in zip/zip_longest/product/pairwise (→ elemOf when shared) /
			// for a, b in combinations/permutations / itertools.* /
			// for combo in combinations/permutations / itertools.* (combo → elemOf) /
			// for a, b in batched / itertools.batched (each slot → elem; n ignored) /
			// for batch in batched / itertools.batched (batch → elemOf) /
			// for k, g in groupby / itertools.groupby (g → elemOf; key untyped) /
			// for a in reversed/sorted/list/iter(items) /
			// for a in filter(pred, items) / for a in map(A, names) /
			// for a in chain/islice/accumulate/cycle / itertools.chain/islice/accumulate/cycle /
			// for a in merge / heapq.merge (shared elem type; key/reverse ignored) /
			// for a in repeat(item) / itertools.repeat(item) (object type, not iterable) /
			// for a in starmap(A, pairs) / itertools.starmap(A, pairs) /
			// for a in chain.from_iterable / itertools.chain.from_iterable /
			// for a in takewhile/dropwhile/filterfalse / itertools.takewhile/dropwhile/filterfalse /
			// for a in compress / itertools.compress /
			// for a in nlargest/nsmallest / heapq.nlargest/nsmallest /
			// for a in choices/sample / random.choices/random.sample /
			// for a in dict.fromkeys(items) /
			// [a.m() for a in xs] / for a in e.exceptions
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left == nil || right == nil {
				break
			}
			switch left.Type() {
			case "identifier":
				// for batch in batched(xs, n) / itertools.batched(...) —
				// batch is a tuple of xs elements (elemOf), not an element itself.
				// Bind into elemOf so nested `for a in batch` / next(batch) type.
				if et := pythonBatchedElemType(right, content, elemOf, egElems, typeOf); et != "" {
					// Foreign element types too — shadow prior same-name collections.
					elemOf[ingest.NodeText(left, content)] = et
					break
				}
				// for combo in combinations/permutations/combinations_with_replacement —
				// combo is a tuple of xs elements (elemOf), not an element itself.
				if et := pythonCombPermElemType(right, content, elemOf, egElems, typeOf); et != "" {
					elemOf[ingest.NodeText(left, content)] = et
					break
				}
				// for pair in pairs when pairs = zip/enumerate/product/... —
				// for pair in zip/list(zip)/reversed(list(zip))/... —
				// bind pairSlots (subscript) + elemOf when all slots share type.
				// enumerate has untyped index → pairSlots only (item[1] path).
				// zip(*[xs, ys]) splat + identity wrappers via pythonPairIterSlotsOf.
				if types := pythonPairIterSlotsOf(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
					pythonBindPairLoopTarget(ingest.NodeText(left, content), types, pairSlots, elemOf)
					break
				}
				if et := pythonIterableElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
					out[ingest.NodeText(left, content)] = true
				}
			case "pattern_list", "tuple_pattern":
				targets := pythonPatternIdents(left, content)
				if len(targets) == 0 {
					break
				}
				// for k, v in d.items() — value type is elemOf[d] (dict value leaf).
				if vt := pythonDictItemsValueType(right, content, elemOf); vt != "" {
					if len(targets) >= 2 && ourReceivers[vt] {
						out[targets[1]] = true
					}
					break
				}
				// for a, b in pairs when pairs = zip/enumerate/product/... /
				// for i, a in enumerate(xs) / for a, b in zip(xs, ys) /
				// for a, b in zip(*[xs, ys]) / zip(*(xs, ys)) /
				// for a, b in zip_longest/product/pairwise / itertools.* /
				// for a, b in list/tuple/iter/reversed/sorted/filter(...zip...) —
				// identity wrappers preserve pair slots (see pythonPairIterSlotsOf).
				if types := pythonPairIterSlotsOf(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
					for i, name := range targets {
						if i < len(types) && ourReceivers[types[i]] {
							out[name] = true
						}
					}
					break
				}
				// for a, b in combinations(items, r) / permutations(items, r) /
				// combinations_with_replacement(items, r) / itertools.* —
				// every unpack slot is an element of the iterable (r ignored).
				if et := pythonCombPermElemType(right, content, elemOf, egElems, typeOf); et != "" {
					for _, name := range targets {
						if ourReceivers[et] {
							out[name] = true
						}
					}
					break
				}
				// for a, b in batched(items, n) / itertools.batched(...) —
				// every unpack slot is an element of the iterable (n/strict ignored).
				if et := pythonBatchedElemType(right, content, elemOf, egElems, typeOf); et != "" {
					for _, name := range targets {
						if ourReceivers[et] {
							out[name] = true
						}
					}
					break
				}
				// for k, g in groupby(xs) / itertools.groupby(xs[, key]) —
				// group g is an iterable of xs elements (key untyped; key= ignored).
				// Bind g into elemOf so nested `for a in g` / next(g) / list(g) type.
				// Do not put g itself into out (group is not an element of ourReceivers).
				if et := pythonGroupbyGroupElemType(right, content, elemOf, egElems, typeOf); et != "" {
					if len(targets) >= 2 {
						// Foreign element types too — shadow prior same-name collections.
						elemOf[targets[1]] = et
					}
					break
				}
				// for a, b in [(A(), B())] — position-wise Class() types.
				if types := pythonHomogeneousPairCtorTypes(right, content); len(types) > 0 {
					for i, name := range targets {
						if i < len(types) && ourReceivers[types[i]] {
							out[name] = true
						}
					}
				}
			}
		case "as_pattern":
			// match `case A() as a`, with `with A() as a`, except `except A as e`.
			// except* is handled above (e is ExceptionGroup, not A).
			// Without this, a.m() is skipped when a foreign same-leaf method exists.
			if name, typ := pythonAsPatternBinding(n, content); name != "" && ourReceivers[typ] {
				out[name] = true
			}
		case "match_statement":
			// match items: case [a]: / case [a, *rest]: — bind sequence captures
			// from the subject's element type (items: list[A] / xs = [A()] / …).
			// match d: case {"k": a}: — bind mapping value captures from the
			// subject's dict value leaf (d: dict[K, A] / …). Without this,
			// a.m() is skipped under foreign same-leaf; *rest loops also stay
			// untyped. as_pattern cases still handled above when walked.
			subject := ingest.ChildByField(n, "subject")
			if subject != nil {
				if et := pythonIterableElemType(subject, content, elemOf, egElems, typeOf); et != "" {
					body := ingest.ChildByField(n, "body")
					if body != nil {
						for i := uint32(0); i < body.ChildCount(); i++ {
							ch := body.Child(i)
							if ch.Type() != "case_clause" {
								continue
							}
							// Patterns precede the consequence block.
							for j := uint32(0); j < ch.ChildCount(); j++ {
								p := ch.Child(j)
								if p.Type() == "block" {
									break
								}
								pythonBindMatchSeqPatterns(p, content, et, ourReceivers, out, elemOf)
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
	walk(root)
	return out, fieldOf
}

// pythonClassFieldIndex maps class type name → field name → field type leaf
// from same-file class_definition annotated assignments (Box with a: A →
// "Box" → {"a":"A"}). Covers dataclass fields and plain annotated class attrs.
func pythonClassFieldIndex(root *grammar.Node, content []byte) map[string]map[string]string {
	out := map[string]map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "class_definition" {
			nameN := ingest.ChildByField(n, "name")
			body := ingest.ChildByField(n, "body")
			if nameN != nil && body != nil {
				typeName := ingest.NodeText(nameN, content)
				fields := map[string]string{}
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					// a: A / a: A = ... — annotated assignment in class body.
					if ch.Type() != "assignment" {
						continue
					}
					left := ingest.ChildByField(ch, "left")
					typeN := ingest.ChildByField(ch, "type")
					if left == nil || typeN == nil || left.Type() != "identifier" {
						continue
					}
					if tn := pythonTypeName(typeN, content); tn != "" {
						fields[ingest.NodeText(left, content)] = tn
					}
				}
				if len(fields) > 0 {
					out[typeName] = fields
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

// pythonMergeNamedtupleFields recovers field type leaves for factory namedtuples
// (no annotations): Box = namedtuple("Box", ["a","b"]) / collections.namedtuple
// plus same-file constructors Box(A(), B()) / Box(a=A(), b=B()) →
// fieldIndex["Box"]["a"]="A". Enables box.a.run() / xa = box.a under foreign
// same-leaf methods (same fieldOf path as annotated dataclass fields).
func pythonMergeNamedtupleFields(root *grammar.Node, content []byte, fieldIndex map[string]map[string]string) {
	if root == nil || fieldIndex == nil {
		return
	}
	fieldNames := pythonNamedtupleFieldNames(root, content)
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "call" {
			fn := ingest.ChildByField(n, "function")
			if fn != nil && fn.Type() == "identifier" {
				typeName := ingest.NodeText(fn, content)
				pythonIndexNamedtupleCtorFields(n, typeName, content, fieldNames[typeName], fieldIndex)
			}
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
}

// pythonNamedtupleFieldNames maps local type name → ordered field names from
// Box = namedtuple(...) / Box = collections.namedtuple(...) assignments.
func pythonNamedtupleFieldNames(root *grammar.Node, content []byte) map[string][]string {
	out := map[string][]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "assignment" {
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left != nil && left.Type() == "identifier" && right != nil && right.Type() == "call" {
				if pythonIsNamedtupleCall(right, content) {
					if fields := pythonParseNamedtupleFieldList(right, content); len(fields) > 0 {
						out[ingest.NodeText(left, content)] = fields
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

// pythonIsNamedtupleCall reports namedtuple(...) / collections.namedtuple(...).
func pythonIsNamedtupleCall(call *grammar.Node, content []byte) bool {
	if call == nil || call.Type() != "call" {
		return false
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return false
	}
	if fn.Type() == "identifier" {
		return ingest.NodeText(fn, content) == "namedtuple"
	}
	if fn.Type() == "attribute" {
		obj := ingest.ChildByField(fn, "object")
		attr := ingest.ChildByField(fn, "attribute")
		return obj != nil && attr != nil &&
			obj.Type() == "identifier" &&
			ingest.NodeText(obj, content) == "collections" &&
			ingest.NodeText(attr, content) == "namedtuple"
	}
	return false
}

// pythonParseNamedtupleFieldList returns field names from the 2nd positional
// arg of namedtuple(typename, fields): list/tuple of strings, or a single
// string ("a b" / "a, b"). Other forms fail closed.
func pythonParseNamedtupleFieldList(call *grammar.Node, content []byte) []string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 2 {
		return nil
	}
	fieldArg := args[1]
	switch fieldArg.Type() {
	case "list", "tuple":
		var fields []string
		for i := uint32(0); i < fieldArg.ChildCount(); i++ {
			ch := fieldArg.Child(i)
			if ch.Type() != "string" {
				continue
			}
			_, text := pythonStringContent(ch, content)
			if !pythonIsIdentifier(text) {
				return nil
			}
			fields = append(fields, text)
		}
		return fields
	case "string":
		_, text := pythonStringContent(fieldArg, content)
		return pythonSplitNamedtupleFieldString(text)
	}
	return nil
}

// pythonSplitNamedtupleFieldString splits "a b" / "a, b" into field names.
func pythonSplitNamedtupleFieldString(s string) []string {
	if s == "" {
		return nil
	}
	// Normalize commas to spaces, then split on whitespace.
	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			buf = append(buf, ' ')
		} else {
			buf = append(buf, s[i])
		}
	}
	parts := strings.Fields(string(buf))
	var fields []string
	for _, p := range parts {
		if !pythonIsIdentifier(p) {
			return nil
		}
		fields = append(fields, p)
	}
	return fields
}

// pythonIndexNamedtupleCtorFields fills fieldIndex[typeName] from a constructor
// call: keyword Class() args always; positional Class() args when fieldNames
// are known from the namedtuple factory (order-sensitive).
func pythonIndexNamedtupleCtorFields(call *grammar.Node, typeName string, content []byte, fieldNames []string, fieldIndex map[string]map[string]string) {
	if call == nil || typeName == "" || fieldIndex == nil {
		return
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return
	}
	// Keyword: Box(a=A(), b=B()) — no factory field list required.
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch.Type() != "keyword_argument" {
			continue
		}
		nameN := ingest.ChildByField(ch, "name")
		valN := ingest.ChildByField(ch, "value")
		if nameN == nil || valN == nil {
			continue
		}
		if tn := pythonExprClassType(valN, content); tn != "" {
			fname := ingest.NodeText(nameN, content)
			if fieldIndex[typeName] == nil {
				fieldIndex[typeName] = map[string]string{}
			}
			fieldIndex[typeName][fname] = tn
		}
	}
	// Positional: Box(A(), B()) — needs ordered field names from factory.
	if len(fieldNames) == 0 {
		return
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok {
		return
	}
	for i, arg := range args {
		if i >= len(fieldNames) {
			break
		}
		if tn := pythonExprClassType(arg, content); tn != "" {
			if fieldIndex[typeName] == nil {
				fieldIndex[typeName] = map[string]string{}
			}
			fieldIndex[typeName][fieldNames[i]] = tn
		}
	}
}

// pythonExprClassType returns T for a Class() call expression (A() → "A").
// Other expressions fail closed (no typed-local lookup — pre-index pass).
func pythonExprClassType(n *grammar.Node, content []byte) string {
	if n == nil || n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "identifier" {
		return ""
	}
	return ingest.NodeText(fn, content)
}

// pythonBindClassLocalFields records "local.field" → type for each annotated
// field of a known same-file class type (enables box.a / xa = box.a typing).
func pythonBindClassLocalFields(local, typeName string, index map[string]map[string]string, fieldOf map[string]string) {
	if local == "" || typeName == "" || index == nil || fieldOf == nil {
		return
	}
	fields := index[typeName]
	if fields == nil {
		return
	}
	for f, t := range fields {
		fieldOf[local+"."+f] = t
	}
}

// pythonFieldAccessType recovers T from box.a when box is a typed local with
// annotated field a of type T (identifier object only; fail closed otherwise).
func pythonFieldAccessType(attr *grammar.Node, content []byte, fieldOf map[string]string) string {
	if attr == nil || attr.Type() != "attribute" || fieldOf == nil {
		return ""
	}
	obj := ingest.ChildByField(attr, "object")
	field := ingest.ChildByField(attr, "attribute")
	if obj == nil || field == nil || obj.Type() != "identifier" {
		return ""
	}
	return fieldOf[ingest.NodeText(obj, content)+"."+ingest.NodeText(field, content)]
}

// pythonBindMatchSeqPatterns binds capture names from match list/tuple/mapping
// patterns when the match subject has element/value type et.
// Sequence: fixed slots → out (if ourReceivers); *rest → elemOf (including
// foreign, for shadowing). Mapping: value captures in case {"k": a} → out;
// **rest fails closed (mapping, not an element). Fails closed on nested /
// class patterns inside the sequence/value (those use other binders).
func pythonBindMatchSeqPatterns(n *grammar.Node, content []byte, et string, ourReceivers, out map[string]bool, elemOf map[string]string) {
	if n == nil || n.IsNull() || et == "" {
		return
	}
	switch n.Type() {
	case "list_pattern", "tuple_pattern":
		fixed, star, ok := pythonMatchSeqPatternCaptures(n, content)
		if !ok {
			return
		}
		for _, name := range fixed {
			if ourReceivers[et] {
				out[name] = true
			}
		}
		if star != "" {
			// Foreign element types too — shadow prior same-name collections.
			elemOf[star] = et
		}
		return
	case "dict_pattern":
		// Mapping values are case_pattern/pattern children; keys are string /
		// integer / dotted_name (capture keys) and are not value-typed.
		// **rest (splat_pattern) is a mapping — fail closed.
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() != "case_pattern" && ch.Type() != "pattern" {
				continue
			}
			pythonBindMatchMapValueCaptures(ch, content, et, ourReceivers, out)
		}
		return
	case "case_pattern", "pattern", "as_pattern":
		// Unwrap; for as_pattern only the left pattern can contain a sequence/map.
		// (alias typing is handled by the as_pattern case in pythonTypedLocals,
		// or by pythonBindMatchMapValueCaptures for capture-as inside mappings.)
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "as" || ch.Type() == "as_pattern_target" {
				continue
			}
			// Skip bare alias identifier after `as`.
			if n.Type() == "as_pattern" && ch.Type() == "identifier" {
				// May be left (class name) or right (alias); recurse left only via
				// sequence patterns nested deeper — safe either way (idents no-op).
			}
			pythonBindMatchSeqPatterns(ch, content, et, ourReceivers, out, elemOf)
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		pythonBindMatchSeqPatterns(n.Child(i), content, et, ourReceivers, out, elemOf)
	}
}

// pythonBindMatchMapValueCaptures binds simple captures (and capture-as aliases)
// from a mapping value pattern when the dict subject's value type is et.
// Nested class/list/or patterns fail closed (class `A() as a` is handled by
// pythonAsPatternBinding when the tree is walked).
func pythonBindMatchMapValueCaptures(n *grammar.Node, content []byte, et string, ourReceivers, out map[string]bool) {
	if n == nil || n.IsNull() || et == "" {
		return
	}
	// Unwrap case_pattern/pattern wrappers to the payload.
	for n != nil && (n.Type() == "case_pattern" || n.Type() == "pattern") {
		inner := pythonMatchPatternInner(n)
		if inner == nil {
			return
		}
		n = inner
	}
	if n == nil || n.IsNull() {
		return
	}
	switch n.Type() {
	case "identifier", "dotted_name":
		name := pythonMatchCaptureName(n, content)
		if name != "" && ourReceivers[et] {
			out[name] = true
		}
	case "as_pattern":
		// `a as x` inside a mapping value: both bind to the value (PEP 634).
		// Left class patterns (A() as a) still get typing from pythonAsPatternBinding.
		var left *grammar.Node
		var alias string
		seenAs := false
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "as" {
				seenAs = true
				continue
			}
			if !seenAs {
				left = ch
				continue
			}
			switch ch.Type() {
			case "identifier":
				alias = ingest.NodeText(ch, content)
			case "as_pattern_target":
				if id := ingest.ChildByType(ch, "identifier"); id != nil {
					alias = ingest.NodeText(id, content)
				}
			}
		}
		if alias != "" && ourReceivers[et] {
			out[alias] = true
		}
		if left != nil {
			pythonBindMatchMapValueCaptures(left, content, et, ourReceivers, out)
		}
	default:
		// Nested list/class/or/value patterns — fail closed for dict-value typing.
	}
}

// pythonMatchSeqPatternCaptures returns fixed capture names and optional *rest
// from a match list_pattern / tuple_pattern (`case [a]:` / `case [a, *rest]:` /
// `case (a, b):`). Match grammar wraps captures in case_pattern and uses
// splat_pattern (not list_splat_pattern). Fails closed on nested patterns.
func pythonMatchSeqPatternCaptures(n *grammar.Node, content []byte) (fixed []string, star string, ok bool) {
	if n == nil {
		return nil, "", false
	}
	switch n.Type() {
	case "list_pattern", "tuple_pattern":
		// ok
	default:
		return nil, "", false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "(", ")", "[", "]", ",", "comment":
			continue
		case "case_pattern", "pattern":
			inner := pythonMatchPatternInner(ch)
			if inner == nil {
				return nil, "", false
			}
			name, isStar, okCap := pythonMatchCaptureOrStar(inner, content)
			if !okCap {
				return nil, "", false
			}
			if isStar {
				if star != "" {
					return nil, "", false
				}
				star = name
			} else {
				fixed = append(fixed, name)
			}
		case "splat_pattern":
			// Bare *rest child (some grammar shapes).
			if star != "" {
				return nil, "", false
			}
			id := ingest.ChildByType(ch, "identifier")
			if id == nil {
				return nil, "", false
			}
			star = ingest.NodeText(id, content)
		case "identifier", "dotted_name":
			name := pythonMatchCaptureName(ch, content)
			if name == "" {
				return nil, "", false
			}
			fixed = append(fixed, name)
		default:
			return nil, "", false
		}
	}
	if len(fixed) == 0 && star == "" {
		return nil, "", false
	}
	return fixed, star, true
}

// pythonMatchPatternInner unwraps a single case_pattern/pattern to its payload.
func pythonMatchPatternInner(n *grammar.Node) *grammar.Node {
	if n == nil {
		return nil
	}
	for n != nil && (n.Type() == "case_pattern" || n.Type() == "pattern") {
		var next *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			switch ch.Type() {
			case "comment":
				continue
			default:
				if next != nil {
					// Multiple payload children — ambiguous.
					return nil
				}
				next = ch
			}
		}
		if next == nil {
			return nil
		}
		n = next
	}
	return n
}

// pythonMatchCaptureOrStar returns a simple capture name, or *rest via splat_pattern.
func pythonMatchCaptureOrStar(n *grammar.Node, content []byte) (name string, isStar bool, ok bool) {
	if n == nil {
		return "", false, false
	}
	switch n.Type() {
	case "splat_pattern":
		id := ingest.ChildByType(n, "identifier")
		if id == nil {
			return "", false, false
		}
		return ingest.NodeText(id, content), true, true
	case "identifier", "dotted_name":
		name = pythonMatchCaptureName(n, content)
		if name == "" {
			return "", false, false
		}
		return name, false, true
	default:
		// class_pattern, nested list/tuple, value patterns — fail closed.
		return "", false, false
	}
}

// pythonMatchCaptureName returns the simple identifier for a capture pattern
// (identifier or single-segment dotted_name). Multi-segment dotted names fail closed.
func pythonMatchCaptureName(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "identifier":
		return ingest.NodeText(n, content)
	case "dotted_name":
		var id *grammar.Node
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() != "identifier" {
				continue
			}
			if id != nil {
				// pkg.name — not a simple capture.
				return ""
			}
			id = ch
		}
		if id == nil {
			return ""
		}
		return ingest.NodeText(id, content)
	}
	return ""
}

// pythonExceptClauseIsStar reports whether n is `except* ...` (PEP 654).
func pythonExceptClauseIsStar(n *grammar.Node) bool {
	if n == nil || n.Type() != "except_clause" {
		return false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.Child(i).Type() == "*" {
			return true
		}
	}
	return false
}

// pythonNextElemType recovers the element type yielded by next(iterable[, default]).
// Uses the first positional arg's iterable element type (next(iter(items)) → A when
// items: list[A]; next(x for x in items) → A for identity genexps). Fails closed on
// splat args or empty call. Default arg is ignored (result may be union with default
// at runtime; we still bind the element type).
func pythonNextElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonMinMaxElemType recovers the element type of min(iterable) / max(iterable)
// (optional key=/default= kwargs ignored). Only the single-positional-arg form is
// handled — min(a, b) / max(x, y, z) compare discrete values and fail closed.
func pythonMinMaxElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonRandomChoiceElemType recovers the element type of choice(seq) /
// random.choice(seq). The sequence is the first positional arg (same element
// typing as next(iterable)). Bare choice (from random import choice) and
// module-qualified random.choice are accepted; other receivers fail closed.
// choices/sample yield lists — see pythonIterableElemType.
func pythonRandomChoiceElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "choice" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "choice" {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "random" {
			return ""
		}
	default:
		return ""
	}
	return pythonNextElemType(call, content, elemOf, egElems, typeOf)
}

// pythonHeappopElemType recovers the element type of heappop(heap) /
// heappushpop(heap, item) / heapreplace(heap, item) and heapq.* forms.
// The heap is the first positional arg (same element typing as next(iterable)).
// Bare names (from heapq import …) and module-qualified heapq.* are accepted;
// other receivers fail closed. Extra args (item on pushpop/replace) ignored —
// result is always a heap element. nlargest/nsmallest yield lists — see
// pythonIterableElemType.
func pythonHeappopElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		switch ingest.NodeText(fn, content) {
		case "heappop", "heappushpop", "heapreplace":
			// ok
		default:
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil {
			return ""
		}
		switch ingest.NodeText(attr, content) {
		case "heappop", "heappushpop", "heapreplace":
			// ok
		default:
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "heapq" {
			return ""
		}
	default:
		return ""
	}
	return pythonNextElemType(call, content, elemOf, egElems, typeOf)
}

// pythonReduceElemType recovers the element type of reduce(function, iterable)
// / reduce(function, iterable, initializer) and functools.reduce(...).
// The iterable is the second positional arg; its element type is the fold result
// for same-leaf accumulators (common product case). Bare reduce (from functools
// import reduce) and module-qualified functools.reduce are accepted; other
// receivers fail closed. Fewer than 2 positional args fails closed.
func pythonReduceElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "reduce" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "reduce" {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "functools" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 2 {
		return ""
	}
	// reduce(function, iterable[, initializer]) — element type of the iterable.
	return pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
}

// pythonItemgetterElemType recovers the element type of
// itemgetter(i)(collection) / operator.itemgetter(i)(collection).
// Single-index itemgetter applied to a known collection yields one element
// (same as collection[i]). Multi-index itemgetter(i, j, ...) returns a tuple
// and fails closed. Bare itemgetter (from operator import itemgetter) and
// module-qualified operator.itemgetter are accepted; other receivers fail closed.
// Stored getters (g = itemgetter(0); a = g(items)) are not tracked.
func pythonItemgetterElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	// Outer call: getter(collection) — function must itself be itemgetter(...).
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "call" {
		return ""
	}
	innerFn := ingest.ChildByField(fn, "function")
	if innerFn == nil {
		return ""
	}
	switch innerFn.Type() {
	case "identifier":
		if ingest.NodeText(innerFn, content) != "itemgetter" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(innerFn, "attribute")
		obj := ingest.ChildByField(innerFn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "itemgetter" {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "operator" {
			return ""
		}
	default:
		return ""
	}
	// itemgetter must have exactly one positional arg (the index).
	idxArgs, ok := pythonCallPositionalArgNodes(fn)
	if !ok || len(idxArgs) != 1 {
		return ""
	}
	// Outer call: getter(collection) — exactly one positional arg.
	collArgs, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(collArgs) != 1 {
		return ""
	}
	return pythonIterableElemType(collArgs[0], content, elemOf, egElems, typeOf)
}

// pythonSubscriptElemType recovers the element type of items[i] / d[k] when the
// subscripted value is a known collection (elemOf / wrappers / literals).
// Also covers d.popitem()[1] / (d.popitem())[1] (pair value leaf; see
// pythonDictPopitemSubscriptValueType) and item[i] when item is a known
// enumerate/zip pair local (pairSlots). Fails closed on slices (items[a:b] /
// items[:]) — those yield sequences, not elements.
func pythonSubscriptElemType(sub *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairSlots, pairIterSlots map[string][]string) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	for i := uint32(0); i < sub.ChildCount(); i++ {
		if sub.Child(i).Type() == "slice" {
			return ""
		}
	}
	// d.popitem()[1] — pair value leaf (before generic collection subscript).
	if et := pythonDictPopitemSubscriptValueType(sub, content, elemOf); et != "" {
		return et
	}
	// item[1] / next(pairs)[0] when pair/pair-iter slots known.
	if et := pythonPairSlotSubscriptType(sub, content, pairSlots, elemOf, egElems, typeOf, pairIterSlots); et != "" {
		return et
	}
	val := ingest.ChildByField(sub, "value")
	return pythonIterableElemType(val, content, elemOf, egElems, typeOf)
}

// pythonPairSlotSubscriptType returns the slot type for pair[i] / next(pairs)[i] /
// pairs[0][i] / list(zip(...))[0][i] when i is a non-negative integer literal.
// The value may be a known pair local, next(pair_iter), or a pair-iter index
// (see pythonPairSlotsOf). Untyped slots (""), OOB indices, and non-literal
// indices fail closed. Parenthesized (item)[1] is accepted.
func pythonPairSlotSubscriptType(sub *grammar.Node, content []byte, pairSlots map[string][]string, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	idxN := ingest.ChildByField(sub, "subscript")
	if idxN == nil || idxN.Type() != "integer" {
		return ""
	}
	idxText := ingest.NodeText(idxN, content)
	if idxText == "" {
		return ""
	}
	idx := 0
	for _, c := range idxText {
		if c < '0' || c > '9' {
			return ""
		}
		idx = idx*10 + int(c-'0')
	}
	slots := pythonPairSlotsOf(ingest.ChildByField(sub, "value"), content, elemOf, egElems, typeOf, pairSlots, pairIterSlots)
	if idx < 0 || idx >= len(slots) {
		return ""
	}
	return slots[idx]
}

// pythonPairSlotsOf recovers per-slot types for a pair expression:
// pair local (pairSlots), next(pair_iter), min/max(pair_iter),
// choice/heappop/itemgetter(pair_iter),
// pair_iter.pop() / list(zip(...)).pop(),
// or pair_iter[i] / list(zip(...))[i] (index into a pair-yielding sequence yields
// one pair with those slots). Parenthesized forms accepted. Slices fail closed.
func pythonPairSlotsOf(n *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairSlots, pairIterSlots map[string][]string) []string {
	if n == nil {
		return nil
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return nil
	}
	switch n.Type() {
	case "identifier":
		if pairSlots == nil {
			return nil
		}
		return pairSlots[ingest.NodeText(n, content)]
	case "call":
		// next(pairs) / next(zip(xs, ys))
		if types := pythonNextPairSlots(n, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
			return types
		}
		// min(pairs) / max(pairs) / min(list(zip(...))) / max(list(zip(...)), key=...)
		if types := pythonMinMaxPairSlots(n, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
			return types
		}
		// choice(pairs) / random.choice(list(zip(...)))
		if types := pythonChoicePairSlots(n, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
			return types
		}
		// heappop(pairs) / heapq.heappop(list(zip(...))) / heappushpop / heapreplace
		if types := pythonHeappopPairSlots(n, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
			return types
		}
		// itemgetter(0)(pairs) / operator.itemgetter(0)(list(zip(...)))
		if types := pythonItemgetterPairSlots(n, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
			return types
		}
		// pairs.pop() / pairs.pop(0) / list(zip(...)).pop()
		return pythonPopPairSlots(n, content, elemOf, egElems, typeOf, pairIterSlots)
	case "subscript":
		// pairs[0] / list(zip(...))[0] / (list(zip(...)))[0] — one pair from a
		// pair-yielding sequence. Any non-slice index yields the same slots.
		for i := uint32(0); i < n.ChildCount(); i++ {
			if n.Child(i).Type() == "slice" {
				return nil
			}
		}
		val := ingest.ChildByField(n, "value")
		return pythonPairIterSlotsOf(val, content, elemOf, egElems, typeOf, pairIterSlots)
	default:
		return nil
	}
}

// pythonPairIterSlotsOf recovers per-slot types for a pair-iterator expression:
// zip/enumerate/product/pairwise calls, combinations/permutations/batched with
// a positive integer-literal size (r/n), assigned pair-iter locals, and identity
// wrappers list/tuple/iter/reversed/sorted/filter around those. Not an element
// type — each yield is a tuple (pair slots preserved through the wrapper).
func pythonPairIterSlotsOf(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) []string {
	if right == nil {
		return nil
	}
	for right != nil && right.Type() == "parenthesized_expression" {
		right = pythonParenInner(right)
	}
	if right == nil {
		return nil
	}
	switch right.Type() {
	case "identifier":
		if pairIterSlots == nil {
			return nil
		}
		return pairIterSlots[ingest.NodeText(right, content)]
	case "call":
		if types := pythonEnumerateZipTargetTypes(right, content, elemOf, egElems, typeOf); len(types) > 0 {
			return types
		}
		// combinations/permutations/combinations_with_replacement/batched with
		// literal r/n → homogeneous slots (assigned pair-iter reuse).
		if types := pythonCombBatchedPairSlots(right, content, elemOf, egElems, typeOf); len(types) > 0 {
			return types
		}
		// list/tuple/iter/reversed/sorted(zip(...)) / filter(pred, zip(...)) —
		// unwrap identity wrappers that re-yield the same pairs.
		fn := ingest.ChildByField(right, "function")
		if fn == nil || fn.Type() != "identifier" {
			return nil
		}
		args, ok := pythonCallPositionalArgNodes(right)
		if !ok {
			return nil
		}
		switch ingest.NodeText(fn, content) {
		case "list", "tuple", "iter", "reversed", "sorted":
			// 1st positional is the pair-iter (kwargs like key=/strict= ignored).
			if len(args) == 0 {
				return nil
			}
			return pythonPairIterSlotsOf(args[0], content, elemOf, egElems, typeOf, pairIterSlots)
		case "filter":
			// filter(function, iterable) — 2nd positional is the pair-iter.
			if len(args) < 2 {
				return nil
			}
			return pythonPairIterSlotsOf(args[1], content, elemOf, egElems, typeOf, pairIterSlots)
		}
		return nil
	default:
		return nil
	}
}

// pythonCombBatchedPairSlots returns [elem]*n for combinations / permutations /
// combinations_with_replacement / batched when the size arg (r or n) is a
// positive integer literal. Enables assigned pair-iters:
// `combos = combinations(xs, 2); for a, b in combos` via pairIterSlots.
// Non-literal size fails closed here; direct for-loops still type via
// pythonCombPermElemType / pythonBatchedElemType without needing the size.
func pythonCombBatchedPairSlots(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) []string {
	et := pythonCombPermElemType(right, content, elemOf, egElems, typeOf)
	if et == "" {
		et = pythonBatchedElemType(right, content, elemOf, egElems, typeOf)
	}
	if et == "" {
		return nil
	}
	n := pythonCallSecondPositionalInt(right, content)
	if n <= 0 {
		return nil
	}
	out := make([]string, n)
	for i := range out {
		out[i] = et
	}
	return out
}

// pythonCallSecondPositionalInt returns the 2nd positional arg as a non-negative
// integer literal, or -1 when missing / not a plain integer.
func pythonCallSecondPositionalInt(call *grammar.Node, content []byte) int {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 2 {
		return -1
	}
	return pythonNonNegIntLiteral(args[1], content)
}

// pythonNonNegIntLiteral parses a non-negative integer literal node, or -1.
func pythonNonNegIntLiteral(n *grammar.Node, content []byte) int {
	if n == nil || n.Type() != "integer" {
		return -1
	}
	text := ingest.NodeText(n, content)
	if text == "" {
		return -1
	}
	v := 0
	for _, c := range text {
		if c < '0' || c > '9' {
			return -1
		}
		v = v*10 + int(c-'0')
	}
	return v
}

// pythonNextPairSlots recovers pair slot types for next(pair_iter[, default]).
// The default arg is ignored (same as next element typing). Yields the tuple
// slots of one pair-iter item — not an element leaf.
func pythonNextPairSlots(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) []string {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" || ingest.NodeText(fn, content) != "next" {
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return nil
	}
	return pythonPairIterSlotsOf(args[0], content, elemOf, egElems, typeOf, pairIterSlots)
}

// pythonMinMaxPairSlots recovers pair slot types for min(pair_iter) / max(pair_iter)
// (optional key=/default= kwargs ignored). Only the single-positional-arg form —
// min(a, b) / max(x, y, z) fail closed (same as min/max element typing). Yields
// the tuple slots of one pair-iter item — not an element leaf.
func pythonMinMaxPairSlots(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) []string {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" {
		return nil
	}
	switch ingest.NodeText(fn, content) {
	case "min", "max":
		// ok
	default:
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return nil
	}
	return pythonPairIterSlotsOf(args[0], content, elemOf, egElems, typeOf, pairIterSlots)
}

// pythonChoicePairSlots recovers pair slot types for choice(pair_iter) /
// random.choice(pair_iter). Same call shape as element typing (1st positional
// is the sequence). Yields the tuple slots of one pair-iter item — not an
// element leaf. Other receivers fail closed.
func pythonChoicePairSlots(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) []string {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return nil
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "choice" {
			return nil
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "choice" {
			return nil
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "random" {
			return nil
		}
	default:
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return nil
	}
	return pythonPairIterSlotsOf(args[0], content, elemOf, egElems, typeOf, pairIterSlots)
}

// pythonHeappopPairSlots recovers pair slot types for heappop(pair_iter) /
// heappushpop(pair_iter, x) / heapreplace(pair_iter, x) and heapq.* forms.
// Same call shape as element typing (1st positional is the heap). Yields the
// tuple slots of one pair-iter item — not an element leaf. Extra args (item on
// pushpop/replace) ignored. Other receivers fail closed.
func pythonHeappopPairSlots(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) []string {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return nil
	}
	switch fn.Type() {
	case "identifier":
		switch ingest.NodeText(fn, content) {
		case "heappop", "heappushpop", "heapreplace":
			// ok
		default:
			return nil
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil {
			return nil
		}
		switch ingest.NodeText(attr, content) {
		case "heappop", "heappushpop", "heapreplace":
			// ok
		default:
			return nil
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "heapq" {
			return nil
		}
	default:
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return nil
	}
	return pythonPairIterSlotsOf(args[0], content, elemOf, egElems, typeOf, pairIterSlots)
}

// pythonItemgetterPairSlots recovers pair slot types for
// itemgetter(i)(pair_iter) / operator.itemgetter(i)(pair_iter). Single-index
// itemgetter applied to a known pair-iter yields one pair (same as
// pair_iter[i]). Multi-index itemgetter fails closed. Yields the tuple slots
// of one pair-iter item — not an element leaf.
func pythonItemgetterPairSlots(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) []string {
	if call == nil || call.Type() != "call" {
		return nil
	}
	// Outer call: getter(collection) — function must itself be itemgetter(...).
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "call" {
		return nil
	}
	innerFn := ingest.ChildByField(fn, "function")
	if innerFn == nil {
		return nil
	}
	switch innerFn.Type() {
	case "identifier":
		if ingest.NodeText(innerFn, content) != "itemgetter" {
			return nil
		}
	case "attribute":
		attr := ingest.ChildByField(innerFn, "attribute")
		obj := ingest.ChildByField(innerFn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "itemgetter" {
			return nil
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "operator" {
			return nil
		}
	default:
		return nil
	}
	// itemgetter must have exactly one positional arg (the index).
	idxArgs, ok := pythonCallPositionalArgNodes(fn)
	if !ok || len(idxArgs) != 1 {
		return nil
	}
	// Outer call: getter(collection) — exactly one positional arg.
	collArgs, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(collArgs) != 1 {
		return nil
	}
	return pythonPairIterSlotsOf(collArgs[0], content, elemOf, egElems, typeOf, pairIterSlots)
}

// pythonPopPairSlots recovers pair slot types for pairs.pop() / pairs.pop(i) /
// list(zip(...)).pop() when the receiver is a known pair-iter. Index args are
// ignored (any pop removes one pair with the same slots). popitem and other
// methods fail closed. Yields the tuple slots of one pair — not an element leaf.
func pythonPopPairSlots(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairIterSlots map[string][]string) []string {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return nil
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "pop" {
		return nil
	}
	obj := ingest.ChildByField(fn, "object")
	return pythonPairIterSlotsOf(obj, content, elemOf, egElems, typeOf, pairIterSlots)
}

// pythonAssignPairUnpackTypes recovers per-slot types for multi-target unpack
// from a known pair: a, b = next(zip(...)) / a, b = next(pairs) /
// a, b = pair / a, b = pairs[0] / a, b = list(zip(...))[0] /
// a, b = pairs.pop() / a, b = list(zip(...)).pop() /
// a, b = min(pairs) / a, b = max(list(zip(...))) /
// a, b = choice(pairs) / a, b = random.choice(list(zip(...))) /
// a, b = heappop(pairs) / a, b = heapq.heappop(list(zip(...))) /
// a, b = itemgetter(0)(pairs) / a, b = operator.itemgetter(0)(list(zip(...))) /
// [a, b] = next(pairs) when pair/pair-iter slots are known.
// Parenthesized forms accepted. Untyped slots stay "" (enumerate index).
func pythonAssignPairUnpackTypes(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairSlots map[string][]string, pairIterSlots map[string][]string) []string {
	return pythonPairSlotsOf(right, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots)
}

// pythonBindPairLoopTarget records pairSlots for a single-target for-loop over a
// pair-yielding iterable, and elemOf when every slot shares a non-empty leaf
// (so nested `for a in pair` / next(pair) type). Foreign slots shadow too.
func pythonBindPairLoopTarget(name string, types []string, pairSlots map[string][]string, elemOf map[string]string) {
	if name == "" || len(types) == 0 {
		return
	}
	if pairSlots != nil {
		pairSlots[name] = types
	}
	if et := pythonSharedSlotType(types); et != "" && elemOf != nil {
		elemOf[name] = et
	}
}

// pythonSharedSlotType returns the common non-empty leaf when every slot agrees;
// any "" or mismatch fails closed.
func pythonSharedSlotType(types []string) string {
	if len(types) == 0 {
		return ""
	}
	var et string
	for _, t := range types {
		if t == "" {
			return ""
		}
		if et == "" {
			et = t
			continue
		}
		if et != t {
			return ""
		}
	}
	return et
}

// pythonIterableElemType recovers the element type of a for/comprehension iterable.
// Uses known collection locals (elemOf), homogeneous Class() list/tuple/set literals,
// identity generator/list/set comprehensions (`x for x in items`),
// element-preserving wrappers (reversed/sorted/list/tuple/set/iter/deque/Counter),
// filter (2nd arg iterable; pred does not change element type),
// map when the first arg is a Class identifier (map(A, xs) → A),
// chain / itertools.chain (all args agree on element type),
// merge / heapq.merge (all args agree on element type; key/reverse ignored),
// chain.from_iterable / itertools.chain.from_iterable (flatten one level;
// arg is a list/tuple of iterables that agree on element type),
// islice / itertools.islice (1st arg iterable; start/stop/step ignored),
// accumulate / itertools.accumulate (1st arg iterable; func/initial ignored),
// cycle / itertools.cycle (1st arg iterable; repeats forever),
// repeat / itertools.repeat (1st arg object type; times ignored — yields the object),
// starmap / itertools.starmap when 1st arg is a Class identifier (like map),
// takewhile / dropwhile / filterfalse / itertools.* (2nd arg iterable; pred ignored),
// compress / itertools.compress (1st arg data; selectors ignored),
// nlargest / nsmallest / heapq.nlargest / heapq.nsmallest (2nd arg iterable; n/key ignored),
// choices / sample / random.choices / random.sample (1st arg population; k/weights ignored),
// Counter / collections.Counter (keys = iterable elements; .elements() same),
// dict.fromkeys(iterable[, value]) (keys = 1st-arg elements; value ignored here),
// items.copy() (zero-arg; same element type as receiver),
// items or [] / items or [A()] (boolean or/and when both sides agree; empty
// list/tuple/set is a wildcard), parenthesized forms, d.values() when d's dict
// value type is in elemOf, or e.exceptions when e was bound by except* Type as e
// (egElems). Does not type d.items() pairs (use unpack).
func pythonIterableElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if right == nil {
		return ""
	}
	switch right.Type() {
	case "identifier":
		if elemOf == nil {
			return ""
		}
		return elemOf[ingest.NodeText(right, content)]
	case "list", "tuple", "set":
		return pythonHomogeneousCtorElem(right, content)
	case "parenthesized_expression":
		// (items or []) / (xs.copy()) — unwrap and re-type the inner expression.
		return pythonIterableElemType(pythonParenInner(right), content, elemOf, egElems, typeOf)
	case "boolean_operator":
		// items or [] / items or [A()] / xs and ys — element type when sides agree.
		// Empty list/tuple/set is a wildcard (does not introduce a type).
		// Mismatched leaves (list[A] or [B()]) fail closed.
		left := ingest.ChildByField(right, "left")
		rightN := ingest.ChildByField(right, "right")
		return pythonBoolOpElemType(left, rightN, content, elemOf, egElems, typeOf)
	case "generator_expression", "list_comprehension", "set_comprehension":
		// next(x for x in items) / for a in [x for x in items] — identity body only.
		return pythonComprehensionElemType(right, content, elemOf, egElems, typeOf)
	case "call":
		// reversed(xs) / sorted(xs) / list(xs) / tuple(xs) / set(xs) /
		// frozenset(xs) / iter(xs) / deque(xs) / Counter(xs) — element type
		// equals the wrapped iterable. Nested wrappers recurse.
		// filter(pred, xs) — element type equals xs (pred only selects).
		// map(A, xs) — element type is A when first arg is a Class identifier;
		// other map callables fail closed (unknown result type).
		// chain(xs, ys) / islice(xs, n) — itertools helpers (bare or imported).
		if fn := ingest.ChildByField(right, "function"); fn != nil && fn.Type() == "identifier" {
			switch ingest.NodeText(fn, content) {
			case "reversed", "sorted", "list", "tuple", "set", "frozenset", "iter", "deque", "Counter":
				// Counter(iterable) keys are the iterable elements (product case).
				// frozenset(iterable) same as set (immutable). Mapping/kwargs
				// constructors fail closed when untyped / non-iterable.
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) == 0 {
					return ""
				}
				return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
			case "filter":
				// filter(function, iterable) — keep iterable's element type.
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) < 2 {
					return ""
				}
				return pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
			case "map", "starmap":
				// map(A, iterable) / starmap(A, pairs) — Class-as-callable yields A.
				// Other callables fail closed (unknown result type).
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) == 0 {
					return ""
				}
				if args[0].Type() == "identifier" {
					return ingest.NodeText(args[0], content)
				}
				return ""
			case "repeat":
				// repeat(object[, times]) from itertools — yields object forever/times.
				// Element type is the object expression type (typed local / Class()).
				// times (2nd arg / kwargs) ignored.
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) == 0 {
					return ""
				}
				return pythonObjectExprType(args[0], content, typeOf)
			case "chain":
				// chain(*iterables) from itertools — shared element type when
				// all args agree; any untyped or mismatched arg fails closed.
				return pythonChainElemType(right, content, elemOf, egElems, typeOf)
			case "merge":
				// merge(*iterables[, key, reverse]) from heapq — shared element
				// type when all positional args agree (same as chain); key/reverse
				// kwargs ignored.
				return pythonChainElemType(right, content, elemOf, egElems, typeOf)
			case "islice", "accumulate", "cycle", "compress":
				// islice(iterable, stop) / islice(iterable, start, stop[, step])
				// accumulate(iterable[, func, *, initial])
				// cycle(iterable)
				// compress(data, selectors)
				// — element type equals the iterable (1st arg).
				// func / initial kwargs or extra positional func are ignored
				// (same-leaf product case; type-changing fold fails closed only
				// when the 1st arg itself is untyped).
				// compress selectors (2nd arg) ignored — only filter which items yield.
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) == 0 {
					return ""
				}
				return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
			case "takewhile", "dropwhile", "filterfalse":
				// takewhile(pred, iterable) / dropwhile(pred, iterable) /
				// filterfalse(pred, iterable) — keep iterable's element type
				// (pred only selects; same as filter).
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) < 2 {
					return ""
				}
				return pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
			case "nlargest", "nsmallest":
				// nlargest(n, iterable[, key]) / nsmallest(n, iterable[, key])
				// from heapq — yields elements of iterable (2nd arg); n/key ignored.
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) < 2 {
					return ""
				}
				return pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
			case "choices", "sample":
				// choices(population, ...[, k=]) / sample(population, k) from random —
				// yield elements of population (1st arg); weights/k ignored.
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) == 0 {
					return ""
				}
				return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
			}
		}
		// items.copy() / list(items).copy() — zero-arg shallow copy preserves
		// the receiver's element type. copy with args fails closed.
		// Counter(...).elements() / c.elements() — yields Counter keys (same
		// element type as iterating the Counter). Zero-arg only.
		// d.values() — dict value type stored in elemOf[d].
		// dict.fromkeys(iterable[, value]) — iteration yields keys (1st arg elements).
		// itertools.chain(...) / itertools.islice(...) / itertools.accumulate(...) /
		// itertools.cycle(...) / itertools.compress(...) /
		// itertools.repeat(...) / itertools.starmap(...) /
		// itertools.takewhile/dropwhile/filterfalse(...) — same as bare helpers.
		// heapq.merge — same as bare merge (shared elem type across args).
		// heapq.nlargest / heapq.nsmallest — same as bare nlargest/nsmallest.
		// random.choices / random.sample — same as bare choices/sample.
		// collections.deque(xs) / collections.Counter(xs) — same as bare forms.
		if fn := ingest.ChildByField(right, "function"); fn != nil && fn.Type() == "attribute" {
			if attr := ingest.ChildByField(fn, "attribute"); attr != nil {
				switch ingest.NodeText(attr, content) {
				case "copy", "elements":
					// copy() and Counter.elements() — zero-arg; element type of receiver.
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) != 0 {
						return ""
					}
					obj := ingest.ChildByField(fn, "object")
					return pythonIterableElemType(obj, content, elemOf, egElems, typeOf)
				case "values":
					obj, method := pythonAttrCall(right, content)
					if obj == "" || method != "values" || elemOf == nil {
						return ""
					}
					return elemOf[obj]
				case "fromkeys":
					// dict.fromkeys(iterable[, value]) — keys are iterable elements.
					// value arg ignored for key iteration (value leaf handled on assign).
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" {
						return ""
					}
					if ingest.NodeText(objN, content) != "dict" {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
				case "chain", "islice", "accumulate", "cycle", "compress", "repeat", "starmap":
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" {
						return ""
					}
					if ingest.NodeText(objN, content) != "itertools" {
						return ""
					}
					switch ingest.NodeText(attr, content) {
					case "chain":
						return pythonChainElemType(right, content, elemOf, egElems, typeOf)
					case "repeat":
						// itertools.repeat(object[, times]) — object expression type.
						args, ok := pythonCallPositionalArgNodes(right)
						if !ok || len(args) == 0 {
							return ""
						}
						return pythonObjectExprType(args[0], content, typeOf)
					case "starmap":
						// itertools.starmap(A, pairs) — Class-as-callable yields A.
						args, ok := pythonCallPositionalArgNodes(right)
						if !ok || len(args) == 0 {
							return ""
						}
						if args[0].Type() == "identifier" {
							return ingest.NodeText(args[0], content)
						}
						return ""
					}
					// islice / accumulate / cycle / compress — element type of 1st arg.
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
				case "takewhile", "dropwhile", "filterfalse":
					// itertools.takewhile/dropwhile/filterfalse(pred, iterable)
					// — element type of 2nd positional arg (same as filter).
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" {
						return ""
					}
					if ingest.NodeText(objN, content) != "itertools" {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) < 2 {
						return ""
					}
					return pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
				case "merge":
					// heapq.merge(*iterables[, key, reverse]) — shared element type
					// when all positional args agree (same as chain); key/reverse ignored.
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" {
						return ""
					}
					if ingest.NodeText(objN, content) != "heapq" {
						return ""
					}
					return pythonChainElemType(right, content, elemOf, egElems, typeOf)
				case "nlargest", "nsmallest":
					// heapq.nlargest(n, iterable[, key]) / heapq.nsmallest(...)
					// — element type of 2nd positional arg (n/key ignored).
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" {
						return ""
					}
					if ingest.NodeText(objN, content) != "heapq" {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) < 2 {
						return ""
					}
					return pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
				case "choices", "sample":
					// random.choices(population, ...[, k=]) / random.sample(population, k)
					// — element type of 1st positional arg (weights/k ignored).
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" {
						return ""
					}
					if ingest.NodeText(objN, content) != "random" {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
				case "deque", "Counter":
					// collections.deque(iterable[, maxlen]) /
					// collections.Counter(iterable) — element of 1st arg.
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" {
						return ""
					}
					if ingest.NodeText(objN, content) != "collections" {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
				case "from_iterable":
					// chain.from_iterable(iterables) /
					// itertools.chain.from_iterable(iterables) — flatten one level.
					// Receiver must be bare chain or itertools.chain.
					objN := ingest.ChildByField(fn, "object")
					if !pythonIsChainReceiver(objN, content) {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok || len(args) != 1 {
						return ""
					}
					return pythonChainFromIterableElemType(args[0], content, elemOf, egElems, typeOf)
				}
			}
		}
		return ""
	case "attribute":
		// e.exceptions from except* A as e
		obj := ingest.ChildByField(right, "object")
		attr := ingest.ChildByField(right, "attribute")
		if obj == nil || attr == nil || obj.Type() != "identifier" {
			return ""
		}
		if ingest.NodeText(attr, content) != "exceptions" || egElems == nil {
			return ""
		}
		return egElems[ingest.NodeText(obj, content)]
	}
	return ""
}

// pythonParenInner returns the expression inside a parenthesized_expression,
// preferring the "expression" field and otherwise the first non-paren child.
func pythonParenInner(n *grammar.Node) *grammar.Node {
	if n == nil {
		return nil
	}
	if inner := ingest.ChildByField(n, "expression"); inner != nil {
		return inner
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch.Type() == "(" || ch.Type() == ")" {
			continue
		}
		return ch
	}
	return nil
}

// pythonChainElemType recovers the shared element type of chain(*iterables)
// (bare or itertools.chain). Every positional arg must resolve to the same
// non-empty element leaf; any untyped arg or mismatched leaves fails closed.
// chain.from_iterable is handled separately (pythonChainFromIterableElemType);
// *splat args fail closed (via call-arg parsing).
func pythonChainElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	var et string
	for _, arg := range args {
		t := pythonIterableElemType(arg, content, elemOf, egElems, typeOf)
		if t == "" {
			return ""
		}
		if et == "" {
			et = t
			continue
		}
		if et != t {
			return ""
		}
	}
	return et
}

// pythonIsChainReceiver reports whether n is bare `chain` or `itertools.chain`
// (the receiver of chain.from_iterable).
func pythonIsChainReceiver(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	switch n.Type() {
	case "identifier":
		return ingest.NodeText(n, content) == "chain"
	case "attribute":
		objN := ingest.ChildByField(n, "object")
		attrN := ingest.ChildByField(n, "attribute")
		if objN == nil || attrN == nil || objN.Type() != "identifier" {
			return false
		}
		return ingest.NodeText(objN, content) == "itertools" &&
			ingest.NodeText(attrN, content) == "chain"
	default:
		return false
	}
}

// pythonChainFromIterableElemType recovers the element type yielded by
// chain.from_iterable(iterables). The sole arg must be a list/tuple of
// iterables that share a non-empty element leaf (e.g. [items, more] with
// items: list[A], or [[A()], [A()]]). Bare identifiers fail closed (no
// nested container map for list[list[A]] params).
func pythonChainFromIterableElemType(arg *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if arg == nil {
		return ""
	}
	for arg != nil && arg.Type() == "parenthesized_expression" {
		arg = pythonParenInner(arg)
	}
	if arg == nil {
		return ""
	}
	switch arg.Type() {
	case "list", "tuple":
		// ok
	default:
		return ""
	}
	var et string
	saw := false
	for i := uint32(0); i < arg.ChildCount(); i++ {
		ch := arg.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		default:
			t := pythonIterableElemType(ch, content, elemOf, egElems, typeOf)
			if t == "" {
				return ""
			}
			if !saw {
				et = t
				saw = true
				continue
			}
			if et != t {
				return ""
			}
		}
	}
	if !saw {
		return ""
	}
	return et
}

// pythonBoolOpElemType recovers a shared element type for `a or b` / `a and b`.
// Both sides must agree when typed; empty list/tuple/set literals are wildcards.
// Unknown non-empty sides fail closed (result can be either operand).
func pythonBoolOpElemType(left, right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	etL := pythonIterableElemType(left, content, elemOf, egElems, typeOf)
	etR := pythonIterableElemType(right, content, elemOf, egElems, typeOf)
	emptyL := pythonIsEmptyCollectionLiteral(left)
	emptyR := pythonIsEmptyCollectionLiteral(right)
	if etL != "" && etR != "" {
		if etL == etR {
			return etL
		}
		return ""
	}
	if etL != "" && emptyR {
		return etL
	}
	if etR != "" && emptyL {
		return etR
	}
	return ""
}

// pythonIsEmptyCollectionLiteral reports [] / () / {} (set) with no elements.
// Parentheses are unwrapped. Non-collection nodes return false.
func pythonIsEmptyCollectionLiteral(n *grammar.Node) bool {
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return false
	}
	switch n.Type() {
	case "list", "tuple", "set":
		// ok
	default:
		return false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		switch n.Child(i).Type() {
		case "[", "]", "(", ")", "{", "}", ",", "comment":
			continue
		default:
			return false
		}
	}
	return true
}

// pythonAttrCall returns (objectIdent, methodName) for obj.method(...) calls.
func pythonAttrCall(call *grammar.Node, content []byte) (obj, method string) {
	if call == nil || call.Type() != "call" {
		return "", ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return "", ""
	}
	objN := ingest.ChildByField(fn, "object")
	attrN := ingest.ChildByField(fn, "attribute")
	if objN == nil || attrN == nil || objN.Type() != "identifier" {
		return "", ""
	}
	return ingest.NodeText(objN, content), ingest.NodeText(attrN, content)
}

// pythonDictItemsValueType returns the value type of d.items() when d is a known
// dict/mapping local (elemOf stores the value leaf from dict[K, V]).
func pythonDictItemsValueType(right *grammar.Node, content []byte, elemOf map[string]string) string {
	obj, method := pythonAttrCall(right, content)
	if obj == "" || method != "items" || elemOf == nil {
		return ""
	}
	return elemOf[obj]
}

// pythonDictPopitemValueType returns the value type of d.popitem() when d is a
// known dict/mapping local (elemOf stores the value leaf from dict[K, V]).
// popitem() yields a (key, value) pair — callers bind the value via unpack
// (k, a = d.popitem()) or pair subscript (a = d.popitem()[1]); the pair itself
// is not an element type.
func pythonDictPopitemValueType(right *grammar.Node, content []byte, elemOf map[string]string) string {
	obj, method := pythonAttrCall(right, content)
	if obj == "" || method != "popitem" || elemOf == nil {
		return ""
	}
	return elemOf[obj]
}

// pythonDictPopitemSubscriptValueType returns the value type of d.popitem()[1]
// when d is a known dict/mapping local. Index must be the integer literal 1
// (value slot of the (key, value) pair). [0] (key) and other indices fail closed.
// Parenthesized (d.popitem())[1] is accepted.
func pythonDictPopitemSubscriptValueType(sub *grammar.Node, content []byte, elemOf map[string]string) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	idx := ingest.ChildByField(sub, "subscript")
	if idx == nil || idx.Type() != "integer" || ingest.NodeText(idx, content) != "1" {
		return ""
	}
	val := ingest.ChildByField(sub, "value")
	for val != nil && val.Type() == "parenthesized_expression" {
		val = pythonParenInner(val)
	}
	return pythonDictPopitemValueType(val, content, elemOf)
}

// pythonBatchedElemType recovers the element type of batches from
// batched(iterable, n[, *]) / itertools.batched(...).
// Each batch is a tuple of n consecutive elements of the 1st positional arg
// (n and strict= ignored). Bare or itertools-qualified only; other forms fail closed.
// Not registered in pythonIterableElemType — bare `for x in batched(xs, n)` yields
// tuples, not elements. Callers bind the batch into elemOf for nested
// `for a in batch` / next(batch), or type every unpack slot as the element.
func pythonBatchedElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if right == nil || right.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "batched" {
			return ""
		}
	case "attribute":
		objN := ingest.ChildByField(fn, "object")
		attrN := ingest.ChildByField(fn, "attribute")
		if objN == nil || attrN == nil || objN.Type() != "identifier" {
			return ""
		}
		if ingest.NodeText(objN, content) != "itertools" {
			return ""
		}
		if ingest.NodeText(attrN, content) != "batched" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(right)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonGroupbyGroupElemType recovers the element type of the group iterator
// yielded by groupby(iterable[, key]) / itertools.groupby(...).
// Yields (key, group) pairs; group iterates elements of the 1st positional arg
// (key function ignored). Bare or itertools-qualified only; other forms fail closed.
// Not registered in pythonIterableElemType — bare `for x in groupby(xs)` yields
// pairs, not elements.
func pythonGroupbyGroupElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if right == nil || right.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "groupby" {
			return ""
		}
	case "attribute":
		objN := ingest.ChildByField(fn, "object")
		attrN := ingest.ChildByField(fn, "attribute")
		if objN == nil || attrN == nil || objN.Type() != "identifier" {
			return ""
		}
		if ingest.NodeText(objN, content) != "itertools" {
			return ""
		}
		if ingest.NodeText(attrN, content) != "groupby" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(right)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonTeeElemType recovers the element type of each independent iterator
// returned by tee(iterable[, n]) / itertools.tee(...).
// tee yields a tuple of n iterators (default n=2); each iterator yields elements
// of the 1st positional arg (n ignored). Bare or itertools-qualified only.
// Not registered in pythonIterableElemType — bare `for x in tee(xs)` yields
// iterators, not elements. Callers bind unpack targets into elemOf so nested
// `for a in it1` / next(it1) / it1.__next__() type.
func pythonTeeElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if right == nil || right.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "tee" {
			return ""
		}
	case "attribute":
		objN := ingest.ChildByField(fn, "object")
		attrN := ingest.ChildByField(fn, "attribute")
		if objN == nil || attrN == nil || objN.Type() != "identifier" {
			return ""
		}
		if ingest.NodeText(objN, content) != "itertools" {
			return ""
		}
		if ingest.NodeText(attrN, content) != "tee" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(right)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonEnumerateZipTargetTypes returns per-unpack-target element types for
// enumerate(xs) → ["", elem(xs)] (index untyped; value is the iterable element)
// and zip / zip_longest / product(a, b, ...) → [elem(a), elem(b), ...].
// Also zip(*[a, b]) / zip(*(a, b)) — single list/tuple-literal splat expands
// to the same per-slot typing (kwargs like strict= still ignored).
// zip_longest / product are accepted bare (from itertools import …) or as
// itertools.zip_longest / itertools.product; fillvalue/repeat kwargs ignored.
// pairwise(xs) / itertools.pairwise(xs) → [elem(xs), elem(xs)] (successive
// overlapping pairs; both slots share the iterable's element type).
// Unknown args yield "" slots; fails closed when the call is not a resolvable
// enumerate/zip/zip_longest/product/pairwise form.
func pythonEnumerateZipTargetTypes(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) []string {
	if right == nil || right.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil {
		return nil
	}
	var fname string
	switch fn.Type() {
	case "identifier":
		fname = ingest.NodeText(fn, content)
	case "attribute":
		// itertools.zip_longest/product/pairwise(...) only —
		// other module attrs fail closed.
		objN := ingest.ChildByField(fn, "object")
		attrN := ingest.ChildByField(fn, "attribute")
		if objN == nil || attrN == nil || objN.Type() != "identifier" {
			return nil
		}
		if ingest.NodeText(objN, content) != "itertools" {
			return nil
		}
		switch ingest.NodeText(attrN, content) {
		case "zip_longest", "pairwise", "product":
			fname = ingest.NodeText(attrN, content)
		default:
			return nil
		}
	default:
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(right)
	if !ok {
		// zip(*[xs, ys]) / zip(*(xs, ys)) — expand single list/tuple splat.
		if fname != "zip" && fname != "zip_longest" {
			return nil
		}
		args = pythonExpandSingleListTupleSplat(right)
		if len(args) == 0 {
			return nil
		}
	}
	switch fname {
	case "enumerate":
		if len(args) == 0 {
			return nil
		}
		et := pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
		if et == "" {
			return nil
		}
		// targets: index (untyped), element
		return []string{"", et}
	case "zip", "zip_longest", "product":
		// product(*iterables[, repeat]) — same per-slot typing as zip for
		// positional iterables; repeat= kwargs are ignored (fail closed only
		// when there are no positional iterables).
		if len(args) == 0 {
			return nil
		}
		out := make([]string, len(args))
		any := false
		for i, a := range args {
			et := pythonIterableElemType(a, content, elemOf, egElems, typeOf)
			out[i] = et
			if et != "" {
				any = true
			}
		}
		if !any {
			return nil
		}
		return out
	case "pairwise":
		// pairwise(iterable) — successive overlapping pairs; both unpack
		// slots are elements of the iterable (1st positional arg).
		if len(args) == 0 {
			return nil
		}
		et := pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
		if et == "" {
			return nil
		}
		return []string{et, et}
	default:
		return nil
	}
}

// pythonCombPermElemType recovers the shared element type for
// combinations/permutations/combinations_with_replacement unpack targets.
// Bare (from itertools import …) or itertools.*; 1st positional arg is the
// iterable; r and other args are ignored. Returns "" when not a resolvable form.
func pythonCombPermElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if right == nil || right.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil {
		return ""
	}
	var fname string
	switch fn.Type() {
	case "identifier":
		fname = ingest.NodeText(fn, content)
	case "attribute":
		objN := ingest.ChildByField(fn, "object")
		attrN := ingest.ChildByField(fn, "attribute")
		if objN == nil || attrN == nil || objN.Type() != "identifier" {
			return ""
		}
		if ingest.NodeText(objN, content) != "itertools" {
			return ""
		}
		fname = ingest.NodeText(attrN, content)
	default:
		return ""
	}
	switch fname {
	case "combinations", "permutations", "combinations_with_replacement":
		// ok
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(right)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonPairIterSharedElemType recovers the shared element type for single-
// target loops over zip / zip_longest / product / pairwise (bare or
// itertools.*; zip(*[xs, ys]) splat included). Every unpack slot from
// pythonEnumerateZipTargetTypes must be the same non-empty leaf — used to bind
// `for pair in zip(...):` into elemOf so nested `for a in pair` / next(pair)
// type. Heterogeneous or untyped slots (incl. enumerate's untyped index) fail
// closed. Not used for bare `for x in zip(...)` element typing — the loop var
// is a tuple, not an element.
func pythonPairIterSharedElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	return pythonSharedSlotType(pythonEnumerateZipTargetTypes(right, content, elemOf, egElems, typeOf))
}

// pythonExpandSingleListTupleSplat returns the elements of a sole *list/*tuple
// splat argument: zip(*[xs, ys]) / zip(*(xs, ys), strict=True). Mixed
// positionals+splat, multi-splat, non-literal splat, or **kwargs fail closed.
func pythonExpandSingleListTupleSplat(call *grammar.Node) []*grammar.Node {
	if call == nil || call.Type() != "call" {
		return nil
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil || argList.Type() != "argument_list" {
		return nil
	}
	var splat *grammar.Node
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment", "keyword_argument":
			continue
		case "list_splat", "parenthesized_list_splat":
			if splat != nil {
				return nil
			}
			splat = ch
		default:
			// Mixed positional + splat — fail closed.
			return nil
		}
	}
	if splat == nil {
		return nil
	}
	var inner *grammar.Node
	for i := uint32(0); i < splat.ChildCount(); i++ {
		ch := splat.Child(i)
		if ch.Type() == "*" || ch.Type() == "comment" {
			continue
		}
		inner = ch
		break
	}
	if inner == nil {
		return nil
	}
	switch inner.Type() {
	case "list", "tuple":
		// ok
	default:
		return nil
	}
	var out []*grammar.Node
	for i := uint32(0); i < inner.ChildCount(); i++ {
		ch := inner.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		default:
			out = append(out, ch)
		}
	}
	return out
}

// pythonDictFromkeysValueType returns the Class name of dict.fromkeys's value
// argument when it is a Class() call: dict.fromkeys(keys, A()) or
// dict.fromkeys(keys, value=A()). Used to seed elemOf for later .values/.get.
// Missing/non-Class value fails closed ("").
func pythonDictFromkeysValueType(right *grammar.Node, content []byte) string {
	if right == nil || right.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	objN := ingest.ChildByField(fn, "object")
	attrN := ingest.ChildByField(fn, "attribute")
	if objN == nil || attrN == nil || objN.Type() != "identifier" {
		return ""
	}
	if ingest.NodeText(objN, content) != "dict" || ingest.NodeText(attrN, content) != "fromkeys" {
		return ""
	}
	// 2nd positional Class() — dict.fromkeys(keys, A())
	if args, ok := pythonCallPositionalArgNodes(right); ok && len(args) >= 2 {
		if name := pythonClassCtorName(args[1], content); name != "" {
			return name
		}
	}
	// keyword value=Class() — dict.fromkeys(keys, value=A())
	argList := ingest.ChildByField(right, "arguments")
	if argList == nil {
		return ""
	}
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch.Type() != "keyword_argument" {
			continue
		}
		nameN := ingest.ChildByField(ch, "name")
		valN := ingest.ChildByField(ch, "value")
		if nameN == nil || valN == nil {
			continue
		}
		if ingest.NodeText(nameN, content) != "value" {
			continue
		}
		return pythonClassCtorName(valN, content)
	}
	return ""
}

// pythonClassCtorName returns T for a T() call (identifier callee only).
func pythonClassCtorName(n *grammar.Node, content []byte) string {
	if n == nil || n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "identifier" {
		return ""
	}
	return ingest.NodeText(fn, content)
}

// pythonObjectExprType recovers the Class leaf of an object expression used as a
// value (not an iterable): typed local identifier (item: A → "A"), Class() call
// (A() → "A"), or parenthesized form. Other shapes fail closed.
func pythonObjectExprType(n *grammar.Node, content []byte, typeOf map[string]string) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "identifier":
		if typeOf == nil {
			return ""
		}
		return typeOf[ingest.NodeText(n, content)]
	case "call":
		return pythonClassCtorName(n, content)
	case "parenthesized_expression":
		return pythonObjectExprType(pythonParenInner(n), content, typeOf)
	}
	return ""
}

// pythonCallPositionalArgNodes returns positional argument nodes of a call.
// Keyword arguments are skipped. Splat (*args / **kwargs) fails closed (ok=false).
// Bare generator expressions (`next(x for x in items)`) attach as the arguments
// field itself (not wrapped in argument_list) — returned as a single arg.
func pythonCallPositionalArgNodes(call *grammar.Node) (args []*grammar.Node, ok bool) {
	if call == nil || call.Type() != "call" {
		return nil, false
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return nil, true
	}
	// next(x for x in items) — tree-sitter puts the genexp in the arguments field
	// (no argument_list wrapper). Treat it as the sole positional arg.
	if argList.Type() == "generator_expression" {
		return []*grammar.Node{argList}, true
	}
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument":
			// enumerate(xs, start=1) / zip(a, b, strict=True) /
			// zip_longest(a, b, fillvalue=None) — ignore kwargs.
			continue
		case "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return nil, false
		default:
			args = append(args, ch)
		}
	}
	return args, true
}

// pythonComprehensionElemType recovers the element type of an identity
// generator/list/set comprehension: `x for x in items` / `[x for x in items if x]`.
// Body must be the same identifier as the single for-target; nested fors and
// transforming bodies (`f(x) for x in items`) fail closed. if_clause is ignored
// (filter does not change element type).
func pythonComprehensionElemType(comp *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if comp == nil {
		return ""
	}
	switch comp.Type() {
	case "generator_expression", "list_comprehension", "set_comprehension":
		// ok
	default:
		return ""
	}
	body := ingest.ChildByField(comp, "body")
	if body == nil || body.Type() != "identifier" {
		return ""
	}
	var forClause *grammar.Node
	forCount := 0
	for i := uint32(0); i < comp.ChildCount(); i++ {
		if ch := comp.Child(i); ch.Type() == "for_in_clause" {
			forCount++
			if forClause == nil {
				forClause = ch
			}
		}
	}
	// Nested `for a in xs for b in ys` — fail closed (yield type is ambiguous here).
	if forCount != 1 || forClause == nil {
		return ""
	}
	left := ingest.ChildByField(forClause, "left")
	right := ingest.ChildByField(forClause, "right")
	if left == nil || right == nil || left.Type() != "identifier" {
		return ""
	}
	// Identity only: body name must match the for-target (`x for x in items`).
	if ingest.NodeText(body, content) != ingest.NodeText(left, content) {
		return ""
	}
	return pythonIterableElemType(right, content, elemOf, egElems, typeOf)
}

// pythonPatternIdents returns simple identifier targets from pattern_list /
// tuple_pattern / list_pattern: a, b / (a, b) / [a, b]. Fail closed on nested
// or starred patterns (use pythonUnpackFixedAndStar for *rest).
func pythonPatternIdents(n *grammar.Node, content []byte) []string {
	fixed, star, ok := pythonUnpackFixedAndStar(n, content)
	if !ok || star != "" {
		return nil
	}
	return fixed
}

// pythonUnpackFixedAndStar returns non-star identifier targets and optional
// star-bound name from pattern_list / tuple_pattern / list_pattern.
// Supports assignment star unpack (`a, *rest` / `*rest, a`) via
// list_splat_pattern. Fails closed on other nested patterns or multiple stars.
func pythonUnpackFixedAndStar(n *grammar.Node, content []byte) (fixed []string, star string, ok bool) {
	if n == nil {
		return nil, "", false
	}
	switch n.Type() {
	case "pattern_list", "tuple_pattern", "list_pattern":
		// ok
	default:
		return nil, "", false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "(", ")", "[", "]", ",", "comment":
			continue
		case "identifier":
			fixed = append(fixed, ingest.NodeText(ch, content))
		case "list_splat_pattern":
			// *rest — at most one star; binds a sequence, not an element.
			if star != "" {
				return nil, "", false
			}
			id := ingest.ChildByType(ch, "identifier")
			if id == nil {
				return nil, "", false
			}
			star = ingest.NodeText(id, content)
		default:
			// Nested patterns — fail closed.
			return nil, "", false
		}
	}
	if len(fixed) == 0 && star == "" {
		return nil, "", false
	}
	return fixed, star, true
}

// pythonCtorListTypes returns Class leaves for A(), B() expression_list / tuple
// / list rows used in unpack assignment (a, b = A(), B() / [a, b] = [A(), B()]).
func pythonCtorListTypes(n *grammar.Node, content []byte) []string {
	if n == nil {
		return nil
	}
	switch n.Type() {
	case "expression_list", "tuple", "list":
		// ok
	default:
		return nil
	}
	var out []string
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "(", ")", "[", "]", ",", "comment":
			continue
		case "call":
			fn := ingest.ChildByField(ch, "function")
			if fn == nil || fn.Type() != "identifier" {
				return nil
			}
			out = append(out, ingest.NodeText(fn, content))
		default:
			return nil
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pythonHomogeneousPairCtorTypes recovers position-wise Class() types from a
// list/tuple/set of homogeneous pairs: [(A(), B()), (A(), B())] → ["A","B"].
func pythonHomogeneousPairCtorTypes(collection *grammar.Node, content []byte) []string {
	if collection == nil {
		return nil
	}
	switch collection.Type() {
	case "list", "tuple", "set":
		// ok
	default:
		return nil
	}
	var row []string
	saw := false
	for i := uint32(0); i < collection.ChildCount(); i++ {
		ch := collection.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", "{", "}", ",", "comment":
			continue
		case "tuple", "expression_list":
			types := pythonCtorListTypes(ch, content)
			if len(types) == 0 {
				return nil
			}
			if !saw {
				row = types
				saw = true
				continue
			}
			if len(types) != len(row) {
				return nil
			}
			for j := range row {
				if types[j] != row[j] {
					return nil
				}
			}
		default:
			return nil
		}
	}
	if !saw {
		return nil
	}
	return row
}

// pythonHomogeneousCtorElem returns T when collection is only T() calls (same T).
func pythonHomogeneousCtorElem(collection *grammar.Node, content []byte) string {
	if collection == nil {
		return ""
	}
	switch collection.Type() {
	case "list", "tuple", "set":
		// ok
	default:
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < collection.ChildCount(); i++ {
		ch := collection.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", "{", "}", ",", "comment":
			continue
		case "call":
			fn := ingest.ChildByField(ch, "function")
			if fn == nil || fn.Type() != "identifier" {
				return ""
			}
			name := ingest.NodeText(fn, content)
			if !saw {
				elem = name
				saw = true
				continue
			}
			if name != elem {
				return ""
			}
		default:
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// pythonContainerElemType extracts the element type leaf from container annotations:
// list[A], List[A], Iterable[A], set[A], tuple[A, ...], dict[K, A] (value).
// Returns "" when the annotation is not a resolvable single-element container.
func pythonContainerElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	if typeN.Type() == "type" && typeN.ChildCount() > 0 {
		typeN = typeN.Child(0)
	}
	// Quoted: "list[A]" / 'dict[str, A]'
	if typeN.Type() == "string" {
		s := strings.Trim(ingest.NodeText(typeN, content), `"'`)
		return pythonParseContainerElemString(s)
	}
	if typeN.Type() != "generic_type" {
		return ""
	}
	// generic_type: identifier + type_parameter
	var contName string
	var typeParam *grammar.Node
	for i := uint32(0); i < typeN.ChildCount(); i++ {
		ch := typeN.Child(i)
		switch ch.Type() {
		case "identifier":
			if contName == "" {
				contName = ingest.NodeText(ch, content)
			}
		case "attribute":
			// typing.List — use leaf
			if contName == "" {
				if attr := ingest.ChildByField(ch, "attribute"); attr != nil {
					contName = ingest.NodeText(attr, content)
				}
			}
		case "type_parameter":
			typeParam = ch
		}
	}
	if typeParam == nil {
		return ""
	}
	var args []string
	for i := uint32(0); i < typeParam.ChildCount(); i++ {
		ch := typeParam.Child(i)
		if ch.Type() != "type" {
			continue
		}
		// Skip ellipsis in tuple[A, ...]
		inner := ch
		if ch.ChildCount() == 1 && ch.Child(0).Type() == "ellipsis" {
			continue
		}
		if ch.ChildCount() > 0 {
			inner = ch.Child(0)
		}
		if inner.Type() == "ellipsis" {
			continue
		}
		if tn := pythonTypeName(ch, content); tn != "" {
			args = append(args, tn)
		}
	}
	if len(args) == 0 {
		return ""
	}
	switch contName {
	case "dict", "Dict", "Mapping", "MutableMapping", "OrderedDict", "defaultdict", "DefaultDict":
		// value type is last type arg when there are two.
		if len(args) == 2 {
			return args[1]
		}
		return ""
	case "list", "List", "set", "Set", "frozenset", "FrozenSet",
		"tuple", "Tuple", "Iterable", "Iterator", "Sequence", "MutableSequence",
		"Collection", "Container", "Generator", "AsyncIterable", "AsyncIterator",
		"AbstractSet", "MutableSet":
		// Single element type, or homogeneous multi (tuple[A, A]) — first only when all agree.
		if len(args) == 1 {
			return args[0]
		}
		first := args[0]
		for _, a := range args[1:] {
			if a != first {
				return ""
			}
		}
		return first
	default:
		// Unknown generic: only when exactly one type arg (deque[A], etc.).
		if len(args) == 1 {
			return args[0]
		}
		return ""
	}
}

// pythonParseContainerElemString handles quoted annotations like "list[A]".
func pythonParseContainerElemString(s string) string {
	s = strings.TrimSpace(s)
	lb := strings.IndexByte(s, '[')
	rb := strings.LastIndexByte(s, ']')
	if lb <= 0 || rb <= lb {
		return ""
	}
	contName := strings.TrimSpace(s[:lb])
	if i := strings.LastIndexByte(contName, '.'); i >= 0 {
		contName = contName[i+1:]
	}
	inner := strings.TrimSpace(s[lb+1 : rb])
	// Strip trailing ", ..." for tuple[A, ...]
	parts := strings.Split(inner, ",")
	var args []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "..." {
			continue
		}
		// Take simple identifier / dotted leaf only.
		if strings.ContainsAny(p, "[]()|") {
			return ""
		}
		if i := strings.LastIndexByte(p, '.'); i >= 0 {
			p = p[i+1:]
		}
		args = append(args, p)
	}
	if len(args) == 0 {
		return ""
	}
	switch contName {
	case "dict", "Dict", "Mapping", "MutableMapping", "OrderedDict", "defaultdict", "DefaultDict":
		if len(args) == 2 {
			return args[1]
		}
		return ""
	default:
		if len(args) == 1 {
			return args[0]
		}
		first := args[0]
		for _, a := range args[1:] {
			if a != first {
				return ""
			}
		}
		return first
	}
}

// pythonAsPatternBinding extracts (alias, typeName) from an as_pattern node.
// typeName is the simple class leaf when the pattern is a class/call/identifier
// we can resolve (A, A(), pkg.A, class_pattern A()).
func pythonAsPatternBinding(n *grammar.Node, content []byte) (name, typ string) {
	if n == nil || n.Type() != "as_pattern" {
		return "", ""
	}
	var left *grammar.Node
	var alias *grammar.Node
	seenAs := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch.Type() == "as" {
			seenAs = true
			continue
		}
		if !seenAs {
			left = ch
			continue
		}
		switch ch.Type() {
		case "as_pattern_target":
			// with/except: alias field wraps identifier
			if id := ingest.ChildByType(ch, "identifier"); id != nil {
				alias = id
			}
		case "identifier":
			// match case: bare identifier after as
			alias = ch
		}
	}
	if left == nil || alias == nil {
		return "", ""
	}
	return ingest.NodeText(alias, content), pythonPatternTypeName(left, content)
}

// pythonPatternTypeName recovers a simple type leaf from match/with/except patterns.
func pythonPatternTypeName(n *grammar.Node, content []byte) string {
	if n == nil || n.IsNull() {
		return ""
	}
	switch n.Type() {
	case "identifier":
		return ingest.NodeText(n, content)
	case "call":
		fn := ingest.ChildByField(n, "function")
		if fn == nil {
			return ""
		}
		if fn.Type() == "identifier" {
			return ingest.NodeText(fn, content)
		}
		if fn.Type() == "attribute" {
			if attr := ingest.ChildByField(fn, "attribute"); attr != nil {
				return ingest.NodeText(attr, content)
			}
		}
		return ""
	case "class_pattern":
		if dn := ingest.ChildByType(n, "dotted_name"); dn != nil {
			return pythonDottedNameLeaf(dn, content)
		}
		if id := ingest.ChildByType(n, "identifier"); id != nil {
			return ingest.NodeText(id, content)
		}
	case "case_pattern", "pattern":
		for i := uint32(0); i < n.ChildCount(); i++ {
			if t := pythonPatternTypeName(n.Child(i), content); t != "" {
				return t
			}
		}
	case "attribute":
		if attr := ingest.ChildByField(n, "attribute"); attr != nil {
			return ingest.NodeText(attr, content)
		}
	case "dotted_name":
		return pythonDottedNameLeaf(n, content)
	}
	return ""
}

// pythonDottedNameLeaf returns the last identifier of a dotted_name (pkg.A → A).
func pythonDottedNameLeaf(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	var last string
	var walk func(x *grammar.Node)
	walk = func(x *grammar.Node) {
		if x == nil || x.IsNull() {
			return
		}
		if x.Type() == "identifier" {
			last = ingest.NodeText(x, content)
			return
		}
		for i := uint32(0); i < x.ChildCount(); i++ {
			walk(x.Child(i))
		}
	}
	walk(n)
	return last
}

// pythonTypeName extracts a simple class name from a type annotation node.
// Unwraps Optional[T], Union[T, None]/Union[None, T], and T | None / None | T
// to T so annotated params/locals participate in ExtraRename. Multi-arm unions
// with more than one non-None type fail closed ("").
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
	case "generic_type":
		// Optional[T] / Union[T, None] (and typing.Optional / typing.Union via leaf).
		return pythonOptionalUnionType(typeN, content)
	case "binary_operator":
		// T | None / None | T (PEP 604). Multi non-None arms fail closed.
		return pythonPipeUnionType(typeN, content)
	}
	return ""
}

// pythonOptionalUnionType handles Optional[T] and Union[..., None, ...].
func pythonOptionalUnionType(typeN *grammar.Node, content []byte) string {
	if typeN == nil || typeN.Type() != "generic_type" {
		return ""
	}
	var contName string
	var typeParam *grammar.Node
	for i := uint32(0); i < typeN.ChildCount(); i++ {
		ch := typeN.Child(i)
		switch ch.Type() {
		case "identifier":
			if contName == "" {
				contName = ingest.NodeText(ch, content)
			}
		case "attribute":
			// typing.Optional — use leaf
			if contName == "" {
				if attr := ingest.ChildByField(ch, "attribute"); attr != nil {
					contName = ingest.NodeText(attr, content)
				}
			}
		case "type_parameter":
			typeParam = ch
		}
	}
	if typeParam == nil {
		return ""
	}
	var args []string
	for i := uint32(0); i < typeParam.ChildCount(); i++ {
		ch := typeParam.Child(i)
		if ch.Type() != "type" {
			continue
		}
		if pythonIsNoneTypeNode(ch, content) {
			continue
		}
		if tn := pythonTypeName(ch, content); tn != "" && tn != "None" {
			args = append(args, tn)
		}
	}
	switch contName {
	case "Optional":
		// Optional[T] — single type arg (None is implicit).
		if len(args) == 1 {
			return args[0]
		}
		return ""
	case "Union":
		// Union[T, None] / Union[None, T] → T; multi non-None fails closed.
		if len(args) == 1 {
			return args[0]
		}
		return ""
	default:
		return ""
	}
}

// pythonPipeUnionType handles PEP 604 unions: T | None / None | T / T | U | None.
// Exactly one non-None arm → that type; otherwise fail closed.
func pythonPipeUnionType(n *grammar.Node, content []byte) string {
	arms := pythonPipeUnionArms(n, content)
	if arms == nil {
		return ""
	}
	var nonNone []string
	for _, a := range arms {
		if a == "" || a == "None" {
			continue
		}
		nonNone = append(nonNone, a)
	}
	if len(nonNone) == 1 {
		return nonNone[0]
	}
	return ""
}

// pythonPipeUnionArms flattens a | chain into type-name leaves; None arms are "".
// Returns nil if any arm is not a resolvable type / None (fail closed).
func pythonPipeUnionArms(n *grammar.Node, content []byte) []string {
	if n == nil {
		return nil
	}
	if n.Type() == "type" && n.ChildCount() > 0 {
		n = n.Child(0)
	}
	if n.Type() == "binary_operator" {
		op := ingest.ChildByField(n, "operator")
		if op == nil || ingest.NodeText(op, content) != "|" {
			return nil
		}
		left := pythonPipeUnionArms(ingest.ChildByField(n, "left"), content)
		right := pythonPipeUnionArms(ingest.ChildByField(n, "right"), content)
		if left == nil || right == nil {
			return nil
		}
		return append(left, right...)
	}
	if pythonIsNoneTypeNode(n, content) {
		return []string{""}
	}
	if tn := pythonTypeName(n, content); tn != "" {
		return []string{tn}
	}
	return nil
}

// pythonIsNoneTypeNode reports None / none in type position.
func pythonIsNoneTypeNode(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	if n.Type() == "type" && n.ChildCount() > 0 {
		n = n.Child(0)
	}
	switch n.Type() {
	case "none":
		return true
	case "identifier":
		return ingest.NodeText(n, content) == "None"
	}
	return false
}

// pythonCastTypeArg returns the type leaf of cast(T, x) / typing.cast(T, x)
// first argument (identifier, attribute leaf, or string).
func pythonCastTypeArg(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	args := ingest.ChildByField(call, "arguments")
	if args == nil {
		return ""
	}
	// First non-punctuation child is the type expression.
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "identifier":
			return ingest.NodeText(ch, content)
		case "attribute":
			if attr := ingest.ChildByField(ch, "attribute"); attr != nil {
				return ingest.NodeText(attr, content)
			}
			return ""
		case "string":
			return strings.Trim(ingest.NodeText(ch, content), `"'`)
		default:
			// Unexpected shape — fail closed.
			return ""
		}
	}
	return ""
}
