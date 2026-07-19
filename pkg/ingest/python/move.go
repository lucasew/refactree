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
	// elemOf maps collection locals → element type leaf for direct access chains
	// (items.popleft().run() / d.get(k).run() under foreign same-leaf methods).
	// typeOf maps object locals → type leaf (item: A → "A"; foreign too for shadowing)
	// so direct copy.copy(item).run() can resolve without assignment form.
	// pairSlots maps pair locals → per-slot types (p = next(...items()); p[1].run()).
	// factoryOf maps partial factory locals → class leaf (pa = partial(A); pa().run()).
	// futureOf maps Future locals → result class leaf (fa.set_result(A()); fa.result().run()).
	typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns := pythonTypedLocals(pf.Root, content, ourReceivers)

	var edits []ingest.Edit
	// mc = methodcaller("run"); mc(A()) — rename name string when all applications
	// type as our receiver (foreign/unknown apps fail closed). Inline
	// methodcaller("run")(A()) stays on pythonMethodcallerStringEdits below.
	if mcStored := pythonStoredMethodcallerStringEdits(fileRel, pf.Root, content, oldLeaf, newLeaf, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns); len(mcStored) > 0 {
		edits = append(edits, mcStored...)
	}
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
				if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
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
					if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
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
					if pythonShouldRenameAttr(obj, content, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
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
			// methodcaller("run")(A()) / operator.methodcaller("run")(A()) —
			// method name string when the applied target is our receiver type.
			// Foreign same-leaf applications keep the string (B.run preserved).
			if mcEdits := pythonMethodcallerStringEdits(fileRel, n, content, oldLeaf, newLeaf, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns); len(mcEdits) > 0 {
				edits = append(edits, mcEdits...)
			}
			// getattr(A(), "run") / getattr(A(), "run")() — method name string when
			// the object arg types as our receiver. Foreign same-leaf keeps "run".
			if gaEdits := pythonGetattrMethodStringEdits(fileRel, n, content, oldLeaf, newLeaf, classHere, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns); len(gaEdits) > 0 {
				edits = append(edits, gaEdits...)
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

// pythonGetattrMethodStringEdits rewrites the attr name string on
// getattr(target, "old") when target types as our receiver (Class() / typed
// local). Enables getattr(A(), "run")() / fa = getattr(A(), "run") under foreign
// same-leaf methods — getattr(B(), "run") keeps "run". Three-arg getattr
// (default) still renames the name when the object is ours. Non-builtin getattr
// / non-string names fail closed.
func pythonGetattrMethodStringEdits(fileRel string, call *grammar.Node, content []byte, oldLeaf, newLeaf, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, fieldOf, elemOf, typeOf map[string]string, pairSlots map[string][]string, factoryOf, futureOf, getterOf, funcReturns map[string]string) []ingest.Edit {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" || ingest.NodeText(fn, content) != "getattr" {
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 2 || args[1].Type() != "string" {
		return nil
	}
	contentN, text := pythonStringContent(args[1], content)
	if contentN == nil || text != oldLeaf {
		return nil
	}
	// Object arg must type as our receiver (same gate as methodcaller target).
	if !pythonShouldRenameAttr(args[0], content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
		return nil
	}
	return []ingest.Edit{{
		File:      fileRel,
		StartByte: contentN.StartByte(),
		EndByte:   contentN.EndByte(),
		NewText:   newLeaf,
	}}
}

// pythonMethodcallerStringEdits rewrites the method name string on
// methodcaller("old")(target) / operator.methodcaller("old")(target) when
// target is one of our receivers (Class() / typed local / factory peel).
// Under foreign same-leaf methods, only applications whose target types as our
// leaf rename the string — methodcaller("run")(B()) keeps "run".
// Multi-arg methodcaller (extra bound args) still renames the name string when
// the applied target is ours. Stored getters (mc = methodcaller("run"); mc(a))
// are handled by pythonStoredMethodcallerStringEdits (application-typed).
func pythonMethodcallerStringEdits(fileRel string, call *grammar.Node, content []byte, oldLeaf, newLeaf, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, fieldOf, elemOf, typeOf map[string]string, pairSlots map[string][]string, factoryOf, futureOf, getterOf, funcReturns map[string]string) []ingest.Edit {
	if call == nil || call.Type() != "call" {
		return nil
	}
	// Outer call's function is methodcaller(...) / operator.methodcaller(...).
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "call" {
		return nil
	}
	contentN := pythonMethodcallerNameContent(fn, content, oldLeaf)
	if contentN == nil {
		return nil
	}
	// Applied target (first positional of outer call) must type as our receiver.
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 1 {
		return nil
	}
	if !pythonShouldRenameAttr(args[0], content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
		return nil
	}
	return []ingest.Edit{{
		File:      fileRel,
		StartByte: contentN.StartByte(),
		EndByte:   contentN.EndByte(),
		NewText:   newLeaf,
	}}
}

// pythonMethodcallerNameContent returns the string content node of the method
// name when call is methodcaller("oldLeaf") / operator.methodcaller("oldLeaf")
// (extra bound args allowed). Non-methodcaller / non-matching names fail closed.
func pythonMethodcallerNameContent(call *grammar.Node, content []byte, oldLeaf string) *grammar.Node {
	if call == nil || call.Type() != "call" {
		return nil
	}
	mcFn := ingest.ChildByField(call, "function")
	if mcFn == nil {
		return nil
	}
	switch mcFn.Type() {
	case "identifier":
		if ingest.NodeText(mcFn, content) != "methodcaller" {
			return nil
		}
	case "attribute":
		attr := ingest.ChildByField(mcFn, "attribute")
		obj := ingest.ChildByField(mcFn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "methodcaller" {
			return nil
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "operator" {
			return nil
		}
	default:
		return nil
	}
	nameStr := pythonFirstStringArg(ingest.ChildByField(call, "arguments"))
	if nameStr == nil {
		return nil
	}
	contentN, text := pythonStringContent(nameStr, content)
	if contentN == nil || text != oldLeaf {
		return nil
	}
	return contentN
}

// pythonStoredMethodcallerStringEdits renames method name strings on
// mc = methodcaller("old") / operator.methodcaller("old") when every application
// mc(target) in the same function/module scope types as our receiver. Mixed
// (mc(A())+mc(B())), foreign-only, or unknown targets fail closed so B.run is
// preserved. Each function_definition is its own scope so same local names in
// different functions do not clobber each other. Inline methodcaller("old")(A())
// stays on pythonMethodcallerStringEdits.
func pythonStoredMethodcallerStringEdits(fileRel string, root *grammar.Node, content []byte, oldLeaf, newLeaf string, ourReceivers, foreignReceivers, typedLocals map[string]bool, fieldOf, elemOf, typeOf map[string]string, pairSlots map[string][]string, factoryOf, futureOf, getterOf, funcReturns map[string]string) []ingest.Edit {
	if root == nil || oldLeaf == "" {
		return nil
	}
	type mcBind struct {
		nameNode *grammar.Node
		ourApp   bool
		badApp   bool // foreign or unknown application
	}
	var edits []ingest.Edit
	seen := map[uint32]bool{}
	// processScope collects methodcaller locals and applications under scope,
	// without descending into nested function_definition bodies (those get their
	// own processScope call). classHere is the enclosing class for self peels.
	processScope := func(scope *grammar.Node, classHere string) {
		if scope == nil || scope.IsNull() {
			return
		}
		binds := map[string]*mcBind{}
		var walk func(n *grammar.Node, class string)
		walk = func(n *grammar.Node, class string) {
			if n == nil || n.IsNull() {
				return
			}
			// Nested functions are separate scopes — skip body here.
			if n != scope && n.Type() == "function_definition" {
				return
			}
			classNow := class
			if n.Type() == "class_definition" {
				if nameN := ingest.ChildByField(n, "name"); nameN != nil {
					classNow = ingest.NodeText(nameN, content)
				}
			}
			if n.Type() == "assignment" {
				left := ingest.ChildByField(n, "left")
				right := ingest.ChildByField(n, "right")
				if left != nil && left.Type() == "identifier" && right != nil {
					if contentN := pythonMethodcallerNameContent(right, content, oldLeaf); contentN != nil {
						binds[ingest.NodeText(left, content)] = &mcBind{nameNode: contentN}
					}
				}
			}
			if n.Type() == "call" {
				fn := ingest.ChildByField(n, "function")
				if fn != nil && fn.Type() == "identifier" {
					if b := binds[ingest.NodeText(fn, content)]; b != nil {
						args, ok := pythonCallPositionalArgNodes(n)
						if !ok || len(args) < 1 {
							b.badApp = true
						} else if pythonShouldRenameAttr(args[0], content, classNow, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
							b.ourApp = true
						} else {
							b.badApp = true
						}
					}
				}
			}
			for i := uint32(0); i < n.ChildCount(); i++ {
				walk(n.Child(i), classNow)
			}
		}
		walk(scope, classHere)
		for _, b := range binds {
			if b == nil || b.nameNode == nil || !b.ourApp || b.badApp {
				continue
			}
			start := b.nameNode.StartByte()
			if seen[start] {
				continue
			}
			seen[start] = true
			edits = append(edits, ingest.Edit{
				File:      fileRel,
				StartByte: start,
				EndByte:   b.nameNode.EndByte(),
				NewText:   newLeaf,
			})
		}
	}
	// Module scope (top-level statements) + each function body.
	processScope(root, "")
	var walkFuncs func(n *grammar.Node, classHere string)
	walkFuncs = func(n *grammar.Node, classHere string) {
		if n == nil || n.IsNull() {
			return
		}
		classNow := classHere
		if n.Type() == "class_definition" {
			if nameN := ingest.ChildByField(n, "name"); nameN != nil {
				classNow = ingest.NodeText(nameN, content)
			}
		}
		if n.Type() == "function_definition" {
			processScope(n, classNow)
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walkFuncs(n.Child(i), classNow)
		}
	}
	walkFuncs(root, "")
	return edits
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
// elemOf maps collection locals → element type leaf for direct access chains
// (items.popleft().run() / d.get(k).run() / items[0].run() under foreign same-leaf methods).
// typeOf maps object locals → type leaf (item: A → "A") for direct copy.copy(item).run()
// and similar identity wrappers that need the arg's class leaf under foreign same-leaf.
// factoryOf maps partial factory locals → class leaf (pa = partial(A); pa().run()).
// futureOf maps Future locals → result class leaf (fa.set_result(A()); fa.result().run()).
func pythonShouldRenameAttr(obj *grammar.Node, content []byte, enclosingClass string, ourReceivers, foreignReceivers, typedLocals map[string]bool, fieldOf, elemOf, typeOf map[string]string, pairSlots map[string][]string, factoryOf, futureOf, getterOf, funcReturns map[string]string) bool {
	if obj == nil {
		return false
	}
	// Peel (expr) / (name := expr) so (ba.get()).run() / (a := ba.get()).run()
	// type under foreign same-leaf (assigned if (a := ba.get()): a.run already works).
	for obj != nil && !obj.IsNull() {
		switch obj.Type() {
		case "parenthesized_expression":
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
				return false
			}
			obj = inner
		case "named_expression":
			// (a := ba.get()).run() — walrus value is the expression type.
			valueN := ingest.ChildByField(obj, "value")
			if valueN == nil {
				return false
			}
			obj = valueN
		default:
			goto peeled
		}
	}
peeled:
	if obj == nil || obj.IsNull() {
		return false
	}
	// super().method(): rewrite when renaming Base.m in Child; leave alone when renaming Child.m.
	if pythonIsSuperCall(obj, content) {
		return enclosingClass == "" || !ourReceivers[enclosingClass]
	}
	// Box().method / make().method — ctor name via maps.
	// box.get("a").run() — TypedDict/record string-key value (fieldOf; same leaf as
	// xa = box.get("a"); xa.run()).
	// asdict(box).get("a").run() / vars(box).get("a").run() /
	// box.__dict__.get("a").run() — dict-view field keys (same leaf as
	// d = asdict(box); d.get("a").run() / asdict(box)["a"].run()).
	// getattr(box, "a").run() — builtin field access (same leaf as box.a.run()).
	// attrgetter("a")(box).run() / attrgetter("a")(replace(box)).run() — single-field
	// getter (same leaf as box.a / replace(box).a).
	// ga(box).run() after ga = attrgetter("a") — stored field getter (same leaf).
	// itemgetter("a")(box).run() / itemgetter("a")(asdict(box)).run() — single
	// string-key getter (same leaf as box["a"] / asdict(box)["a"]).
	// itemgetter(0)(items).run() / operator.itemgetter(0)(items).run() — collection
	// element via single-index getter (same leaf as a = itemgetter(0)(items); a.run()).
	// gi(items).run() after gi = itemgetter(0) — stored element getter (same leaf).
	// copy.copy(asdict(box)["a"]).run() / copy.deepcopy(vars(box)["a"]).run() —
	// object copy of a dict-view field key (same leaf as asdict(box)["a"].run()).
	// copy.copy(box.a).run() / copy.copy(item).run() — field / typed-local arg
	// (same leaf as box.a.run() / a = copy.copy(item); a.run()).
	// next(iter(items)).run() / next(items).run() — iterable element (same leaf as
	// a = next(iter(items)); a.run()). Default arg ignored (same as assignment).
	// next(iter(astuple(box))).run() — first declaration-order field of the
	// heterogeneous field tuple (same leaf as astuple(box)[0].run()).
	// min(items).run() / max(asdict(pair).values()).run() / min(astuple(pair)).run() —
	// element of iterable / homogeneous values view (same leaf as a = min(...); a.run()).
	// choice(items).run() / random.choice(items).run() — sequence element (same leaf
	// as a = choice(items); a.run()).
	// heappop(items).run() / heapq.heappushpop(items, x).run() /
	// heapreplace(items, x).run() — heap element (same leaf as a = heappop(...); a.run()).
	// reduce(fn, items).run() / functools.reduce(fn, items, init).run() — fold result
	// typed as iterable element (same leaf as a = reduce(...); a.run()).
	// partial(A)().run() / functools.partial(A)().run() — factory call result is A
	// (same leaf as Class() ctor under foreign same-leaf methods).
	// pa().run() after pa = partial(A) / functools.partial(A) — factory local
	// call result is A (same leaf as partial(A)().run()).
	// ex.submit(lambda: A()).result().run() — Future result is A (Callable lambda
	// body Class(); same leaf as Java ExecutorService.submit(() -> new A()).get()).
	// items.popleft().run() / d.get(k).run() / q.get().run() / items.pop().run() /
	// list(items).pop().run() / items.__getitem__(i).run() — collection/queue
	// element accessors (same leaf as a = items.popleft(); a.run()).
	// getitem(items, i).run() / operator.getitem(items, i).run() — same leaf as
	// items[i] (bare from operator / module-qualified).
	// Unknown call receivers: unique-leaf only.
	if obj.Type() == "call" {
		if fn := ingest.ChildByField(obj, "function"); fn != nil {
			// getattr(box, "a") before bare-ident ctor path (function is identifier).
			if ft := pythonGetattrFieldType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			// copy.copy(item).run() / copy.copy(box.a).run() /
			// copy.copy(asdict(box)["a"]).run() / copy.deepcopy(...) —
			// copy.copy(ba.get()).run() — preserve object type of the single arg
			// (typed local / field / dict-view key / method-return).
			if ft := pythonCopyCallObjectType(obj, content, typeOf, fieldOf, funcReturns); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			// replace(box).run() / dataclasses.replace(box).run() /
			// replace(ba.get()).run() / replace(A()).run() — same object type as
			// first positional arg (method-return under foreign same-leaf).
			if ft := pythonReplaceCallObjectType(obj, content, typeOf, fieldOf, funcReturns); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			// weakref.proxy(a).run() / weakref.ref(a)().run() /
			// weakref.proxy(ba.get()).run() — identity of referent (method-return too).
			if ft := pythonWeakrefCallObjectType(obj, content, typeOf, funcReturns, fieldOf); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			// cast(A, x).run() / typing.cast(A, x).run() — type-arg peel under foreign same-leaf.
			if ft := pythonCastCallType(obj, content); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			// partial(A)().run() / functools.partial(A)().run() — before Class() path.
			if et := pythonPartialCallResultType(obj, content); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// partial(ba.get)().run() / functools.partial(ba.get)() — bound-method
			// partial under foreign same-leaf (Class peels via partial(A) above).
			if et := pythonPartialBoundMethodCallResultType(obj, content, funcReturns, typeOf, fieldOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// pa().run() after pa = partial(A) — factory local call yields A.
			if et := pythonPartialFactoryLocalResultType(obj, content, factoryOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// ga(box).run() after ga = attrgetter("a") /
			// gi(items).run() after gi = itemgetter(0) — stored operator getter.
			// Also gi([ba.get()]) / ga(w.get()) / ga(Box(A())) under foreign same-leaf.
			if ft := pythonStoredOperatorGetterType(obj, content, getterOf, fieldOf, elemOf, nil, typeOf, funcReturns); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			// ex.submit(lambda: A()).result().run() / fa.result().run() after
			// fa.set_result(A()) — Future.result peel.
			if et := pythonFutureResultCallType(obj, content, futureOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			if fn.Type() == "identifier" {
				name := ingest.NodeText(fn, content)
				// next(iter(items)).run() / next(items).run() / next(reversed(items)).run()
				// — before Class() ctor path (function is bare "next", not a class).
				// Element type of the iterable arg (same path as assignment binding).
				// next(iter(astuple(box))).run() — first field of heterogeneous
				// astuple (not a homogeneous collection elemOf).
				if name == "next" {
					if et := pythonNextElemType(obj, content, elemOf, nil, typeOf); et != "" {
						return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
					}
					if et := pythonAstupleNextFirstField(obj, content, fieldOf); et != "" {
						return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
					}
					// next(iter([ba.get()])) / next(dict(k=ba.get()).values()) —
					// method-return collection / object-dict values under foreign
					// same-leaf (pythonNextElemType only peels Class()/typed locals).
					if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) > 0 {
						if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); et != "" {
							return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
						}
					}
				}
				// min(...).run() / max(...).run() — before Class() ctor path.
				// Covers min(items) / max(d.values()) / min(asdict(...).values()) /
				// max(astuple(...)) when element type is known (assignment path
				// already binds via pythonMinMaxElemType).
				if name == "min" || name == "max" {
					if et := pythonMinMaxElemType(obj, content, elemOf, nil, typeOf, fieldOf); et != "" {
						return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
					}
					// min([ba.get()], key=...) / max(dict(k=ba.get()).values()) —
					// method-return collection / object-dict values under foreign
					// same-leaf (single positional; kwargs ignored).
					if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) == 1 {
						if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); et != "" {
							return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
						}
					}
				}
				// choice(seq).run() — before Class() ctor path (bare from random).
				if name == "choice" {
					if et := pythonRandomChoiceElemType(obj, content, elemOf, nil, typeOf); et != "" {
						return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
					}
					// choice([ba.get()]) — method-return sequence under foreign same-leaf.
					if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) > 0 {
						if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); et != "" {
							return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
						}
					}
				}
				// heappop(heap).run() / heappushpop(...).run() / heapreplace(...).run()
				// — before Class() ctor path (bare from heapq).
				if name == "heappop" || name == "heappushpop" || name == "heapreplace" {
					if et := pythonHeappopElemType(obj, content, elemOf, nil, typeOf); et != "" {
						return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
					}
					// heappop([ba.get()]) — method-return heap under foreign same-leaf.
					if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) > 0 {
						if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); et != "" {
							return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
						}
					}
				}
				// reduce(fn, iterable[, init]).run() — before Class() ctor path
				// (bare from functools; fold result is iterable element).
				if name == "reduce" {
					if et := pythonReduceElemType(obj, content, elemOf, nil, typeOf); et != "" {
						return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
					}
					// reduce(fn, [ba.get()]) — method-return iterable under foreign same-leaf.
					if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) >= 2 {
						if et := pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf); et != "" {
							return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
						}
					}
				}
				// getitem(items, i).run() — before Class() ctor path (bare from operator).
				if name == "getitem" {
					if et := pythonGetitemElemType(obj, content, elemOf, nil, typeOf); et != "" {
						return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
					}
					// getitem([ba.get()], 0) — method-return collection under foreign same-leaf.
					if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) >= 1 {
						if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); et != "" {
							return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
						}
					}
				}
				// make_a().run() after def make_a() -> A / def make_a(): return A() /
				// @lru_cache def make_a() -> A / make_a = lambda: A() — same-file
				// factory return (before Class() ctor path).
				if rt := funcReturns[name]; rt != "" {
					return pythonRenameByTypeMaps(rt, ourReceivers, foreignReceivers, nil)
				}
				return pythonRenameByTypeMaps(name, ourReceivers, foreignReceivers, nil)
			}
			// A.make().run() / A.create().run() — @staticmethod / @classmethod
			// factory recorded as "A.make" / "A.create" in funcReturns.
			// ba.get().run() — instance method with annotated return (BoxA.get → A).
			if fn.Type() == "attribute" {
				if rt := pythonCallFuncReturnType(obj, content, funcReturns, typeOf, fieldOf); rt != "" {
					return pythonRenameByTypeMaps(rt, ourReceivers, foreignReceivers, nil)
				}
			}
			if ft := pythonRecordKeyAccessType(obj, content, fieldOf); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			if ft := pythonDictViewKeyAccessType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			if ft := pythonAttrgetterFieldType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			if ft := pythonItemgetterFieldType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
				return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
			}
			// itemgetter(0)(items).run() / operator.itemgetter(0)(list(items)).run() —
			// single-index getter on a known collection (same leaf as assignment).
			// Multi-index / string-key itemgetter use field path or fail closed.
			if et := pythonItemgetterElemType(obj, content, elemOf, nil, typeOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// itemgetter(0)([ba.get()]).run() / itemgetter("k")({"k": ba.get()}).run() —
			// method-return collection / object-dict under foreign same-leaf.
			if et := pythonItemgetterObjectElemType(obj, content, funcReturns, typeOf, fieldOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// getitem(items, i).run() / operator.getitem(items, i).run() —
			// same element leaf as items[i] / a = getitem(items, i); a.run().
			if et := pythonGetitemElemType(obj, content, elemOf, nil, typeOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// operator.getitem([ba.get()], 0) — method-return collection under foreign same-leaf.
			if fn := ingest.ChildByField(obj, "function"); fn != nil {
				isGetitem := false
				if fn.Type() == "identifier" && ingest.NodeText(fn, content) == "getitem" {
					isGetitem = true
				} else if fn.Type() == "attribute" {
					if attr := ingest.ChildByField(fn, "attribute"); attr != nil && ingest.NodeText(attr, content) == "getitem" {
						if o := ingest.ChildByField(fn, "object"); o != nil && o.Type() == "identifier" && ingest.NodeText(o, content) == "operator" {
							isGetitem = true
						}
					}
				}
				if isGetitem {
					if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) >= 1 {
						if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); et != "" {
							return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
						}
					}
				}
			}
			// random.choice(seq).run() — module-qualified form (function is attribute).
			if et := pythonRandomChoiceElemType(obj, content, elemOf, nil, typeOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// heapq.heappop(heap).run() / heapq.heappushpop(...).run() /
			// heapq.heapreplace(...).run() — module-qualified form.
			if et := pythonHeappopElemType(obj, content, elemOf, nil, typeOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// functools.reduce(fn, iterable[, init]).run() — module-qualified form.
			if et := pythonReduceElemType(obj, content, elemOf, nil, typeOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			// random.choice([ba.get()]) / heapq.heappop([ba.get()]) /
			// functools.reduce(fn, [ba.get()]) — method-return collections under
			// foreign same-leaf (helpers above only peel Class()/typed locals).
			if fn := ingest.ChildByField(obj, "function"); fn != nil && fn.Type() == "attribute" {
				if attr := ingest.ChildByField(fn, "attribute"); attr != nil {
					switch ingest.NodeText(attr, content) {
					case "choice", "heappop", "heappushpop", "heapreplace":
						if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) > 0 {
							if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); et != "" {
								return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
							}
						}
					case "reduce":
						if args, ok := pythonCallPositionalArgNodes(obj); ok && len(args) >= 2 {
							if et := pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf); et != "" {
								return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
							}
						}
					}
				}
			}
			if et := pythonCollectionAccessElemType(obj, content, elemOf, funcReturns, typeOf, fieldOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
		}
		return len(foreignReceivers) == 0
	}
	if obj.Type() == "identifier" {
		name := ingest.NodeText(obj, content)
		if name == "self" || name == "cls" {
			return pythonRenameByTypeMaps(enclosingClass, ourReceivers, foreignReceivers, nil)
		}
		return pythonRenameByTypeMaps(name, ourReceivers, foreignReceivers, typedLocals)
	}
	// box.a.run() — dataclass/class field access when box is a typed local.
	// replace(box).a.run() / dataclasses.replace(box).a.run() — same field leaf
	// as box.a (return type of replace is the dataclass of its first arg).
	// ba._replace(...).a.run() / Box._make([A()]).a.run() — namedtuple peels.
	// ba.self().item.run() — method-return peel then property/field.
	if obj.Type() == "attribute" {
		if ft := pythonFieldAccessType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		if ft := pythonReplaceFieldAccessType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		if ft := pythonNamedtupleReplaceFieldAccessType(obj, content, fieldOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		if ft := pythonNamedtupleMakeFieldAccessType(obj, content); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		// SimpleNamespace(k=ba.get()).k / types.SimpleNamespace(k=A()).k —
		// inline SNS kwargs (Class() or method-return) under foreign same-leaf.
		if ft := pythonInlineSimpleNamespaceObjectFieldType(obj, content, funcReturns, typeOf, fieldOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		return len(foreignReceivers) == 0
	}
	// box["a"].run() — TypedDict/record string-key value (fieldOf).
	// asdict(box)["a"].run() / vars(box)["a"].run() / box.__dict__["a"].run() —
	// dict-view field keys of first arg / object (same leaf as d = asdict(box);
	// d["a"].run()).
	// ba._asdict()["a"].run() / vars(SimpleNamespace(k=A()))["k"].run() —
	// namedtuple / inline SNS dict-view peels.
	// astuple(box)[0].run() / dataclasses.astuple(box)[0].run() /
	// astuple(replace(box))[0].run() / list(astuple(box))[0].run() /
	// tuple(astuple(box))[0].run() — declaration-order index slots of first arg
	// (same leaf as t = astuple(box); t[0].run(); not box[0]).
	// list(asdict(box).values())[i].run() / list(d.values())[i].run() after
	// d = asdict(box) / vars / __dict__ — same declaration-order slots (dict
	// preserves order; values()[i] is field i — same leaf as next(...values())).
	// list(asdict(box).items())[i][1].run() / list(d.items())[i][1].run() —
	// items pair value at declaration-order index i (same leaf as values()[i];
	// deep stack: asdict→items→list→[i]→[1]).
	// p[1].run() after p = next(...items()) / next(pairs) / min(...items()) —
	// pair local value slot (pairSlots; same leaf as a = p[1]; a.run()).
	// items[0].run() / d["k"].run() / list(items)[0].run() — collection subscript
	// element (same leaf as a = items[0]; a.run()). Slices fail closed.
	// Box._make([A()])[0].run() — namedtuple make sequence element peel.
	if obj.Type() == "subscript" {
		if ft := pythonRecordKeyAccessType(obj, content, fieldOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		if ft := pythonDictViewKeyAccessType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		if ft := pythonAstupleIndexAccessType(obj, content, fieldOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		// box[0].run() — namedtuple positional field (fieldOf["box.#0"]).
		if ft := pythonNamedtupleIndexType(obj, content, fieldOf); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		// Box._make([A()])[0].run() — make sequence element peel.
		if ft := pythonNamedtupleMakeIndexType(obj, content); ft != "" {
			return pythonRenameByTypeMaps(ft, ourReceivers, foreignReceivers, nil)
		}
		// pairSlots enables p[1].run() for assigned pair locals; typeOf for
		// collection wrappers that need object typing. pairIterSlots stays nil
		// (direct next(pairs)[i].run() still covered via assignment path).
		if et := pythonSubscriptElemType(obj, content, elemOf, nil, typeOf, pairSlots, nil, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// list(enumerate([ba.get()]))[0][1].run() / list(zip([ba.get()], …))[0][0].run() /
		// next(enumerate([ba.get()]))[1].run() — object pair-slot subscript under
		// foreign same-leaf (Class peels via pythonSubscriptElemType / pairIterSlots).
		if et := pythonObjectPairSlotSubscriptType(obj, content, funcReturns, typeOf, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// [ba.get()][0].run() / (ba.get(),)[0].run() / {"k": ba.get()}["k"].run() /
		// dict(k=ba.get())["k"].run() / ([ba.get()] + [ba.get()])[0].run() /
		// ([ba.get()] * 2)[0].run() / [*[ba.get()]][0].run() /
		// list([ba.get()])[0].run() / ([ba.get()] or [ba.get()])[0].run() /
		// list({"k": ba.get()}.values())[0].run() / [ba.get()][:][0].run() /
		// [ba.get()].copy()[0].run() / UserList([ba.get()])[0].run() /
		// ({"k": ba.get()} | {"j": ba.get()})["j"].run() —
		// method-return / typed-local collection peels under foreign same-leaf
		// (assigned forms peel via elemOf after bind).
		val := ingest.ChildByField(obj, "value")
		hasSlice := false
		for i := uint32(0); i < obj.ChildCount(); i++ {
			if obj.Child(i).Type() == "slice" {
				hasSlice = true
				break
			}
		}
		if hasSlice {
			// [ba.get()][:][0] — slice preserves object-collection element type
			// (Class() peels via pythonIterableElemType / pythonSubscriptElemType).
			if et := pythonObjectCollectionElem(val, content, funcReturns, typeOf, fieldOf); et != "" {
				return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
			}
			return len(foreignReceivers) == 0
		}
		if et := pythonObjectCollectionElem(val, content, funcReturns, typeOf, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// list({"k": ba.get()}.values())[0] / list(dict(k=ba.get()).values())[0] —
		// object-dict values via iterable peels (collection peels miss .values()).
		if et := pythonObjectIterableElemType(val, content, funcReturns, typeOf, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// {"k": ba.get()}["k"].run() — dict value peel (kept separate from
		// pythonObjectCollectionElem so assign of {"k": frozenset([A()])}
		// still hits nested peels before scalar dict binding).
		if et := pythonHomogeneousObjectDictValue(val, content, funcReturns, typeOf, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// dict.fromkeys(keys, ba.get())["k"] — fromkeys value leaf.
		if et := pythonDictFromkeysObjectValueType(val, content, funcReturns, typeOf, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// MappingProxyType({"k": ba.get()})["k"] — proxy of object-dict value.
		if et := pythonMappingProxyObjectValueType(val, content, elemOf, funcReturns, typeOf, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		// ({"k": ba.get()} | {"j": ba.get()})["j"] — object-dict merge value.
		if et := pythonObjectDictMergeValueType(val, content, funcReturns, typeOf, fieldOf); et != "" {
			return pythonRenameByTypeMaps(et, ourReceivers, foreignReceivers, nil)
		}
		return len(foreignReceivers) == 0
	}
	// (a if c else x).run() — both arms agree on Class leaf (typed local / Class()).
	// (ba.get() if c else ba.get()).run() — method-return arms under foreign
	// same-leaf (pythonConditionalExprType only peels local/Class()/nested).
	// Infer early-return only on positive our-type; otherwise both arms must
	// rename as ours (disagree / foreign fail closed). Mirrors Java/JS ternary.
	if obj.Type() == "conditional_expression" {
		if t := pythonConditionalExprType(obj, content, typeOf); t != "" &&
			pythonRenameByTypeMaps(t, ourReceivers, foreignReceivers, nil) {
			return true
		}
		body, alt := pythonConditionalArms(obj)
		if body != nil && alt != nil &&
			pythonShouldRenameAttr(body, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) &&
			pythonShouldRenameAttr(alt, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
			return true
		}
		return false
	}
	// await qa.get() — peel through await so Queue/collection accessors type.
	if obj.Type() == "await" {
		inner := pythonAwaitArg(obj)
		if inner != nil {
			return pythonShouldRenameAttr(inner, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns)
		}
		return len(foreignReceivers) == 0
	}
	// (ba.get() or ba.get()).run() / (a and a).run() — both arms rename as ours
	// under foreign same-leaf (nested or/and recurse). Mixed / None fail closed.
	if obj.Type() == "boolean_operator" {
		left := ingest.ChildByField(obj, "left")
		rightN := ingest.ChildByField(obj, "right")
		if left != nil && rightN != nil &&
			pythonShouldRenameAttr(left, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) &&
			pythonShouldRenameAttr(rightN, content, enclosingClass, ourReceivers, foreignReceivers, typedLocals, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns) {
			return true
		}
		return false
	}
	// Complex receivers without static type: unique-leaf only.
	if obj.Type() == "binary_operator" {
		return len(foreignReceivers) == 0
	}
	return false
}

// pythonAwaitArg returns the expression under await (await qa.get() → qa.get()).
// Missing arg → nil.
func pythonAwaitArg(n *grammar.Node) *grammar.Node {
	if n == nil || n.Type() != "await" {
		return nil
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil || ch.Type() == "await" {
			continue
		}
		return ch
	}
	return nil
}

// pythonCollectionAccessElemType recovers the element type of a collection/
// queue accessor call used as a method receiver:
//
//	items.popleft().run() / items.pop().run() / items.pop(0).run()
//	d.get(k).run() / d.setdefault(k).run() / q.get().run()
//	it.__next__().run() / items.__getitem__(i).run() / list(items).pop().run()
//	{"k": ba.get()}.get("k").run() / dict(k=ba.get()).get("k").run() —
//	inline method-return / Class() dict value peels under foreign same-leaf
//
// Same methods as the assignment path in pythonTypedLocals. Other methods and
// untyped receivers fail closed ("").
func pythonCollectionAccessElemType(call *grammar.Node, content []byte, elemOf, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil {
		return ""
	}
	switch ingest.NodeText(attr, content) {
	case "pop", "popleft", "get", "get_nowait", "setdefault", "__next__", "__getitem__":
		// Element/value type of the receiver collection (default args ignored).
		// __getitem__(i) is the same leaf as items[i] / d[k].
		// get_nowait is asyncio.Queue / queue.Queue non-blocking get (same E as get).
		obj := ingest.ChildByField(fn, "object")
		if et := pythonIterableElemType(obj, content, elemOf, nil, typeOf); et != "" {
			return et
		}
		// deque([ba.get()]).popleft() / [ba.get()].pop() / list([ba.get()]).pop() —
		// method-return object collections under foreign same-leaf (Class()-only
		// peels via pythonIterableElemType; assigned forms peel via elemOf).
		if et := pythonObjectIterableElemType(obj, content, funcReturns, typeOf, fieldOf); et != "" {
			return et
		}
		// {"k": ba.get()}.get("k") / dict(k=ba.get()).get("k") /
		// {"k": ba.get()}.__getitem__("k") / {"k": A()}.get("k") /
		// dict(k=A()).get("k") / ({"k": ba.get()} | {}).get("k") —
		// inline mapping value peels when the receiver is not a typed local
		// (assigned forms peel via elemOf after bind).
		// popleft/__next__ on non-dict shapes fail closed here.
		switch ingest.NodeText(attr, content) {
		case "get", "setdefault", "pop", "__getitem__":
			if et := pythonHomogeneousObjectDictValue(obj, content, funcReturns, typeOf, fieldOf); et != "" {
				return et
			}
			if et := pythonObjectDictMergeValueType(obj, content, funcReturns, typeOf, fieldOf); et != "" {
				return et
			}
			if et := pythonMappingProxyObjectValueType(obj, content, elemOf, funcReturns, typeOf, fieldOf); et != "" {
				return et
			}
			if et := pythonHomogeneousDictValueCtorElem(obj, content); et != "" {
				return et
			}
			if et := pythonDictMergeValueType(obj, content, elemOf); et != "" {
				return et
			}
		}
		return ""
	}
	return ""
}

// pythonPartialCallResultType recovers T from partial(T)(...) / functools.partial(T)(...):
// the outer call applies a partial factory whose first positional arg is a class
// identifier. Enables partial(A)().run() under foreign same-leaf methods.
// Extra partial args (bound constructor kwargs) are ignored; non-class first
// args and non-partial factories fail closed.
func pythonPartialCallResultType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	// Outer call's function is the partial(...) factory call.
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "call" {
		return ""
	}
	return pythonPartialFactoryClassType(fn, content)
}

// pythonPartialFactoryLocalResultType recovers T from pa() when pa is a local
// bound to partial(T) / functools.partial(T) via factoryOf. Enables
// pa = partial(A); pa().run() under foreign same-leaf methods. Unknown locals
// and non-factory identifiers fail closed.
func pythonPartialFactoryLocalResultType(call *grammar.Node, content []byte, factoryOf map[string]string) string {
	if call == nil || call.Type() != "call" || factoryOf == nil {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" {
		return ""
	}
	return factoryOf[ingest.NodeText(fn, content)]
}

// pythonPartialFactoryClassType recovers T from partial(T[, ...]) /
// functools.partial(T[, ...]) when the first positional arg is an identifier
// class leaf. Used by partial(A)().run() peels.
func pythonPartialFactoryClassType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "partial" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "partial" {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "functools" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 1 || args[0].Type() != "identifier" {
		return ""
	}
	return ingest.NodeText(args[0], content)
}

// pythonFutureResultCallType recovers T from fut.result() when fut is
// executor.submit(lambda: T()) / submit(lambda: T()) with an expression-bodied
// zero-arg lambda whose body is a Class() call, or when fut is a local bound via
// futureOf from fut.set_result(T()). Enables
// ex.submit(lambda: A()).result().run() and fa.set_result(A()); fa.result().run()
// under foreign same-leaf methods. Timeout args on result() are ignored;
// non-submit / non-set_result receivers fail closed.
func pythonFutureResultCallType(call *grammar.Node, content []byte, futureOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "result" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if et := pythonSubmitCallableResultType(obj, content); et != "" {
		return et
	}
	// fa.result() after fa.set_result(A()) — local Future result leaf.
	if obj != nil && obj.Type() == "identifier" && futureOf != nil {
		return futureOf[ingest.NodeText(obj, content)]
	}
	return ""
}

// pythonSimpleNamespaceFieldTypes recovers field→Class leaves from
// SimpleNamespace(k=A(), m=B()) / types.SimpleNamespace(...). Keyword args only;
// each value must be a Class() call. Splat / positional / non-Class values fail
// closed (nil). Used to bind fieldOf["da.k"] for da.k.run() under foreign
// same-leaf without inventing namedtuple fieldIndex entries.
// order is declaration-order Class leaves for @astuple.local.#i slots so
// vars(da).values() / d = vars(da); d.values() peels when values are uniform.
func pythonSimpleNamespaceFieldTypes(call *grammar.Node, content []byte) (map[string]string, []string) {
	return pythonSimpleNamespaceObjectFieldTypes(call, content, nil, nil, nil)
}

// pythonSimpleNamespaceObjectFieldTypes peels SNS kwargs as Class() or
// method-return / typed-local leaves (k=ba.get() → A) under foreign same-leaf.
func pythonSimpleNamespaceObjectFieldTypes(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) (map[string]string, []string) {
	if call == nil || call.Type() != "call" {
		return nil, nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return nil, nil
	}
	if pythonSimpleCalleeName(fn, content) != "SimpleNamespace" {
		return nil, nil
	}
	// types.SimpleNamespace — require module ident "types". Bare SimpleNamespace
	// (from types import SimpleNamespace) accepted by leaf name alone.
	if fn.Type() == "attribute" {
		obj := ingest.ChildByField(fn, "object")
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "types" {
			return nil, nil
		}
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return nil, nil
	}
	out := map[string]string{}
	var order []string
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument":
			nameN := ingest.ChildByField(ch, "name")
			valN := ingest.ChildByField(ch, "value")
			if nameN == nil || nameN.Type() != "identifier" {
				return nil, nil
			}
			et := pythonObjectLeafType(valN, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return nil, nil
			}
			out[ingest.NodeText(nameN, content)] = et
			order = append(order, et)
		default:
			// Positional / splat — fail closed (product kwargs-only form).
			return nil, nil
		}
	}
	if len(order) == 0 {
		return nil, nil
	}
	return out, order
}

// pythonCopyLocalFieldOf copies fieldOf entries from src local to dst local:
// named keys src.f → dst.f and @astuple.src.#i → @astuple.dst.#i. Used when
// d = vars(src) / d = src.__dict__ for SimpleNamespace (instance fieldOf without
// typeOf / fieldIndex). Empty / identical locals fail closed.
func pythonCopyLocalFieldOf(src, dst string, fieldOf map[string]string) {
	if src == "" || dst == "" || fieldOf == nil || src == dst {
		return
	}
	namedPrefix := src + "."
	astuplePrefix := "@astuple." + src + "."
	type kv struct{ k, v string }
	var adds []kv
	for k, v := range fieldOf {
		if v == "" {
			continue
		}
		switch {
		case strings.HasPrefix(k, astuplePrefix):
			adds = append(adds, kv{"@astuple." + dst + "." + k[len(astuplePrefix):], v})
		case strings.HasPrefix(k, namedPrefix):
			adds = append(adds, kv{dst + "." + k[len(namedPrefix):], v})
		}
	}
	for _, p := range adds {
		fieldOf[p.k] = p.v
	}
}

// pythonBindSimpleNamespaceLocal records fieldOf["local.f"] and declaration-order
// @astuple.local.#i slots from SimpleNamespace(...) / types.SimpleNamespace(...).
func pythonBindSimpleNamespaceLocal(local string, call *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string) {
	if local == "" || fieldOf == nil {
		return
	}
	fields, order := pythonSimpleNamespaceObjectFieldTypes(call, content, funcReturns, typeOf, fieldOf)
	if len(fields) == 0 {
		return
	}
	for f, t := range fields {
		if f != "" && t != "" {
			fieldOf[local+"."+f] = t
		}
	}
	for i, t := range order {
		if t != "" {
			fieldOf["@astuple."+local+".#"+fmt.Sprintf("%d", i)] = t
		}
	}
}

// pythonCollectionMutationElemType recovers (local, T, nested) from list/deque/
// set/mapping-bucket mutations that insert a Class() or method-return instance:
//
//	xs.append(A()) / xs.extend([A()]) / xs.insert(0, A()) → (xs, A, false)
//	xs.append(ba.get()) / xs.extend([ba.get()]) / xs.insert(0, ba.get())
//	xs.add(A()) / deque.extendleft([A()]) → (xs, A, false)
//	xs.add(ba.get()) / deque.append(ba.get()) — method-return inserts
//	da["k"].append(A()) / da.get("k").extend([A()]) / da["k"].insert(0, A())
//	  → (da, A, true)  // @nested leaf for mapping-of-list
//
// Enables bare list/deque/set mutation peels (xs=[]; xs.append(A()); xs[0].run()
// / xs=set(); xs.add(A()); next(iter(xs)).run()) and defaultdict(list)
// extend/insert peels under foreign same-leaf. Non-Class/method-return args,
// heterogeneous extend collections, and non-ident/non-mapping receivers fail closed.
func pythonCollectionMutationElemType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) (local, classType string, nested bool) {
	if call == nil || call.Type() != "call" {
		return "", "", false
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return "", "", false
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil {
		return "", "", false
	}
	method := ingest.NodeText(attr, content)
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok {
		return "", "", false
	}
	var et string
	switch method {
	case "append", "appendleft", "add":
		// xs.append(A()) / xs.append(ba.get()) / deque.appendleft(A()) /
		// set.add(A()) / set.add(ba.get()) — single Class()/method-return leaf.
		if len(args) != 1 {
			return "", "", false
		}
		et = pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf)
	case "extend", "extendleft", "update":
		// xs.extend([A()]) / xs.extend([ba.get()]) / xs.extend((A(),)) /
		// deque.extendleft([A()]) — homogeneous Class()/method-return collection.
		// ca.update([A()]) / s.update({A()}) — Counter/set keys from Class() elems
		// (product dual-class Counter peels under foreign same-leaf).
		if len(args) != 1 {
			return "", "", false
		}
		// Object leaves first (covers Class() + method-return); Class()-only
		// fallback when object maps unavailable.
		et = pythonHomogeneousObjectElem(args[0], content, funcReturns, typeOf, fieldOf)
		if et == "" {
			et = pythonHomogeneousCtorElem(args[0], content)
		}
	case "insert":
		// xs.insert(i, A()) / xs.insert(i, ba.get()) — second positional peels
		// to Class leaf; index shape free.
		if len(args) != 2 {
			return "", "", false
		}
		et = pythonObjectLeafType(args[1], content, funcReturns, typeOf, fieldOf)
	default:
		return "", "", false
	}
	if et == "" {
		return "", "", false
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return "", "", false
	}
	switch obj.Type() {
	case "identifier":
		// xs.append(A()) / deque().append path after xs = deque().
		return ingest.NodeText(obj, content), et, false
	case "subscript":
		// da["k"].append(A()) — non-slice key only.
		for i := uint32(0); i < obj.ChildCount(); i++ {
			if obj.Child(i).Type() == "slice" {
				return "", "", false
			}
		}
		val := ingest.ChildByField(obj, "value")
		if val == nil || val.Type() != "identifier" {
			return "", "", false
		}
		return ingest.NodeText(val, content), et, true
	case "call":
		// da.get("k").append(A()) / da.setdefault("k").extend([A()]).
		objFn := ingest.ChildByField(obj, "function")
		if objFn == nil || objFn.Type() != "attribute" {
			return "", "", false
		}
		objAttr := ingest.ChildByField(objFn, "attribute")
		if objAttr == nil {
			return "", "", false
		}
		switch ingest.NodeText(objAttr, content) {
		case "get", "setdefault":
			// ok
		default:
			return "", "", false
		}
		recv := ingest.ChildByField(objFn, "object")
		if recv == nil || recv.Type() != "identifier" {
			return "", "", false
		}
		return ingest.NodeText(recv, content), et, true
	default:
		return "", "", false
	}
}

// pythonDictMutationValueType recovers (local, T) from unannotated mapping
// mutations that insert a Class() or method-return value:
//
//	da.setdefault("k", A()) — 2nd positional is Class()
//	da.setdefault("k", ba.get()) — 2nd positional is zero-arg method return
//	da.update({"k": A()}) / da.update(k=A(), j=A()) — homogeneous Class values
//
// Enables da["k"].run() / a = da.setdefault(...) / da.setdefault(...).run()
// under foreign same-leaf after da = {}. Non-ident receivers / mixed values /
// non-Class fail closed. set.update([A()]) is handled by
// pythonCollectionMutationElemType (set elems).
func pythonDictMutationValueType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) (local, classType string) {
	if call == nil || call.Type() != "call" {
		return "", ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return "", ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil || obj.Type() != "identifier" {
		return "", ""
	}
	method := ingest.NodeText(attr, content)
	switch method {
	case "setdefault":
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) < 2 {
			return "", ""
		}
		// Class() first, then method-return / typed-local leaf (ba.get() → A).
		et := pythonObjectLeafType(args[1], content, funcReturns, typeOf, fieldOf)
		if et == "" {
			return "", ""
		}
		return ingest.NodeText(obj, content), et
	case "update":
		et := pythonDictUpdateValueClassType(call, content, funcReturns, typeOf, fieldOf)
		if et == "" {
			return "", ""
		}
		return ingest.NodeText(obj, content), et
	default:
		return "", ""
	}
}

// pythonDictUpdateValueClassType recovers homogeneous Class()/method-return
// value type from da.update({"k": A()}) / da.update(k=A(), j=A()) /
// da.update(k=ba.get()) / da.update({"k": ba.get()}). Positional list/set of
// Class() is Counter/set path (not mapping values) — fail closed here.
// Mixed / non-Class / splat fail closed.
func pythonDictUpdateValueClassType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument":
			// k=A() / k=ba.get()
			val := ingest.ChildByField(ch, "value")
			if val == nil {
				return ""
			}
			et := pythonObjectLeafType(val, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		case "dictionary":
			// {"k": A()} / {"k": ba.get()}
			et := pythonHomogeneousObjectDictLiteralValue(ch, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				// Class()-only fallback when object maps unavailable.
				et = pythonDictLiteralHomogeneousValueClass(ch, content)
			}
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		case "list", "tuple":
			// [("k", A())] / (("k", ba.get()),) — homogeneous pair values.
			et := pythonDictUpdatePairsValueClass(ch, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		default:
			// positional set/other — not mapping-value update.
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// pythonDictUpdatePairsValueClass recovers T from [("k", A()), ("j", A())] /
// (("k", ba.get()),) / [["k", A()]] when every element is a 2-slot list/tuple
// whose second slot peels to the same Class leaf (ctor or method-return).
// Empty / mixed / non-pair fail closed.
func pythonDictUpdatePairsValueClass(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "list", "tuple":
		// ok
	default:
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		case "list", "tuple":
			et := pythonPairSecondObjectLeaf(ch, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		default:
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// pythonPairSecondObjectLeaf recovers T from (k, A()) / [k, ba.get()] when the
// second positional element peels to a Class leaf. Other shapes / length fail closed.
func pythonPairSecondObjectLeaf(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "list", "tuple":
		// ok
	default:
		return ""
	}
	var elems []*grammar.Node
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		default:
			elems = append(elems, ch)
		}
	}
	if len(elems) != 2 {
		return ""
	}
	return pythonObjectLeafType(elems[1], content, funcReturns, typeOf, fieldOf)
}

// pythonPairSecondClassCtor recovers T from (k, A()) / [k, A()] when the second
// positional element is Class(). Other shapes / length fail closed.
func pythonPairSecondClassCtor(n *grammar.Node, content []byte) string {
	return pythonPairSecondObjectLeaf(n, content, nil, nil, nil)
}

// pythonDictLiteralHomogeneousValueClass recovers T from {"k": A(), "j": A()}
// when every value is Class() of the same leaf. Empty / mixed / non-pair fail closed.
func pythonDictLiteralHomogeneousValueClass(n *grammar.Node, content []byte) string {
	if n == nil || n.Type() != "dictionary" {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "{", "}", ",", "comment":
			continue
		case "pair":
			val := ingest.ChildByField(ch, "value")
			if val == nil {
				return ""
			}
			et := pythonClassCtorName(val, content)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		default:
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// pythonDictSubscriptAssignValueType recovers (local, T) from da["k"] = A() /
// da[k] = A() / xs[0] = A() when the value is Class() and the key is not a slice.
// Also peels method-return RHS: da["k"] = ba.get() / xs[0] = ba.get().
// Foreign values bind too for shadowing under dual-class same-leaf.
func pythonDictSubscriptAssignValueType(left, right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) (local, classType string) {
	if left == nil || left.Type() != "subscript" || right == nil {
		return "", ""
	}
	for i := uint32(0); i < left.ChildCount(); i++ {
		if left.Child(i).Type() == "slice" {
			return "", ""
		}
	}
	val := ingest.ChildByField(left, "value")
	if val == nil || val.Type() != "identifier" {
		return "", ""
	}
	// Class() / typed local / zero-arg method return (ba.get()).
	et := pythonObjectLeafType(right, content, funcReturns, typeOf, fieldOf)
	if et == "" {
		return "", ""
	}
	return ingest.NodeText(val, content), et
}

// pythonSetdefaultDefaultClassType recovers T from d.setdefault(k, A()) /
// d.setdefault(k, ba.get()) when the 2nd positional arg peels to a Class leaf
// (ctor or zero-arg method return). Used when the dict local is untyped so
// elemOf cannot peel the return. Single-arg setdefault fails closed here.
func pythonSetdefaultDefaultClassType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 2 {
		return ""
	}
	return pythonObjectLeafType(args[1], content, funcReturns, typeOf, fieldOf)
}

// pythonNamedtupleCtorInstanceFields recovers field→Class leaves from one
// namedtuple constructor call: Box(A(), B()) / Box(a=A(), b=B()) when field
// names are known from namedtuple(...). Used to bind instance-level fieldOf
// (ba.a / ba.#0) under dual-class same-field names — type-level fieldIndex is
// last-writer-wins across ba=Box(A()); bb=Box(B()) and would under-rename A.
// Unknown field order (no fieldNames) accepts kwargs only. Non-Class / splat fail closed (nil).
func pythonNamedtupleCtorInstanceFields(call *grammar.Node, content []byte, fieldNames []string) map[string]string {
	if call == nil || call.Type() != "call" {
		return nil
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return nil
	}
	out := map[string]string{}
	// Keyword: Box(a=A(), b=B()).
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch.Type() != "keyword_argument" {
			continue
		}
		nameN := ingest.ChildByField(ch, "name")
		valN := ingest.ChildByField(ch, "value")
		if nameN == nil || nameN.Type() != "identifier" {
			return nil
		}
		et := pythonExprClassType(valN, content)
		if et == "" {
			return nil
		}
		out[ingest.NodeText(nameN, content)] = et
	}
	// Positional: Box(A(), B()) — needs ordered field names.
	if len(fieldNames) > 0 {
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok {
			return nil
		}
		for i, arg := range args {
			if i >= len(fieldNames) {
				break
			}
			et := pythonExprClassType(arg, content)
			if et == "" {
				// Non-Class positional — fail closed for whole call (mixed leaves).
				return nil
			}
			out[fieldNames[i]] = et
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pythonFutureSetResultType recovers T from fa.set_result(T()) / fa.set_result(T())
// when the first positional arg is a Class() call (or bare Class identifier is
// not accepted — result value is the instance). Enables futureOf binding for
// fa.result().run() under foreign same-leaf methods.
func pythonFutureSetResultType(call *grammar.Node, content []byte) (futLocal, classType string) {
	if call == nil || call.Type() != "call" {
		return "", ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return "", ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "set_result" {
		return "", ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil || obj.Type() != "identifier" {
		return "", ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 1 {
		return "", ""
	}
	// set_result(A()) — Class() instance. Bare identifiers fail closed (may be a
	// pre-built local; assignment path already binds those via typeOf).
	if et := pythonExprClassType(args[0], content); et != "" {
		return ingest.NodeText(obj, content), et
	}
	return "", ""
}

// pythonSubmitCallableResultType recovers T from executor.submit(lambda: T())
// when the first positional arg is a zero-arg expression-bodied lambda whose body
// is a Class() call. Other submit forms (fn, *args) / blocks fail closed.
func pythonSubmitCallableResultType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "submit" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 1 || args[0].Type() != "lambda" {
		return ""
	}
	// Zero-arg lambda only (Callable with no parameters).
	if params := ingest.ChildByField(args[0], "parameters"); params != nil {
		for i := uint32(0); i < params.ChildCount(); i++ {
			ch := params.Child(i)
			switch ch.Type() {
			case ",", "(", ")", "comment":
				continue
			default:
				// Any real parameter fails closed.
				return ""
			}
		}
	}
	body := ingest.ChildByField(args[0], "body")
	if body == nil {
		return ""
	}
	// Expression-bodied: lambda: A()  (body is the expression, not a block).
	if body.Type() == "call" {
		return pythonExprClassType(body, content)
	}
	return ""
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
// `a = min(asdict(pair).values(), key=...)` / `a = max(astuple(pair))` when all
// declaration-order field types agree (homogeneous values; mixed fail closed —
// same leaf as for x in asdict(...).values() / for x in astuple(...)),
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
// `k, a = next(d.items())` / `k, a = next(iter(d.items()))` (dict value on 2nd slot),
// `k, x = next(asdict(pair).items())` / `next(iter(asdict(pair).items()))` when
// all declaration-order field types agree (homogeneous values; mixed fail closed),
// `a = d.popitem()[1]` / `a = (d.popitem())[1]` (pair value slot; [0]/other fail closed),
// `it1, it2 = tee(items)` / `it1, it2 = itertools.tee(items[, n])` —
// each target is an iterator of items elements (elemOf; not an element itself),
// `xs = items.copy()` / `xs = items or []` (elemOf preserved for later index/for),
// `a = it.__next__()` when `it = iter(items)` (or other known iterable) has element type,
// `a = items.__getitem__(i)` / `a = d.__getitem__(k)` (same element/value as items[i]),
// `a = getitem(items, i)` / `a = operator.getitem(items, i)` (same leaf as items[i]),
// as-bindings (`case A() as a`, `with A() as a`, `except A as e`),
// match sequence captures from a known collection subject
// (`match items: case [a]:` / `case [a, *rest]:` with `items: list[A]` —
// fixed slots are elements; *rest is a sequence of the same element type),
// match mapping value captures from a known dict subject
// (`match d: case {"k": a}:` / `case {"k": a as x}:` with `d: dict[K, A]` —
// value slots are the dict value leaf; **rest fails closed),
// match TypedDict/record key captures from a known object subject
// (`match box: case {"a": xa}:` / `case {"a": xa as x}:` with `box: Box`
// and annotated field a: A — key-specific fieldOf leaf; non-string keys
// and **rest fail closed),
// match class-pattern keyword and positional captures
// (`match box: case Box(a=xa, b=xb):` / `case Box(xa, xb):` / `case Box(a=xa as x):`
// — field types of Box via fieldIndex; positionals use declaration order),
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
// `for item in d.items(): item[1]` / `for item in asdict(...).items(): item[1]`
// (pairSlots; key untyped — same leaf as p = next(...items()); p[1]),
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
// constructors (`Box = namedtuple("Box", ["a","b"]); box = Box(A(), B())`),
// including positional index `box[0].run()` / `xa = box[0]` (fieldOf["box.#0"]),
// `xa = box["a"]` / `box["a"].run()` / `xa = box.get("a")` for TypedDict-style
// string keys of the same annotated fields (fieldOf),
// `new = replace(box)` / `dataclasses.replace(box)` / walrus — same object type
// as box (fieldOf for `new.a.run()`), plus direct chains `replace(box).a.run()`
// and `xa = replace(box).a` (field leaf of first positional arg),
// `d = asdict(box)` / `dataclasses.asdict(box)` / walrus — field keys of the
// first positional arg (fieldOf for `d["a"].run()` / `d.get("a").run()` under
// foreign same-leaf methods; asdict yields a dict of field values),
// plus direct chains `asdict(box)["a"].run()` / `asdict(box).get("a").run()` /
// `xa = asdict(box)["a"]` / `xa = asdict(box).get("a")`,
// `d = vars(box)` / walrus — same field-key binding (vars yields obj.__dict__),
// plus direct `vars(box)["a"].run()` / `vars(box).get("a").run()` /
// `xa = vars(box)["a"]`,
// `d = box.__dict__` / walrus — same field-key binding (instance attribute dict),
// plus direct `box.__dict__["a"].run()` / `box.__dict__.get("a").run()` /
// `xa = box.__dict__["a"]`,
// `t = astuple(box)` / `dataclasses.astuple(box)` / `t = astuple(replace(box))` /
// walrus — ordered index slots of the first positional arg (fieldOf["t.#0"] for
// `t[0].run()`; astuple yields a tuple of field values in declaration order, not
// named keys; replace peels to the same dataclass),
// plus direct chains `astuple(box)[0].run()` / `astuple(replace(box))[0].run()` /
// `list(astuple(box))[0].run()` / `tuple(astuple(box))[0].run()` /
// `list(asdict(box).values())[0].run()` / `list(d.values())[0].run()` after
// `d = asdict(box)` / vars / `__dict__` (dict preserves declaration order),
// `for x in asdict(box).values()` / `for x in d.values()` after d = asdict(box) /
// vars / `__dict__` / list(...values()) — only when all declaration-order field
// types agree (homogeneous values; mixed fields fail closed — not a shared elemOf),
// `for x in astuple(box)` / `for x in list(astuple(box))` / `t = astuple(box); for x in t`
// — same homogeneous-values gate (mixed fields fail closed),
// `for k, x in asdict(box).items()` / `for k, x in d.items()` after d = asdict(box) /
// vars / `__dict__` — same homogeneous-values gate (value slot only; key untyped),
// `for item in asdict(box).items(): item[1]` / `for item in d.items(): item[1]` —
// pairSlots (key untyped; same leaf as p = next(...items()); p[1]),
// `xa, xb = asdict(box).values()` / `list(asdict(box).values())` / `d.values()` after
// d = asdict(box) / vars / `__dict__` — declaration-order field types (dict preserves
// order; same leaf as `xa, xb = astuple(box)` / `list(asdict(box).values())[i]`),
// `xa = astuple(box)[0]` (synthetic `@astuple.box.#i` slots — not `box.#i`, so
// bare `box[0]` stays unbound for non-indexable dataclasses),
// plus assignment `xs = list(astuple(box)); xs[0]` (index slots on xs),
// plus unpack `xa, xb = astuple(box)` / `xa, xb = list(astuple(box))` /
// `xa, *rest = astuple(box)` (per-slot field types; *rest of mixed tuple fails closed),
// `sorted/min/max/groupby(items, key=lambda x: x.m())` / `items.sort(key=lambda x: x.m())` /
// `nlargest/nsmallest(n, items, key=lambda x: x.m())` /
// `nlargest/nsmallest(n, items, lambda x: x.m())` (positional key) / `heapq.nlargest(...)` /
// `merge(xs, ys, key=lambda x: x.m())` / `heapq.merge(..., key=lambda x: x.m())` /
// `bisect_left/bisect_right/bisect/insort_*(a, x, key=lambda e: e.m())` /
// `bisect.bisect_left(...)` — untyped unary key=lambda from list element type /
// `sorted/min/max(..., key=cmp_to_key(lambda a, b: a.m()-b.m()))` /
// `functools.cmp_to_key(...)` — untyped bi-lambda params from element type /
// `map/filter/takewhile/dropwhile/filterfalse(lambda x: x.m(), items)` /
// `itertools.takewhile/groupby/...` — untyped unary lambda params from the iterable
// element type (under foreign same-leaf).
// `reduce/functools.reduce(lambda a, b: a.m(), items)` /
// `accumulate/itertools.accumulate(items, lambda a, b: a.m())` — untyped bi-lambda
// params from the iterable element type (same-leaf fold; under foreign same-leaf).
// fieldOf maps "local.field" → field type leaf for class field access.
// elemOf maps collection locals → element type leaf (list[A] / deque[A] /
// dict value / Queue[A] → "A") for direct access chains under foreign same-leaf.
// typeOf maps object locals → type leaf (item: A / x = A() → "A"; foreign too)
// for direct identity wrappers under foreign same-leaf (copy.copy(item).run()).
// factoryOf maps partial factory locals → class leaf (pa = partial(A) → "A") so
// pa().run() peels under foreign same-leaf (local itself is not an A instance).
// futureOf maps Future locals → result class leaf (fa.set_result(A()) → "A") so
// fa.result().run() peels under foreign same-leaf.

// pythonSameFileFuncReturnTypes maps same-file function names to concrete return
// type leaves from annotations: def make_a() -> A / @lru_cache def make_a() -> A.
// When no return annotation is present, recovers T from body-only `return T()`
// when every return in the function body is a call of the same bare identifier
// (def make_a(): return A()) or `return x` after a local `x = T()` assignment
// (def make_a(): a = A(); return a). Decorated definitions (lru_cache /
// functools.lru_cache / cache / etc.) peel through to the nested
// function_definition. Nested functions inside bodies are included (same-file
// name → last wins). Missing / mixed / non-simple returns fail closed.
//
// Also records:
//   - make_a = lambda: A() — zero-arg expression-bodied lambda factory locals
//   - A.make / A.create — @staticmethod / @classmethod factories that return
//     A() or cls() (keys are "Class.method" for A.make().run() peels)
func pythonSameFileFuncReturnTypes(root *grammar.Node, content []byte) map[string]string {
	out := map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		switch n.Type() {
		case "function_definition", "async_function_definition":
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.Type() == "identifier" {
				name := ingest.NodeText(nameN, content)
				if name != "" {
					retN := ingest.ChildByField(n, "return_type")
					if retN != nil {
						if tn := pythonTypeName(retN, content); tn != "" {
							out[name] = tn
						}
					} else if tn := pythonFuncBodyReturnCtor(n, content); tn != "" {
						// Body-only factory: def make_a(): return A()
						out[name] = tn
					}
				}
			}
		case "assignment":
			// make_a = lambda: A() / make_b = lambda: B() — factory local.
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left != nil && left.Type() == "identifier" && right != nil && right.Type() == "lambda" {
				lname := ingest.NodeText(left, content)
				if lname != "" {
					if tn := pythonLambdaFactoryReturnCtor(right, content); tn != "" {
						out[lname] = tn
					}
				}
			}
		case "class_definition":
			// @staticmethod / @classmethod factories on the class:
			// A.make().run() after @staticmethod def make(): return A()
			// A.create().run() after @classmethod def create(cls): return cls()
			pythonHarvestClassFactoryReturns(n, content, out)
			// Instance methods with annotated returns: BoxA.get → A.
			pythonHarvestClassInstanceMethodReturns(n, content, out)
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	return out
}

// pythonLambdaFactoryReturnCtor recovers T from a zero-arg expression-bodied
// lambda whose body is a Class() call: lambda: A(). Other shapes fail closed.
func pythonLambdaFactoryReturnCtor(lam *grammar.Node, content []byte) string {
	if lam == nil || lam.Type() != "lambda" {
		return ""
	}
	// Zero-arg only (same gate as submit(lambda: T())).
	if params := ingest.ChildByField(lam, "parameters"); params != nil {
		for i := uint32(0); i < params.ChildCount(); i++ {
			ch := params.Child(i)
			switch ch.Type() {
			case ",", "(", ")", "comment":
				continue
			default:
				return ""
			}
		}
	}
	body := ingest.ChildByField(lam, "body")
	if body == nil || body.Type() != "call" {
		return ""
	}
	return pythonExprClassType(body, content)
}

// pythonHarvestClassInstanceMethodReturns records Class.method → T for
// undecorated instance methods with a concrete return annotation:
//
//	class BoxA:
//	    def get(self) -> A: ...
//	    def self(self) -> BoxA: ...
//
// Keys use "Class.method" so ba.get().run() / ba.self().item.run() peel under
// foreign same-leaf. Decorated methods (@property / @staticmethod / …) are
// skipped (properties already go through fieldIndex; factories above).
func pythonHarvestClassInstanceMethodReturns(classDef *grammar.Node, content []byte, out map[string]string) {
	if classDef == nil || classDef.Type() != "class_definition" || out == nil {
		return
	}
	nameN := ingest.ChildByField(classDef, "name")
	if nameN == nil || nameN.Type() != "identifier" {
		return
	}
	className := ingest.NodeText(nameN, content)
	if className == "" {
		return
	}
	body := ingest.ChildByField(classDef, "body")
	if body == nil {
		return
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		ch := body.Child(i)
		if ch == nil {
			continue
		}
		// Only bare function_definition — skip decorated_definition (@property etc.).
		if ch.Type() != "function_definition" && ch.Type() != "async_function_definition" {
			continue
		}
		mNameN := ingest.ChildByField(ch, "name")
		retN := ingest.ChildByField(ch, "return_type")
		if mNameN == nil || mNameN.Type() != "identifier" || retN == nil {
			continue
		}
		method := ingest.NodeText(mNameN, content)
		if method == "" || method == "_" {
			continue
		}
		// Skip dunder / private noise except intentional product names.
		if strings.HasPrefix(method, "__") && strings.HasSuffix(method, "__") {
			continue
		}
		tn := pythonTypeName(retN, content)
		if tn == "" {
			continue
		}
		out[className+"."+method] = tn
	}
}

// pythonHarvestClassFactoryReturns records Class.method → Class for
// @staticmethod / @classmethod methods whose body always returns Class() or
// (classmethod only) cls(). Keys use "Class.method" so A.make().run() peels
// under foreign same-leaf methods without colliding with bare make().
func pythonHarvestClassFactoryReturns(classDef *grammar.Node, content []byte, out map[string]string) {
	if classDef == nil || classDef.Type() != "class_definition" || out == nil {
		return
	}
	nameN := ingest.ChildByField(classDef, "name")
	if nameN == nil || nameN.Type() != "identifier" {
		return
	}
	className := ingest.NodeText(nameN, content)
	if className == "" {
		return
	}
	body := ingest.ChildByField(classDef, "body")
	if body == nil {
		return
	}
	for i := uint32(0); i < body.ChildCount(); i++ {
		ch := body.Child(i)
		if ch == nil || ch.Type() != "decorated_definition" {
			continue
		}
		kind := pythonFactoryDecoratorKind(ch, content)
		if kind == "" {
			continue
		}
		var fn *grammar.Node
		for j := uint32(0); j < ch.ChildCount(); j++ {
			c := ch.Child(j)
			if c != nil && (c.Type() == "function_definition" || c.Type() == "async_function_definition") {
				fn = c
				break
			}
		}
		if fn == nil {
			continue
		}
		mNameN := ingest.ChildByField(fn, "name")
		if mNameN == nil || mNameN.Type() != "identifier" {
			continue
		}
		method := ingest.NodeText(mNameN, content)
		if method == "" {
			continue
		}
		ret := ""
		if tn := pythonFuncBodyReturnCtor(fn, content); tn == className {
			// @staticmethod def make(): return A() / @classmethod … return A()
			ret = className
		} else if kind == "classmethod" && pythonFuncBodyReturnsCls(fn, content) {
			// @classmethod def create(cls): return cls()
			ret = className
		}
		if ret != "" {
			out[className+"."+method] = ret
		}
	}
}

// pythonFactoryDecoratorKind returns "staticmethod" / "classmethod" when the
// decorated_definition has that bare decorator (or fails closed on others).
func pythonFactoryDecoratorKind(decorated *grammar.Node, content []byte) string {
	if decorated == nil || decorated.Type() != "decorated_definition" {
		return ""
	}
	kind := ""
	for i := uint32(0); i < decorated.ChildCount(); i++ {
		ch := decorated.Child(i)
		if ch == nil || ch.Type() != "decorator" {
			continue
		}
		// @staticmethod / @classmethod — decorator child is bare identifier.
		var id *grammar.Node
		for j := uint32(0); j < ch.ChildCount(); j++ {
			c := ch.Child(j)
			if c != nil && c.Type() == "identifier" {
				id = c
				break
			}
		}
		if id == nil {
			continue
		}
		switch ingest.NodeText(id, content) {
		case "staticmethod", "classmethod":
			// Last matching decorator wins (unusual stacks fail toward last).
			kind = ingest.NodeText(id, content)
		}
	}
	return kind
}

// pythonFuncBodyReturnsCls reports whether every return in fn's body is
// `return cls(...)` (classmethod factory). Nested scopes are skipped.
func pythonFuncBodyReturnsCls(fn *grammar.Node, content []byte) bool {
	if fn == nil {
		return false
	}
	body := ingest.ChildByField(fn, "body")
	if body == nil {
		return false
	}
	saw := false
	ok := true
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() || !ok {
			return
		}
		switch n.Type() {
		case "function_definition", "async_function_definition", "class_definition", "lambda":
			return
		case "return_statement":
			var expr *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == nil || ch.Type() == "return" {
					continue
				}
				expr = ch
				break
			}
			if expr == nil || expr.Type() != "call" {
				ok = false
				return
			}
			f := ingest.ChildByField(expr, "function")
			if f == nil || f.Type() != "identifier" || ingest.NodeText(f, content) != "cls" {
				ok = false
				return
			}
			saw = true
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(body)
	return saw && ok
}

// pythonCallFuncReturnType recovers the same-file factory/method return class
// leaf of a call: make_a() (bare), A.make() / A.create() ("Class.method" keys),
// or ba.get() when typeOf[ba]=BoxA and funcReturns["BoxA.get"]=A (instance
// method with annotated return under foreign same-leaf).
// typeOf / fieldOf may be nil (static/class factory paths only).
func pythonCallFuncReturnType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" || funcReturns == nil {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		return funcReturns[ingest.NodeText(fn, content)]
	case "attribute":
		obj := ingest.ChildByField(fn, "object")
		attr := ingest.ChildByField(fn, "attribute")
		if obj == nil || attr == nil || attr.Type() != "identifier" {
			return ""
		}
		mname := ingest.NodeText(attr, content)
		if mname == "" {
			return ""
		}
		// Peel (expr) / (name := expr) receivers so (ba.self()).get() /
		// (xa := ba.self()).get() peels under foreign same-leaf.
		for obj != nil && !obj.IsNull() {
			switch obj.Type() {
			case "parenthesized_expression":
				obj = pythonParenInner(obj)
			case "named_expression":
				obj = ingest.ChildByField(obj, "value")
			default:
				goto objPeeled
			}
		}
	objPeeled:
		if obj == nil || obj.IsNull() {
			return ""
		}
		// A.make() — class-qualified factory / static method.
		if obj.Type() == "identifier" {
			oname := ingest.NodeText(obj, content)
			if rt := funcReturns[oname+"."+mname]; rt != "" {
				return rt
			}
			// ba.get() — instance method on typed local (typeOf[ba]=BoxA).
			if typeOf != nil {
				if tn := typeOf[oname]; tn != "" {
					if rt := funcReturns[tn+"."+mname]; rt != "" {
						return rt
					}
				}
			}
		}
		// ba.self().get() — peel nested call receiver type then method return.
		if obj.Type() == "call" {
			if rt := pythonCallFuncReturnType(obj, content, funcReturns, typeOf, fieldOf); rt != "" {
				if t := funcReturns[rt+"."+mname]; t != "" {
					return t
				}
			}
		}
		// oa.h.get() — field access peel then instance method return (dual-class).
		// oa typed OuterA with field h: HolderA; funcReturns["HolderA.get"]=A.
		// Assigned ha = oa.h; ha.get() already peels via typeOf.
		if obj.Type() == "attribute" {
			if ft := pythonFieldAccessType(obj, content, fieldOf, funcReturns, typeOf); ft != "" {
				if t := funcReturns[ft+"."+mname]; t != "" {
					return t
				}
			}
		}
		// (ba if c else ba).get() — both arms agree on object type T then T.m.
		if obj.Type() == "conditional_expression" {
			body, alt := pythonConditionalArms(obj)
			t1 := pythonObjectExprType(body, content, typeOf)
			t2 := pythonObjectExprType(alt, content, typeOf)
			if t1 != "" && t1 == t2 {
				if t := funcReturns[t1+"."+mname]; t != "" {
					return t
				}
			}
		}
	}
	return ""
}

// pythonFuncBodyReturnCtor recovers T when every return in fn's body is
// `return T(...)` (call of a bare identifier) or `return x` after a local
// assignment `x = T()` / `x = T(...)` of that same bare identifier in the
// function body (def make_a(): a = A(); return a). Nested function/class/lambda
// bodies are skipped. Zero, mixed, or non-ctor returns fail closed ("").
func pythonFuncBodyReturnCtor(fn *grammar.Node, content []byte) string {
	if fn == nil {
		return ""
	}
	body := ingest.ChildByField(fn, "body")
	if body == nil {
		return ""
	}
	// Local name → ctor type from simple assignments x = T() in this body.
	localCtor := map[string]string{}
	const fail = "-"
	found := ""
	saw := false
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() || found == fail {
			return
		}
		switch n.Type() {
		case "function_definition", "async_function_definition", "class_definition", "lambda":
			// Nested scopes: do not harvest their returns for the outer factory.
			return
		case "assignment":
			// x = T() / x = T(...) — track local → ctor for return x peels.
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left != nil && left.Type() == "identifier" && right != nil && right.Type() == "call" {
				if f := ingest.ChildByField(right, "function"); f != nil && f.Type() == "identifier" {
					name := ingest.NodeText(left, content)
					ctor := ingest.NodeText(f, content)
					if name != "" && ctor != "" {
						localCtor[name] = ctor
					}
				}
			}
		case "return_statement":
			var expr *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == nil || ch.Type() == "return" {
					continue
				}
				expr = ch
				break
			}
			t := ""
			if expr != nil && expr.Type() == "call" {
				if f := ingest.ChildByField(expr, "function"); f != nil && f.Type() == "identifier" {
					t = ingest.NodeText(f, content)
				}
			} else if expr != nil && expr.Type() == "identifier" {
				// return a after a = A()
				t = localCtor[ingest.NodeText(expr, content)]
			}
			if t == "" {
				found = fail
				return
			}
			if !saw {
				found = t
				saw = true
			} else if found != t {
				found = fail
			}
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(body)
	if !saw || found == fail {
		return ""
	}
	return found
}

// pythonSameFileGeneratorYields maps same-file bare generator function names
// to the concrete yield type leaf:
//
//	def gen_a():
//	    yield A()
//
//	async def agen_a():
//	    a = A()
//	    yield a
//
//	def gen_mr(ba: BoxA):
//	    yield ba.get()
//
// Enables next(gen_a()).run() / for a in gen_a(): a.run() under foreign
// same-leaf methods. @contextmanager / @asynccontextmanager factories are
// skipped (with-as uses pythonSameFileContextManagerYields). Mixed/non-leaf
// yields and yield-from fail closed (pythonFuncBodyYieldCtor).
// funcReturns peels method-return yields under foreign same-leaf.
func pythonSameFileGeneratorYields(root *grammar.Node, content []byte, funcReturns map[string]string) map[string]string {
	out := map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		// Skip CM-decorated defs: with-as peels separately; iterating a
		// contextmanager factory is not the generator-yield product case.
		if n.Type() == "decorated_definition" && pythonIsContextManagerDecorated(n, content) {
			return
		}
		if n.Type() == "function_definition" || n.Type() == "async_function_definition" {
			nameN := ingest.ChildByField(n, "name")
			if nameN != nil && nameN.Type() == "identifier" {
				name := ingest.NodeText(nameN, content)
				if name != "" {
					if tn := pythonFuncBodyYieldCtor(n, content, funcReturns); tn != "" {
						out[name] = tn
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

// pythonSameFileContextManagerYields maps same-file @contextmanager /
// @asynccontextmanager factory names to the concrete yield type leaf:
//
//	@contextmanager
//	def make_a():
//	    yield A()
//
//	@contextlib.contextmanager
//	def make_a2():
//	    a = A()
//	    yield a
//
// Enables `with make_a() as a: a.run()` under foreign same-leaf methods.
// Mixed/non-ctor yields and non-CM decorators fail closed.
func pythonSameFileContextManagerYields(root *grammar.Node, content []byte) map[string]string {
	out := map[string]string{}
	if root == nil {
		return out
	}
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() {
			return
		}
		if n.Type() == "decorated_definition" {
			if pythonIsContextManagerDecorated(n, content) {
				var fn *grammar.Node
				for i := uint32(0); i < n.ChildCount(); i++ {
					ch := n.Child(i)
					if ch != nil && (ch.Type() == "function_definition" || ch.Type() == "async_function_definition") {
						fn = ch
						break
					}
				}
				if fn != nil {
					nameN := ingest.ChildByField(fn, "name")
					if nameN != nil && nameN.Type() == "identifier" {
						name := ingest.NodeText(nameN, content)
						if name != "" {
							// CM factories: Class()-only yield peel (funcReturns nil).
							if tn := pythonFuncBodyYieldCtor(fn, content, nil); tn != "" {
								out[name] = tn
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
	return out
}

// pythonIsContextManagerDecorated reports whether decorated has a
// contextmanager / asynccontextmanager decorator (bare or contextlib.*).
func pythonIsContextManagerDecorated(decorated *grammar.Node, content []byte) bool {
	if decorated == nil || decorated.Type() != "decorated_definition" {
		return false
	}
	for i := uint32(0); i < decorated.ChildCount(); i++ {
		ch := decorated.Child(i)
		if ch == nil || ch.Type() != "decorator" {
			continue
		}
		// @contextmanager / @asynccontextmanager / @contextlib.contextmanager
		for j := uint32(0); j < ch.ChildCount(); j++ {
			c := ch.Child(j)
			if c == nil {
				continue
			}
			switch c.Type() {
			case "identifier":
				switch ingest.NodeText(c, content) {
				case "contextmanager", "asynccontextmanager":
					return true
				}
			case "attribute":
				// contextlib.contextmanager / contextlib.asynccontextmanager
				attr := ingest.ChildByField(c, "attribute")
				if attr != nil && attr.Type() == "identifier" {
					switch ingest.NodeText(attr, content) {
					case "contextmanager", "asynccontextmanager":
						return true
					}
				}
			}
		}
	}
	return false
}

// pythonFuncBodyYieldCtor recovers T when every yield in fn's body is
// `yield T(...)` / `yield ba.get()` (typed param or local) / `yield x` after
// a local `x = T()` or `x = ba.get()` assignment. Nested scopes and `yield from`
// fail closed. Zero/mixed/non-leaf yields fail closed.
// funcReturns peels method-return yields under foreign same-leaf (may be nil
// for Class()-only paths such as contextmanager factories).
func pythonFuncBodyYieldCtor(fn *grammar.Node, content []byte, funcReturns map[string]string) string {
	if fn == nil {
		return ""
	}
	body := ingest.ChildByField(fn, "body")
	if body == nil {
		return ""
	}
	// Typed locals inside the generator: params (ba: BoxA) + assignments.
	localCtor := map[string]string{}
	// Harvest annotated parameters so yield ba.get() peels under foreign same-leaf.
	if params := ingest.ChildByField(fn, "parameters"); params != nil {
		for i := uint32(0); i < params.ChildCount(); i++ {
			ch := params.Child(i)
			if ch == nil {
				continue
			}
			// typed_parameter / typed_default_parameter: ba: BoxA / ba: "BoxA" = ...
			if ch.Type() != "typed_parameter" && ch.Type() != "typed_default_parameter" {
				continue
			}
			nameN := ingest.ChildByField(ch, "name")
			typeN := ingest.ChildByField(ch, "type")
			if nameN == nil {
				nameN = ingest.ChildByType(ch, "identifier")
			}
			if nameN != nil && typeN != nil {
				if tn := pythonTypeName(typeN, content); tn != "" {
					if name := ingest.NodeText(nameN, content); name != "" {
						localCtor[name] = tn
					}
				}
			}
		}
	}
	const fail = "-"
	found := ""
	saw := false
	var walk func(n *grammar.Node)
	walk = func(n *grammar.Node) {
		if n == nil || n.IsNull() || found == fail {
			return
		}
		switch n.Type() {
		case "function_definition", "async_function_definition", "class_definition", "lambda":
			return
		case "assignment":
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left != nil && left.Type() == "identifier" && right != nil {
				name := ingest.NodeText(left, content)
				if name == "" {
					break
				}
				// x = T() / x = ba.get() / x = BoxA().get() (when peels).
				if t := pythonObjectLeafType(right, content, funcReturns, localCtor, nil); t != "" {
					localCtor[name] = t
				} else if right.Type() == "call" {
					if f := ingest.ChildByField(right, "function"); f != nil && f.Type() == "identifier" {
						ctor := ingest.NodeText(f, content)
						if ctor != "" {
							localCtor[name] = ctor
						}
					}
				}
			}
		case "yield":
			// yield T() / yield ba.get() / yield x — reject yield from.
			var expr *grammar.Node
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch == nil {
					continue
				}
				switch ch.Type() {
				case "yield":
					continue
				case "from":
					found = fail
					return
				default:
					if expr == nil {
						expr = ch
					}
				}
			}
			t := ""
			if expr != nil {
				// Class() / method-return / typed local leaf.
				t = pythonObjectLeafType(expr, content, funcReturns, localCtor, nil)
				if t == "" && expr.Type() == "call" {
					if f := ingest.ChildByField(expr, "function"); f != nil && f.Type() == "identifier" {
						t = ingest.NodeText(f, content)
					}
				} else if t == "" && expr.Type() == "identifier" {
					t = localCtor[ingest.NodeText(expr, content)]
				}
			}
			if t == "" {
				found = fail
				return
			}
			if !saw {
				found = t
				saw = true
			} else if found != t {
				found = fail
			}
			return
		}
		for i := uint32(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(body)
	if !saw || found == fail {
		return ""
	}
	return found
}

// pythonAsPatternCMYieldBinding recovers (alias, yieldType) from
// `with make_a() as a` when make_a is a same-file contextmanager factory
// recorded in cmYieldOf. Other shapes fail closed.
func pythonAsPatternCMYieldBinding(n *grammar.Node, content []byte, cmYieldOf map[string]string) (name, typ string) {
	if n == nil || n.Type() != "as_pattern" || cmYieldOf == nil {
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
			if id := ingest.ChildByType(ch, "identifier"); id != nil {
				alias = id
			}
		case "identifier":
			alias = ch
		}
	}
	if left == nil || alias == nil || left.Type() != "call" {
		return "", ""
	}
	fn := ingest.ChildByField(left, "function")
	if fn == nil || fn.Type() != "identifier" {
		return "", ""
	}
	fname := ingest.NodeText(fn, content)
	if fname == "" {
		return "", ""
	}
	tn := cmYieldOf[fname]
	if tn == "" {
		return "", ""
	}
	return ingest.NodeText(alias, content), tn
}

func pythonTypedLocals(root *grammar.Node, content []byte, ourReceivers map[string]bool) (map[string]bool, map[string]string, map[string]string, map[string]string, map[string][]string, map[string]string, map[string]string, map[string]string, map[string]string) {
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
	// Returned so direct method chains p[1].run() resolve under foreign same-leaf.
	pairSlots := map[string][]string{}
	// Assigned pair-iterators → per-slot types of each yielded tuple
	// (pairs = zip/enumerate/product/...; combos = combinations/batched(xs, n);
	// for a, b in pairs / for pair in pairs).
	pairIterSlots := map[string][]string{}
	// Class field access: "box.a" → "A" when box is typed as a class with field a: A.
	fieldOf := map[string]string{}
	// partial factory locals → class leaf (pa = partial(A) / functools.partial(A)).
	factoryOf := map[string]string{}
	// Future locals → result class leaf (fa.set_result(A()) / set_result(B())).
	futureOf := map[string]string{}
	// Stored operator getters: ga = attrgetter("a") / gi = itemgetter(0) →
	// "attrgetter:a" / "itemgetter:#" so ga(box).run() / gi(items).run() peel.
	getterOf := map[string]string{}
	funcReturns := map[string]string{}
	// @contextmanager factories → yield type leaf (with make_a() as a → a is A).
	cmYieldOf := map[string]string{}
	if root == nil || len(ourReceivers) == 0 {
		return out, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns
	}
	funcReturns = pythonSameFileFuncReturnTypes(root, content)
	cmYieldOf = pythonSameFileContextManagerYields(root, content)
	// Same-file bare generators (yield A() / yield ba.get()) → elemOf["@yield.name"]
	// so next(gen_a()) / for a in gen_a() / g = gen_a(); next(g) peel under
	// foreign same-leaf methods. @contextmanager factories stay out (with-as).
	// funcReturns peels method-return yields under foreign same-leaf.
	for name, t := range pythonSameFileGeneratorYields(root, content, funcReturns) {
		if name != "" && t != "" {
			elemOf["@yield."+name] = t
		}
	}
	fieldIndex := pythonClassFieldIndex(root, content)
	// Declaration order for positional match class patterns (case Box(xa, xb)).
	fieldOrder := pythonClassFieldOrder(root, content)
	// namedtuple factory fields have no annotations — recover from same-file ctors.
	// fieldNames keeps factory order so box[i] can resolve via fieldOf["box.#i"].
	fieldNames := pythonNamedtupleFieldNames(root, content)
	for tn, names := range fieldNames {
		if len(fieldOrder[tn]) == 0 && len(names) > 0 {
			fieldOrder[tn] = names
		}
	}
	pythonMergeNamedtupleFields(root, content, fieldIndex)
	// typing.NamedTuple("Box", [("a", A), ...]) / NamedTuple("Box", a=A, b=B).
	pythonMergeFunctionalNamedTupleFields(root, content, fieldIndex)
	// Type-level Class.field → T so BoxA(A()).a / BoxA(A()).item peels without
	// a typed local (dual-class under foreign same-leaf).
	for tn, fields := range fieldIndex {
		for f, t := range fields {
			if tn != "" && f != "" && t != "" {
				fieldOf[tn+"."+f] = t
			}
		}
	}
	// bindFields: annotated/named fields + ordered namedtuple index aliases.
	// Also synthetic `@astuple.local.#i` slots for direct astuple(local)[i]
	// (fieldOrder; not local.#i so dataclasses stay non-indexable).
	bindFields := func(local, typeName string) {
		pythonBindClassLocalFields(local, typeName, fieldIndex, fieldOf)
		pythonBindNamedtupleIndexFields(local, typeName, fieldNames, fieldIndex, fieldOf)
		pythonBindNamedtupleIndexFields("@astuple."+local, typeName, fieldOrder, fieldIndex, fieldOf)
	}
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
							bindFields(lname, tn)
							if ourReceivers[tn] {
								out[lname] = true
							}
						}
						// Record even foreign element types so a later `items: list[B]`
						// shadows a prior `items: list[A]` (file-global map).
						// Optional[list[A]] / list[A] | None unwrap to the collection ann.
						ann := pythonUnwrapOptionalTypeNode(typeN, content)
						if et := pythonContainerElemType(ann, content); et != "" {
							elemOf[lname] = et
						}
						// Mapping of list/set of T: defaultdict[str, list[A]] → nested A
						// so da["k"][0].run() / for a in da["k"] peel under foreign same-leaf.
						// Collection of list/set: list[list[A]] / deque[list[A]] → nested A
						// so aa[0][0].run() / for row in aa; for a in row peel.
						// Optional[list[A]] / Optional[dict[str, list[A]]] unwrap first.
						if nest := pythonMappingNestedListElemType(ann, content); nest != "" {
							elemOf["@nested."+lname] = nest
						} else if nest := pythonCollectionNestedListElemType(ann, content); nest != "" {
							elemOf["@nested."+lname] = nest
						}
					}
				}
			}
		case "assignment":
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			typeN := ingest.ChildByField(n, "type")
			// a = await qa.get() — peel await so collection/object peels apply.
			for right != nil && right.Type() == "await" {
				right = pythonAwaitArg(right)
			}
			if left != nil && left.Type() == "identifier" {
				lname := ingest.NodeText(left, content)
				if typeN != nil {
					if tn := pythonTypeName(typeN, content); tn != "" {
						// x: A / x: B = ... — foreign too for shadowing.
						typeOf[lname] = tn
						bindFields(lname, tn)
						if ourReceivers[tn] {
							out[lname] = true
						}
					}
					// Foreign element types too — shadow prior same-name collections.
					// Optional[list[A]] / list[A] | None unwrap to the collection ann.
					ann := pythonUnwrapOptionalTypeNode(typeN, content)
					if et := pythonContainerElemType(ann, content); et != "" {
						elemOf[lname] = et
					}
					// Mapping of list/set of T (same as typed_parameter path).
					// Collection of list/set: list[list[A]] (same as typed_parameter path).
					// Optional[list[A]] / Optional[dict[str, list[A]]] unwrap first.
					if nest := pythonMappingNestedListElemType(ann, content); nest != "" {
						elemOf["@nested."+lname] = nest
					} else if nest := pythonCollectionNestedListElemType(ann, content); nest != "" {
						elemOf["@nested."+lname] = nest
					}
				}
				// xa = a if c else x / xa = A() if c else A() — both arms agree on T.
				// xa = ba.get() if c else ba.get() — method-return arms under foreign
				// same-leaf (pythonObjectLeafType peels ternary + method-return).
				// Foreign too for shadowing (xb = b if c else y / mrB = bb.get() if c …).
				if right != nil {
					if tn := pythonObjectExprType(right, content, typeOf); tn != "" {
						typeOf[lname] = tn
						bindFields(lname, tn)
						if ourReceivers[tn] {
							out[lname] = true
						}
					} else if tn := pythonObjectLeafType(right, content, funcReturns, typeOf, fieldOf); tn != "" {
						// Method-return / factory peels not covered by Class()-only
						// pythonObjectExprType (e.g. ba.get() if c else ba.get()).
						typeOf[lname] = tn
						bindFields(lname, tn)
						if ourReceivers[tn] {
							out[lname] = true
						}
					}
				}
				if right != nil && right.Type() == "call" {
					fn := ingest.ChildByField(right, "function")
					// pa = partial(A) / functools.partial(A) — factory local (not an A
					// instance). Call result pa() is A under foreign same-leaf.
					// Foreign factories too for shadowing (pb = partial(B)).
					// pa = partial(ba.get) — bound-method partial; pa() yields A.
					if et := pythonPartialFactoryClassType(right, content); et != "" {
						factoryOf[lname] = et
					} else if et := pythonPartialBoundMethodReturnType(right, content, funcReturns, typeOf, fieldOf); et != "" {
						factoryOf[lname] = et
					}
					// ra = weakref.ref(a) — factory local; ra() peels as A.
					// Foreign too for shadowing (rb = weakref.ref(b)).
					if et := pythonWeakrefRefFactoryType(right, content, typeOf); et != "" {
						factoryOf[lname] = et
					}
					// pa = weakref.proxy(a) / weakref.proxy(ba.get()) — referent object type
					// (not a factory; method-return peels under foreign same-leaf).
					if et := pythonWeakrefFactoryName(right, content); et == "proxy" {
						if tn := pythonWeakrefCallObjectType(right, content, typeOf, funcReturns, fieldOf); tn != "" {
							typeOf[lname] = tn
							bindFields(lname, tn)
							if ourReceivers[tn] {
								out[lname] = true
							}
						}
					}
					// ga = attrgetter("a") / operator.attrgetter("a") /
					// gi = itemgetter(0) / itemgetter("a") — stored operator getter
					// (not a field value). Application ga(box) peels via getterOf.
					if spec := pythonOperatorGetterLocalSpec(right, content); spec != "" {
						getterOf[lname] = spec
					}
					// da = SimpleNamespace(k=A()) / types.SimpleNamespace(k=A()) —
					// bind fieldOf["da.k"] + @astuple.da.#i so da.k.run() /
					// vars(da)["k"] / for x in vars(da).values() peel under foreign
					// same-leaf. Dedicated path (not namedtuple fieldIndex): kwargs
					// are attributes, not invented ctor fields.
					// Foreign fields too for shadowing (db = SimpleNamespace(k=B())).
					pythonBindSimpleNamespaceLocal(lname, right, content, fieldOf, funcReturns, typeOf)
					// a = A.make() / a = A.create() — @staticmethod / @classmethod
					// factory (attribute callee; "Class.method" keys in funcReturns).
					// Bare make_a() is handled in the identifier branch below too.
					if fn != nil && fn.Type() == "attribute" {
						if rt := pythonCallFuncReturnType(right, content, funcReturns, typeOf, fieldOf); rt != "" {
							typeOf[lname] = rt
							bindFields(lname, rt)
							if ourReceivers[rt] {
								out[lname] = true
							}
						}
					}
					if fn != nil && fn.Type() == "identifier" {
						fname := ingest.NodeText(fn, content)
						if ourReceivers[fname] {
							// x = A() — Class() ctor of our receiver.
							out[lname] = true
							typeOf[lname] = fname
							bindFields(lname, fname)
						} else if len(fieldIndex[fname]) > 0 || len(fieldNames[fname]) > 0 {
							// x = Box(...) — namedtuple/class with known fields (not our
							// receiver); bind fieldOf so box.a.run() / xa = box.a work.
							typeOf[lname] = fname
							bindFields(lname, fname)
							// Instance-level override from this call's Class() args.
							// Type-level fieldIndex is last-writer-wins across
							// ba=Box(A()); bb=Box(B()) and would under-rename A.
							// Foreign fields too for shadowing.
							if fields := pythonNamedtupleCtorInstanceFields(right, content, fieldNames[fname]); len(fields) > 0 {
								for f, t := range fields {
									if f != "" && t != "" {
										fieldOf[lname+"."+f] = t
									}
								}
								if names := fieldNames[fname]; len(names) > 0 {
									for i, fnameN := range names {
										if t := fields[fnameN]; t != "" {
											fieldOf[lname+".#"+fmt.Sprintf("%d", i)] = t
										}
									}
								}
							}
						}
						// a = make_a() after def make_a() -> A / @lru_cache def make_a() -> A /
						// make_a = lambda: A(). Foreign returns too for shadowing.
						if rt := funcReturns[fname]; rt != "" {
							typeOf[lname] = rt
							bindFields(lname, rt)
							if ourReceivers[rt] {
								out[lname] = true
							}
						}
						// a = cast(A, x) / cast("A", x)
						if fname == "cast" {
							if tn := pythonCastTypeArg(right, content); ourReceivers[tn] {
								out[lname] = true
							}
						}
						// a = pa() after pa = partial(A) — Class() result of factory local.
						if et := factoryOf[fname]; ourReceivers[et] {
							out[lname] = true
							typeOf[lname] = et
							bindFields(lname, et)
						}
						// xa = ga(box) after ga = attrgetter("a") /
						// a = gi(items) after gi = itemgetter(0) — stored operator
						// getter application (same leaf as inline getter call).
						// Also a = gi([ba.get()]) / xa = ga(w.get()) under foreign same-leaf.
						if ft := pythonStoredOperatorGetterType(right, content, getterOf, fieldOf, elemOf, egElems, typeOf, funcReturns); ft != "" {
							spec := getterOf[fname]
							if !strings.HasSuffix(spec, ":#") {
								typeOf[lname] = ft
								bindFields(lname, ft)
							}
							if ourReceivers[ft] {
								out[lname] = true
							}
						}
						// a = next(iter(items)) / next(items) / next(x for x in items) /
						// next(reversed(items)) — result type is the element type of
						// the iterable arg (identity genexp preserves that type).
						// a = next(iter(astuple(box))) — first declaration-order field
						// of the heterogeneous field tuple (same leaf as astuple(box)[0]).
						// pair = next(pairs) when pairs = zip/enumerate(...) — pair is a
						// tuple (pairSlots + shared elemOf), not an element; use pair[i] /
						// unpack / nested for a in pair.
						if fname == "next" {
							if types := pythonNextPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if types := pythonObjectNextPairSlots(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
								// pair = next(enumerate([ba.get()])) / next(zip([ba.get()], …))
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if types := pythonItemsCallPairSlots(right, content, elemOf, fieldOf); len(types) > 0 {
								// p = next(asdict(pair).items()) / next(d.items()) —
								// pair local (key untyped, value leaf); use p[1] / unpack.
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonNextElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							} else if et := pythonAstupleNextFirstField(right, content, fieldOf); ourReceivers[et] {
								out[lname] = true
							} else if args, ok := pythonCallPositionalArgNodes(right); ok && len(args) > 0 {
								// a = next(iter([ba.get()])) / next(dict(k=ba.get()).values()) /
								// a = next(x for x in [ba.get()])
								if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									out[lname] = true
								}
							}
						}
						// a = min(items) / max(items) / min(items, key=...) —
						// single-iterable form yields an element of that iterable.
						// Multi-arg min(a, b, ...) fails closed (not an iterable fold).
						// a = min(asdict(pair).values(), key=...) / max(astuple(pair)) —
						// homogeneous field values only (mixed fail closed).
						// pair = min(pairs) when pairs is a pair-iter (list(zip(...)), …) —
						// pair is a tuple (pairSlots + shared elemOf), not an element.
						if fname == "min" || fname == "max" {
							if types := pythonMinMaxPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if types := pythonObjectMinMaxPairSlots(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
								// pair = min(list(zip([ba.get()], …))) — object pair under foreign same-leaf.
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if types := pythonItemsCallPairSlots(right, content, elemOf, fieldOf); len(types) > 0 {
								// p = min(asdict(pair).items(), key=...) / max(d.items()) —
								// pair local (same path as next(...items())).
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonMinMaxElemType(right, content, elemOf, egElems, typeOf, fieldOf); ourReceivers[et] {
								out[lname] = true
							} else if args, ok := pythonCallPositionalArgNodes(right); ok && len(args) == 1 {
								// a = min([ba.get()], key=...) / max(dict(k=ba.get()).values())
								if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									out[lname] = true
								}
							}
						}
						// a = choice(items) — from random import choice; element of seq.
						// pair = choice(pairs) when pairs is a pair-iter (pairSlots + shared
						// elemOf), not an element; use pair[i] / unpack / nested for.
						if fname == "choice" {
							if types := pythonChoicePairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if types := pythonObjectChoicePairSlots(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
								// pair = choice(list(zip([ba.get()], …))) — object pair under foreign same-leaf.
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonRandomChoiceElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							} else if args, ok := pythonCallPositionalArgNodes(right); ok && len(args) > 0 {
								// a = choice([ba.get()])
								if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									out[lname] = true
								}
							}
						}
						// a = heappop(items) / heappushpop(items, x) / heapreplace(items, x)
						// — from heapq import …; element of heap (1st arg).
						// pair = heappop(pairs) when pairs is a pair-iter (pairSlots + shared
						// elemOf), not an element; use pair[i] / unpack / nested for.
						if fname == "heappop" || fname == "heappushpop" || fname == "heapreplace" {
							if types := pythonHeappopPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if types := pythonObjectHeappopPairSlots(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
								// pair = heappop(list(zip([ba.get()], …))) — object pair under foreign same-leaf.
								pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
							} else if et := pythonHeappopElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							} else if args, ok := pythonCallPositionalArgNodes(right); ok && len(args) > 0 {
								// a = heappop([ba.get()])
								if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									out[lname] = true
								}
							}
						}
						// a = reduce(fn, items) / reduce(fn, items, init) — from functools
						// import reduce; result type is the iterable element (fold of same
						// leaf). Multi-arg without iterable fails closed inside helper.
						if fname == "reduce" {
							if et := pythonReduceElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							} else if args, ok := pythonCallPositionalArgNodes(right); ok && len(args) >= 2 {
								// a = reduce(fn, [ba.get()])
								if et := pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									out[lname] = true
								}
							}
						}
						// a = copy(item) / deepcopy(item) — from copy import copy, deepcopy;
						// preserve object type of the single arg (same as copy.copy(item)).
						// a = copy(box.a) / copy(asdict(box)["a"]) — field / dict-view key (fieldOf).
						// a = copy(ba.get()) — method-return under foreign same-leaf.
						// Collection form xs = copy(items) is handled via elemOf below.
						if fname == "copy" || fname == "deepcopy" {
							if tn := pythonCopyCallObjectType(right, content, typeOf, fieldOf, funcReturns); tn != "" {
								typeOf[lname] = tn
								bindFields(lname, tn)
								if ourReceivers[tn] {
									out[lname] = true
								}
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
							case "result":
								// xa = fa.result() after fa.set_result(A()) — Future result
								// leaf via futureOf. Also ex.submit(lambda: A()).result().
								// Timeout args ignored. Other receivers fail closed.
								if et := pythonFutureResultCallType(right, content, futureOf); ourReceivers[et] {
									out[lname] = true
									typeOf[lname] = et
									bindFields(lname, et)
								}
							case "pop", "popleft", "get", "get_nowait", "setdefault":
								// a = items.pop() / items.pop(0) / d.pop(k) / list(items).pop()
								// a = items.popleft() (deque) — element type of receiver.
								// a = d.get(k) / d.get(k, default) — element/value type of
								// the receiver collection (dict value leaf via elemOf).
								// a = qa.get_nowait() / await qa.get() — Queue element (same E).
								// a = box.get("a") / box.pop("a") / box.setdefault("a") —
								// TypedDict/record string-key value via fieldOf (key-specific).
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
								if ft := pythonRecordKeyAccessType(right, content, fieldOf); ft != "" {
									typeOf[lname] = ft
									bindFields(lname, ft)
									if ourReceivers[ft] {
										out[lname] = true
									}
									break
								}
								// a = asdict(box).get("a") / vars(box).get("a") /
								// box.__dict__.get("a") — dict-view field keys.
								if ft := pythonDictViewKeyAccessType(right, content, fieldOf, funcReturns, typeOf); ft != "" {
									typeOf[lname] = ft
									bindFields(lname, ft)
									if ourReceivers[ft] {
										out[lname] = true
									}
									break
								}
								if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
									out[lname] = true
								} else if et := pythonObjectIterableElemType(obj, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									// a = deque([ba.get()]).popleft() / [ba.get()].pop() —
									// method-return object collection element under foreign same-leaf.
									out[lname] = true
									typeOf[lname] = et
									bindFields(lname, et)
								} else if et := pythonHomogeneousObjectDictValue(obj, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									// a = {"k": ba.get()}.get("k") / dict(k=ba.get()).get("k")
									out[lname] = true
								} else if et := pythonObjectDictMergeValueType(obj, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									// a = ({"k": ba.get()} | {}).get("k") / ({} | {"k": ba.get()}).get("k")
									// — object-dict merge value under foreign same-leaf (inline
									// already peels; Class merge peels via pythonIterableElemType).
									out[lname] = true
								} else if et := pythonMappingProxyObjectValueType(obj, content, elemOf, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									// a = MappingProxyType({"k": ba.get()}).get("k") /
									// types.MappingProxyType({"k": ba.get()}).get("k") —
									// proxy of object-dict value under foreign same-leaf (inline
									// already peels; Class proxy peels via pythonIterableElemType).
									out[lname] = true
								} else if et := pythonHomogeneousDictValueCtorElem(obj, content); ourReceivers[et] {
									// a = {"k": A()}.get("k") / dict(k=A()).get("k")
									out[lname] = true
								} else if ingest.NodeText(attr, content) == "setdefault" {
									// a = da.setdefault("k", A()) / da.setdefault("k", ba.get()) —
									// unannotated dict: peel Class()/method-return default and bind
									// elemOf so da["k"] peels under foreign same-leaf.
									if et := pythonSetdefaultDefaultClassType(right, content, funcReturns, typeOf, fieldOf); et != "" {
										if obj != nil && obj.Type() == "identifier" {
											elemOf[ingest.NodeText(obj, content)] = et
										}
										if ourReceivers[et] {
											out[lname] = true
										}
									}
								}
							case "copy", "deepcopy":
								// a = copy.copy(item) / copy.deepcopy(item) — preserve object
								// type of the single arg (typed local / Class()).
								// a = copy.copy(box.a) / copy.copy(replace(box).a) — field leaf.
								// a = copy.copy(asdict(box)["a"]) — dict-view field key (fieldOf).
								// a = copy.copy(ba.get()) — method-return under foreign same-leaf.
								// Collection form xs = copy.copy(items) is handled via elemOf below.
								if tn := pythonCopyCallObjectType(right, content, typeOf, fieldOf, funcReturns); tn != "" {
									typeOf[lname] = tn
									bindFields(lname, tn)
									if ourReceivers[tn] {
										out[lname] = true
									}
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
							case "__getitem__":
								// a = items.__getitem__(i) / d.__getitem__(k) /
								// list(items).__getitem__(0) — same element/value leaf as
								// items[i] / d[k] (key/index arg ignored for typing).
								obj := ingest.ChildByField(fn, "object")
								if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
									out[lname] = true
								} else if et := pythonObjectIterableElemType(obj, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
									// a = [ba.get()].__getitem__(0) — method-return collection.
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
					// xa = itemgetter("a")(box) / itemgetter("a")(asdict(box)) —
					// TypedDict/record or dict-view string-key (fieldOf).
					if ft := pythonItemgetterFieldType(right, content, fieldOf, funcReturns, typeOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if types := pythonItemgetterPairSlots(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
						pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
					} else if et := pythonItemgetterElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
						out[lname] = true
					} else if et := pythonItemgetterObjectElemType(right, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
						// a = itemgetter(0)([ba.get()]) / itemgetter("k")({"k": ba.get()})
						out[lname] = true
					}
					// a = getitem(items, i) / operator.getitem(items, i) /
					// getitem(d, k) — same element/value leaf as items[i] / d[k].
					if et := pythonGetitemElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
						out[lname] = true
					} else if et := pythonGetitemObjectElemType(right, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
						// a = getitem([ba.get()], 0) / operator.getitem([ba.get()], 0)
						out[lname] = true
					}
					// xa = attrgetter("a")(box) / attrgetter("a")(replace(box)) /
					// operator.attrgetter("a")(box) — single-field getter on a typed
					// local (or replace of it) yields the field type (same as box.a).
					// Multi-field attrgetter fails closed.
					if ft := pythonAttrgetterFieldType(right, content, fieldOf, funcReturns, typeOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					}
					// xa = getattr(box, "a") — builtin field access (same leaf as box.a).
					if ft := pythonGetattrFieldType(right, content, fieldOf, funcReturns, typeOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					}
					// new = replace(box) / dataclasses.replace(box) — same object type
					// as first positional arg (fieldOf for new.a.run() under foreign
					// same-leaf methods). Method-return first arg (replace(ba.get()))
					// peels via funcReturns (Class peels via ctor). Keyword field
					// rewrites stay in ExtraRename.
					if tn := pythonReplaceCallObjectType(right, content, typeOf, fieldOf, funcReturns); tn != "" {
						typeOf[lname] = tn
						bindFields(lname, tn)
						if ourReceivers[tn] {
							out[lname] = true
						}
					}
					// d = asdict(box) / dataclasses.asdict(box) — dict of field values
					// (fieldOf for d["a"].run() under foreign same-leaf methods).
					// Not an object of the dataclass type (no typeOf / out).
					if tn := pythonAsdictCallObjectType(right, content, typeOf); tn != "" {
						bindFields(lname, tn)
					}
					// d = vars(box) — same field keys as asdict (obj.__dict__).
					// typeOf path for annotated dataclasses; always also copy
					// instance fieldOf for SNS (no typeOf / fieldIndex). Idempotent
					// when bindFields already recorded the same keys.
					if tn := pythonVarsCallObjectType(right, content, typeOf); tn != "" {
						bindFields(lname, tn)
					}
					// d = vars(box) / d = asdict(box) / d = ba._asdict() — copy instance
					// fieldOf for dual-class namedtuple/SNS (type-level bindFields is
					// last-writer-wins across ba=Box(A()); bb=Box(B())).
					if src := pythonDictViewObjectLocal(right, content); src != "" {
						pythonCopyLocalFieldOf(src, lname, fieldOf)
					}
					// ra = ba._replace(...) — same namedtuple instance fields with
					// keyword overrides (dual-class under foreign same-leaf).
					if pythonBindNamedtupleReplaceLocal(lname, right, content, typeOf, fieldOf, fieldNames, fieldIndex, ourReceivers, out) {
						// bound
					}
					// ba = Box._make([A()]) — namedtuple from iterable; instance fields
					// from sequence elements (dual-class under foreign same-leaf).
					if pythonBindNamedtupleMakeLocal(lname, right, content, typeOf, fieldOf, fieldNames, fieldIndex, ourReceivers, out) {
						// bound
					}
					// t = astuple(box) / dataclasses.astuple(box) — tuple of field
					// values in declaration order (fieldOf["t.#i"] for t[i].run()).
					// Index slots only — plain tuple has no named keys / attrs.
					if tn := pythonAstupleCallObjectType(right, content, typeOf); tn != "" {
						pythonBindNamedtupleIndexFields(lname, tn, fieldOrder, fieldIndex, fieldOf)
					}
				}
				// a = items[0] / a = d[k] / a = list(items)[0] — element/value of collection.
				// a = item[1] when item from enumerate/zip pair (pairSlots).
				// a = pairs[0][0] / a = list(zip(...))[0][0] — double subscript slot.
				// pair = pairs[0] / pair = list(zip(...))[0] — index into pair-iter binds
				// pairSlots (+ elemOf when slots share type, for nested for/next).
				// xa = box["a"] — TypedDict/record string-key value (fieldOf).
				// xa = asdict(box)["a"] / vars(box)["a"] / box.__dict__["a"] — dict-view
				// field keys (same leaf as d = asdict(box); xa = d["a"]).
				// xa = astuple(box)[0] — declaration-order index (same leaf as t[0]).
				// Slices (items[1:3]) fail closed (sequence, not element).
				if right != nil && right.Type() == "subscript" {
					if types := pythonPairSlotsOf(right, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots); len(types) > 0 {
						// Foreign slots too — shadow prior same-name pair locals.
						pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
					} else if types := pythonObjectPairSlotsOf(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
						// pair = list(enumerate([ba.get()]))[0] / list(zip([ba.get()], …))[0]
						// — object pair-iter index under foreign same-leaf.
						pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
					} else if ft := pythonRecordKeyAccessType(right, content, fieldOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonDictViewKeyAccessType(right, content, fieldOf, funcReturns, typeOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonAstupleIndexAccessType(right, content, fieldOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonNamedtupleIndexType(right, content, fieldOf); ft != "" {
						// xa = box[0] — namedtuple positional field (same leaf as box.a).
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonNamedtupleMakeIndexType(right, content); ft != "" {
						// xa = Box._make([A()])[0] — make sequence element peel.
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if et := pythonSubscriptElemType(right, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots, fieldOf); ourReceivers[et] {
						out[lname] = true
					} else if et := pythonObjectPairSlotSubscriptType(right, content, funcReturns, typeOf, fieldOf); et != "" {
						// xa = list(enumerate([ba.get()]))[0][1] / list(zip(...))[0][0]
						// — object pair-slot double subscript under foreign same-leaf.
						typeOf[lname] = et
						bindFields(lname, et)
						if ourReceivers[et] {
							out[lname] = true
						}
					} else if et := pythonObjectSubscriptElemType(right, content, funcReturns, typeOf, fieldOf); et != "" {
						// xa = sorted([ba.get()])[0] / [ba.get()][0] / list([ba.get()])[0]
						// — method-return collection element under foreign same-leaf
						// (direct sorted([ba.get()])[0].run() already peels; assigned
						// form needs typeOf bind). Foreign too for shadowing.
						typeOf[lname] = et
						bindFields(lname, et)
						if ourReceivers[et] {
							out[lname] = true
						}
					}
				}
				// xa = box.a — dataclass/class field access when box is a typed local
				// with annotated field a: A (under foreign same-leaf methods).
				// xa = replace(box).a / dataclasses.replace(box).a — field of first arg.
				// d = box.__dict__ — same field keys as vars/asdict (not a field leaf).
				// typeOf path for annotated classes; always also copy instance
				// fieldOf for SNS (no typeOf / fieldIndex).
				if right != nil && right.Type() == "attribute" {
					if tn := pythonDunderDictObjectType(right, content, typeOf); tn != "" {
						bindFields(lname, tn)
					}
					if src := pythonDictViewObjectLocal(right, content); src != "" {
						pythonCopyLocalFieldOf(src, lname, fieldOf)
					}
					if ft := pythonFieldAccessType(right, content, fieldOf, funcReturns, typeOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonReplaceFieldAccessType(right, content, fieldOf, funcReturns, typeOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonNamedtupleReplaceFieldAccessType(right, content, fieldOf); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonNamedtupleMakeFieldAccessType(right, content); ft != "" {
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					} else if ft := pythonInlineSimpleNamespaceObjectFieldType(right, content, funcReturns, typeOf, fieldOf); ft != "" {
						// mrA = SimpleNamespace(k=ba.get()).k / types.SimpleNamespace —
						// inline SNS kwargs (Class() or method-return) under foreign
						// same-leaf. Direct .run() already peels; assigned form needs
						// typeOf bind. Foreign too for shadowing.
						typeOf[lname] = ft
						bindFields(lname, ft)
						if ourReceivers[ft] {
							out[lname] = true
						}
					}
				}
				// xs = [A()] / (A(),) / [B()] — track element type for later for-loops.
				// xs = xs + [A()] / xs = [*xs, A()] / zs = xs + ys — assign-concat /
				// star-list peels (self-target untyped arms are wildcards).
				// aa = [[A()]] / ((A(),),) / [{"k": A()}] — nested list/tuple/dict-row
				// local of leaf A (@nested) so aa[0][0].run() / la[0]["k"].run() /
				// match aa: case [[xa]]: peel under foreign same-leaf.
				// da = {"k": [A()]} / {"k": (A(),)} / {"k": {A()}} / {"k": frozenset([A()])} /
				// {"k": deque([A()])} / {"outer": {"k": A()}} — mapping of list/tuple/set/
				// frozenset/deque/dict of leaf A (@nested); also dict(k=[A()]) /
				// OrderedDict(k=[A()]) / ChainMap({"k": [A()]}) / dict([("k", [A()])]) /
				// dict({"k": [A()]}) / {k: [A()] for k in ...} so da["k"][0].run() /
				// for a in da["k"] / match da: case {"k": [xa]}: peel.
				// da = {"k": A()} / dict(k=A()) / OrderedDict(k=A()) / ChainMap({"k": A()}) /
				// {k: A() for k in ...} — scalar mapping values of A (elemOf).
				// xs = list(items) / UserList(items) / filter(...) — preserve element type.
				// d = dict.fromkeys(keys, A()) — value leaf is A (for .values/.get).
				// pairs = zip/enumerate/product/pairwise(...) /
				// pairs = list/tuple/iter/reversed/sorted/filter(...zip...) —
				// combos = combinations/permutations/batched(...) (literal r/n) — pair-iter slots.
				// groups = groupby(items) / itertools.groupby(...) — @groupby local
				// (not pair-iter slots: 2nd yield is a group iterator, not an element).
				if right != nil {
					if types := pythonPairIterSlotsOf(right, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
						// Foreign slots too — shadow prior same-name pair-iters.
						pairIterSlots[lname] = types
					} else if types := pythonObjectPairIterSlotsOf(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
						// pairs = list(enumerate([ba.get()])) / list(zip([ba.get()], …))
						// — object pair-iter under foreign same-leaf (reuse pairIterSlots
						// so pairs[0] / pairs[0][1] peels via Class subscript path).
						pairIterSlots[lname] = types
					} else if et := pythonGroupbyGroupElemType(right, content, elemOf, egElems, typeOf); et != "" {
						// groups = groupby(items) / groups = itertools.groupby(...) /
						// groups = iter(groupby(items)) / groups2 = groups —
						// group-iterator local (key, group) yields.
						// Store under @groupby.* so next(groups) / for k, g in groups
						// recover the group element leaf. Do not put into elemOf[lname]
						// (groups itself is not an element collection).
						// Foreign element types too — shadow prior same-name groupbys.
						elemOf["@groupby."+lname] = et
					} else if et := pythonNextGroupbyGroupSubElemType(right, content, elemOf, egElems, typeOf); et != "" {
						// ga = next(groupby(items))[1] / ga = next(groups)[1] —
						// group iterator (same leaf as _, ga = next(groupby(...))).
						// Bind into elemOf so next(ga) / for a in ga type.
						// Foreign too for shadowing.
						elemOf[lname] = et
					} else if et := pythonDictFromkeysObjectValueType(right, content, funcReturns, typeOf, fieldOf); et != "" {
						elemOf[lname] = et
					} else if et := pythonMappingProxyObjectValueType(right, content, elemOf, funcReturns, typeOf, fieldOf); et != "" {
						// pa = MappingProxyType({"k": ba.get()}) — proxy value leaf.
						elemOf[lname] = et
					} else if et := pythonHomogeneousCtorElem(right, content); et != "" {
						elemOf[lname] = et
					} else if et := pythonObjectCollectionElem(right, content, funcReturns, typeOf, fieldOf); et != "" {
						// xs = [ba.get()] / xs = (ba.get(),) /
						// xs = [ba.get()] + [ba.get()] / xs = list([ba.get()]) /
						// xs = [ba.get()] or [ba.get()] — method-return / typed-local
						// collection peels under foreign same-leaf.
						// Dict literals intentionally excluded (see
						// pythonHomogeneousObjectDictValue after nested peels).
						elemOf[lname] = et
					} else if et := pythonListConcatElemType(right, content, elemOf, egElems, typeOf, lname); et != "" {
						// xs = xs + [A()] / zs = xs + [A()] / xs = [] + [A()] —
						// self-target untyped arms are wildcards (assign-concat).
						elemOf[lname] = et
					} else if et := pythonListRepeatElemType(right, content, elemOf, egElems, typeOf); et != "" {
						// xs = [A()] * n / n * [A()] — list/tuple repeat element type.
						elemOf[lname] = et
					} else if et := pythonHomogeneousSplatListCtorElem(right, content, elemOf, egElems, typeOf, lname); et != "" {
						// xs = [*xs, A()] / zs = [*xs, A()] — star-list peels.
						elemOf[lname] = et
					} else if nest := pythonNestedHomogeneousCtorElem(right, content); nest != "" {
						// aa = [[A()]] / [{"k": A()}] — not a scalar list of A; store nested leaf.
						// Foreign too for shadowing (bb = [[B()]] after aa = [[A()]]).
						elemOf["@nested."+lname] = nest
					} else if nest := pythonNestedDictHomogeneousListCtorElem(right, content); nest != "" {
						// da = {"k": [A()]} / OrderedDict(k=[A()]) / ChainMap({"k":[A()]}) /
						// {k: [A()] for k in ...} / {"outer": {"k": A()}} — not scalar
						// dict values of A; store nested leaf.
						// Foreign too for shadowing (db = {"k": [B()]} after da).
						elemOf["@nested."+lname] = nest
					} else if nest := pythonDictStarCopyNestedValueType(right, content, elemOf); nest != "" {
						// ca = {**da} / {**da, **ea} / {**da, "j": [A()]} when da is
						// dict[str, list[A]] (@nested) — preserve nested leaf so
						// ca["k"][0].run() peels under foreign same-leaf.
						// Foreign too for shadowing (cb = {**db} after ca).
						elemOf["@nested."+lname] = nest
					} else if nest := pythonNestedMappingCopyCallType(right, content, elemOf); nest != "" {
						// ca = da.copy() / da.deepcopy() when da is nested mapping of A —
						// preserve @nested leaf (scalar .copy peels via elemOf below).
						// Foreign too for shadowing.
						elemOf["@nested."+lname] = nest
					} else if nest := pythonDictMergeNestedValueType(right, content, elemOf); nest != "" {
						// ca = da | ea / da | {"j": [A()]} when da is nested mapping of A —
						// preserve @nested leaf so ca["k"][0].run() peels.
						// Foreign too for shadowing.
						elemOf["@nested."+lname] = nest
					} else if nest := pythonChainMapLocalNestedValueType(right, content, elemOf); nest != "" {
						// ca = ChainMap(da) / ChainMap(da, ea) when da is nested mapping —
						// preserve @nested leaf (literal ChainMap peels above).
						// Foreign too for shadowing.
						elemOf["@nested."+lname] = nest
					} else if nest := pythonChainMapNewChildNestedValueType(right, content, elemOf); nest != "" {
						// ca = ChainMap(da).new_child({"j": [A()]}) / ca.new_child() nested —
						// preserve @nested leaf. Foreign too for shadowing.
						elemOf["@nested."+lname] = nest
					} else if et := pythonHomogeneousObjectDictValue(right, content, funcReturns, typeOf, fieldOf); et != "" {
						// da = {"k": ba.get()} / da = {"k": a} — method-return /
						// typed-local scalar dict values under foreign same-leaf.
						// After nested peels so {"k": frozenset([A()])} stays @nested.
						elemOf[lname] = et
					} else if et := pythonHomogeneousDictValueCtorElem(right, content); et != "" {
						// da = {"k": A()} / dict(k=A()) / OrderedDict(k=A()) /
						// ChainMap({"k": A()}) / {k: A() for k in ...} — scalar values.
						// Foreign too for shadowing (db = {"k": B()} after da).
						elemOf[lname] = et
					} else if et := pythonObjectDictMergeValueType(right, content, funcReturns, typeOf, fieldOf); et != "" {
						// ca = {"k": ba.get()} | {} / da | {"j": ba.get()} — object-dict
						// merge value under foreign same-leaf (Class peels below).
						// Foreign too for shadowing.
						elemOf[lname] = et
					} else if et := pythonDictMergeValueType(right, content, elemOf); et != "" {
						// ca = da | ea / da | {"j": A()} / {} | {"k": A()} — scalar merge.
						// Foreign too for shadowing.
						elemOf[lname] = et
					} else if et := pythonChainMapLocalValueType(right, content, elemOf); et != "" {
						// ca = ChainMap(da) when da is scalar mapping of A.
						// Foreign too for shadowing.
						elemOf[lname] = et
					} else if et := pythonChainMapNewChildValueType(right, content, elemOf); et != "" {
						// ca = ChainMap(da).new_child({"j": A()}) / ca.new_child() scalar.
						// Foreign too for shadowing.
						elemOf[lname] = et
					} else if et := pythonIterableElemType(right, content, elemOf, egElems, typeOf); et != "" {
						elemOf[lname] = et
					} else if et := pythonObjectIterableElemType(right, content, funcReturns, typeOf, fieldOf); et != "" {
						// it = repeat(ba.get()) / it = cycle([ba.get()]) /
						// it = list(repeat(ba.get(), 1)) — method-return object
						// iterables under foreign same-leaf (Class peels above).
						// Enables next(it) / for x in it after bind.
						elemOf[lname] = et
					} else if et := pythonDictViewValuesHomogeneousType(right, content, fieldOf); et != "" {
						// xs = asdict(box).values() / list(asdict(box).values()) —
						// homogeneous field values only (mixed fail closed).
						elemOf[lname] = et
					} else if et := pythonAstupleHomogeneousType(right, content, fieldOf); et != "" {
						// xs = astuple(box) / list(astuple(box)) — homogeneous field
						// values only (mixed fail closed; index slots still bound above).
						elemOf[lname] = et
					}
					// d = da.data when da is UserDict of nested list values —
					// underlying .data shares @nested leaf (scalar .data peels
					// via pythonIterableElemType above). Foreign too for shadowing.
					if obj := pythonDataAttrObjectIdent(right, content); obj != "" {
						if nest := elemOf["@nested."+obj]; nest != "" {
							elemOf["@nested."+lname] = nest
						}
					}
				}
			}
			// da["k"] = A() / da[k] = A() / xs[0] = A() —
			// da["k"] = ba.get() / xs[0] = ba.get() — unannotated subscript assign.
			// Bind elemOf so da["k"].run() / xs[0].run() peels under foreign same-leaf.
			// Non-slice key only; foreign values too for shadowing.
			// Method-return peels via pythonObjectLeafType.
			if left != nil && left.Type() == "subscript" && right != nil {
				if local, et := pythonDictSubscriptAssignValueType(left, right, content, funcReturns, typeOf, fieldOf); local != "" && et != "" {
					elemOf[local] = et
				}
				if local, et := pythonListSliceAssignElemType(left, right, content, funcReturns, typeOf, fieldOf); local != "" && et != "" {
					// xs[:] = [A()] / xs[i:j] = [A()] /
					// xs[:] = [ba.get()] — list slice assign element peel.
					elemOf[local] = et
				}
			}
			// a, b = A(), B() / (a, b) = A(), B() / a, b = (A(), B()) /
			// [a, b] = [A(), B()] /
			// a, b = next(zip(xs, ys)) / a, b = next(pairs) / a, b = pair /
			// a, b = pairs[0] / a, b = list(zip(...))[0]
			// (pair-slot unpack; see pythonAssignPairUnpackTypes) /
			// xa, xb = astuple(box) / dataclasses.astuple(box) (declaration-order
			// field types; same leaf as t = astuple(box); xa = t[0]) /
			// xa, xb = asdict(box).values() / list(asdict(box).values()) /
			// d.values() after d = asdict(box) / vars / __dict__ (dict preserves
			// declaration order — same per-slot leaves as astuple unpack) /
			// k, a = d.popitem() (value leaf on 2nd slot; same as for k, a in d.items()) /
			// k, a = next(d.items()) / next(iter(d.items())) (typed dict value on 2nd) /
			// k, x = next(asdict(pair).items()) / next(iter(asdict(pair).items())) when
			// homogeneous field values (same leaf as for k, x in asdict(...).items()) /
			// it1, it2 = tee(items) / itertools.tee(items[, n]) (each → elemOf) /
			// _, ga = next(groupby(items)) / k, ga = next(itertools.groupby(items))
			// (ga → elemOf; key untyped; same leaf as for k, g in groupby) /
			// a, *rest = items / *rest, a = items / a, = items (items: list[A]) /
			// xa, *rest = astuple(box) / asdict(box).values() (fixed slots by
			// declaration order; *rest of mixed fails closed)
			if left != nil && right != nil {
				if targets := pythonPatternIdents(left, content); len(targets) > 0 {
					if types := pythonCtorListTypes(right, content); len(types) > 0 {
						for i, name := range targets {
							if i < len(types) && ourReceivers[types[i]] {
								out[name] = true
							}
						}
					} else if types := pythonObjectLeafListTypes(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
						// [mrA] = [ba.get()] / (mrA,) = (ba.get(),) / mrA, = [ba.get()] —
						// method-return list/tuple unpack under foreign same-leaf
						// (Class() peels via pythonCtorListTypes above).
						for i, name := range targets {
							if i < len(types) && types[i] != "" {
								typeOf[name] = types[i]
								bindFields(name, types[i])
								if ourReceivers[types[i]] {
									out[name] = true
								}
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
					} else if types := pythonObjectPairSlotsOf(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
						// a, b = list(zip([ba.get()], …))[0] / list(enumerate([ba.get()]))[0]
						// / next(zip([ba.get()], …)) — object pair under foreign same-leaf.
						for i, name := range targets {
							if i < len(types) && ourReceivers[types[i]] {
								out[name] = true
							}
						}
					} else if types := pythonObjectNextPairSlots(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
						// a, b = next(zip([ba.get()], …)) / next(enumerate([ba.get()]))
						// object pair-iter under foreign same-leaf.
						for i, name := range targets {
							if i < len(types) && ourReceivers[types[i]] {
								out[name] = true
							}
						}
					} else if types := pythonObjectEnumerateZipTargetTypes(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
						// a, b = zip([ba.get()], …) unlikely; covers direct pair-iter forms.
						for i, name := range targets {
							if i < len(types) && ourReceivers[types[i]] {
								out[name] = true
							}
						}
					} else if types := pythonAstupleFieldTypes(right, content, typeOf, fieldOrder, fieldIndex); len(types) > 0 {
						// xa, xb = astuple(box) / dataclasses.astuple(box) —
						// declaration-order field types (heterogeneous; not elemOf).
						for i, name := range targets {
							if i < len(types) && types[i] != "" {
								typeOf[name] = types[i]
								bindFields(name, types[i])
								if ourReceivers[types[i]] {
									out[name] = true
								}
							}
						}
					} else if types := pythonDictViewValuesFieldTypes(right, content, fieldOf); len(types) > 0 {
						// xa, xb = asdict(box).values() / list(asdict(box).values()) /
						// vars(box).values() / box.__dict__.values() /
						// d.values() after d = asdict(box) / vars / __dict__ —
						// declaration-order field types (same leaf as astuple unpack).
						for i, name := range targets {
							if i < len(types) && types[i] != "" {
								typeOf[name] = types[i]
								bindFields(name, types[i])
								if ourReceivers[types[i]] {
									out[name] = true
								}
							}
						}
					} else if vt := pythonDictPopitemValueType(right, content, elemOf); vt != "" {
						// k, a = d.popitem() — value type is elemOf[d] (dict value leaf).
						if len(targets) >= 2 && ourReceivers[vt] {
							out[targets[1]] = true
						}
					} else if vt := pythonNextItemsValueType(right, content, elemOf, fieldOf); vt != "" {
						// k, a = next(d.items()) / next(iter(d.items())) — typed dict value
						// leaf on 2nd slot; k, x = next(asdict(pair).items()) /
						// next(iter(asdict(pair).items())) when homogeneous field values
						// (same leaf as for k, x in asdict(...).items()).
						if len(targets) >= 2 && ourReceivers[vt] {
							out[targets[1]] = true
						}
					} else if vt := pythonMinMaxItemsValueType(right, content, elemOf, fieldOf); vt != "" {
						// k, x = min(asdict(pair).items(), key=...) / max(d.items()) —
						// value leaf on 2nd slot (same as next(...items())).
						if len(targets) >= 2 && ourReceivers[vt] {
							out[targets[1]] = true
						}
					} else if et := pythonTeeElemType(right, content, elemOf, egElems, typeOf, funcReturns, fieldOf); et != "" {
						// it1, it2 = tee(items) / itertools.tee(items[, n]) —
						// each target is an iterator of items elements (like groupby's g).
						// Also tee([ba.get()]) method-return object collections under
						// foreign same-leaf (Class/typed-local peels via iterable path).
						// Do not put targets into out (iterators, not elements).
						// Foreign element types too — shadow prior same-name collections.
						for _, name := range targets {
							elemOf[name] = et
						}
					} else if et := pythonNextGroupbyGroupElemType(right, content, elemOf, egElems, typeOf); et != "" {
						// _, ga = next(groupby(items)) / k, ga = next(itertools.groupby(items)) —
						// next yields (key, group); group is an iterable of items elements
						// (same leaf as for k, g in groupby(items): g). Key slot untyped.
						// Do not put ga into out (group is not an element).
						if len(targets) >= 2 {
							elemOf[targets[1]] = et
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
					// xa, *rest = astuple(box) / asdict(box).values() — fixed slots by
					// declaration order; *rest of mixed fails closed (no elemOf).
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
					} else if types := pythonAstupleFieldTypes(right, content, typeOf, fieldOrder, fieldIndex); len(types) > 0 {
						for i, name := range fixed {
							if i < len(types) && types[i] != "" {
								typeOf[name] = types[i]
								bindFields(name, types[i])
								if ourReceivers[types[i]] {
									out[name] = true
								}
							}
						}
					} else if types := pythonDictViewValuesFieldTypes(right, content, fieldOf); len(types) > 0 {
						for i, name := range fixed {
							if i < len(types) && types[i] != "" {
								typeOf[name] = types[i]
								bindFields(name, types[i])
								if ourReceivers[types[i]] {
									out[name] = true
								}
							}
						}
					}
				}
			}
		case "augmented_assignment":
			// xs += [A()] / xs += (A(),) / xs += ys / xs += [*ys, A()] —
			// mutation-via-assign peels under foreign same-leaf (same leaf as
			// xs.append(A()) / xs.extend([A()])). Only += ; other ops fail closed.
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			op := ingest.ChildByField(n, "operator")
			if left != nil && right != nil && left.Type() == "identifier" &&
				op != nil && ingest.NodeText(op, content) == "+=" {
				lname := ingest.NodeText(left, content)
				if et := pythonHomogeneousCtorElem(right, content); et != "" {
					elemOf[lname] = et
				} else if et := pythonObjectCollectionElem(right, content, funcReturns, typeOf, fieldOf); et != "" {
					// xs += [ba.get()] / xs += [ba.get()] + [ba.get()] — method-return
					// elements under foreign same-leaf.
					elemOf[lname] = et
				} else if et := pythonListConcatElemType(right, content, elemOf, egElems, typeOf, lname); et != "" {
					elemOf[lname] = et
				} else if et := pythonHomogeneousSplatListCtorElem(right, content, elemOf, egElems, typeOf, lname); et != "" {
					elemOf[lname] = et
				} else if et := pythonIterableElemType(right, content, elemOf, egElems, typeOf); et != "" {
					elemOf[lname] = et
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
				// a := A.make() / a := A.create() — class factory attribute callee.
				// a := ba.get() — instance method annotated return.
				if fn != nil && fn.Type() == "attribute" {
					if rt := pythonCallFuncReturnType(valueN, content, funcReturns, typeOf, fieldOf); rt != "" {
						typeOf[lname] = rt
						if ourReceivers[rt] {
							out[lname] = true
						}
					}
				}
				if fn != nil && fn.Type() == "identifier" {
					fname := ingest.NodeText(fn, content)
					if ourReceivers[fname] {
						// a := A() — Class() ctor of our receiver.
						out[lname] = true
						typeOf[lname] = fname
					}
					// a := make_a() after def make_a() -> A / @lru_cache … /
					// make_a = lambda: A()
					if rt := funcReturns[fname]; rt != "" {
						typeOf[lname] = rt
						if ourReceivers[rt] {
							out[lname] = true
						}
					}
					// a := cast(A, x)
					if fname == "cast" {
						if tn := pythonCastTypeArg(valueN, content); ourReceivers[tn] {
							out[lname] = true
						}
					}
					// a := next(iter(items)) / next(items) / next(x for x in items) /
					// next(reversed(items)) /
					// a := next(iter(astuple(box))) — first declaration-order field
					// of the heterogeneous field tuple (same leaf as astuple(box)[0]).
					// pair := next(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "next" {
						if types := pythonNextPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if types := pythonItemsCallPairSlots(valueN, content, elemOf, fieldOf); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonNextElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						} else if et := pythonAstupleNextFirstField(valueN, content, fieldOf); ourReceivers[et] {
							out[lname] = true
						} else if args, ok := pythonCallPositionalArgNodes(valueN); ok && len(args) > 0 {
							// a := next(iter([ba.get()])) / next(dict(k=ba.get()).values())
							if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					}
					// a := min(items) / max(items) / min(items, key=...) /
					// a := min(asdict(pair).values(), key=...) / max(astuple(pair)) /
					// pair := min(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "min" || fname == "max" {
						if types := pythonMinMaxPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if types := pythonObjectMinMaxPairSlots(valueN, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
							// pair := min(list(zip([ba.get()], …))) — object pair under foreign same-leaf.
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if types := pythonItemsCallPairSlots(valueN, content, elemOf, fieldOf); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonMinMaxElemType(valueN, content, elemOf, egElems, typeOf, fieldOf); ourReceivers[et] {
							out[lname] = true
						} else if args, ok := pythonCallPositionalArgNodes(valueN); ok && len(args) == 1 {
							// a := min([ba.get()], key=...) / max(dict(k=ba.get()).values())
							if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					}
					// a := choice(items) — from random import choice /
					// pair := choice(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "choice" {
						if types := pythonChoicePairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if types := pythonObjectChoicePairSlots(valueN, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
							// pair := choice(list(zip([ba.get()], …))) — object pair under foreign same-leaf.
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonRandomChoiceElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						} else if args, ok := pythonCallPositionalArgNodes(valueN); ok && len(args) > 0 {
							// a := choice([ba.get()])
							if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					}
					// a := heappop(items) / heappushpop(items, x) / heapreplace(items, x) /
					// pair := heappop(pairs) when pairs is a pair-iter (pairSlots + shared elemOf).
					if fname == "heappop" || fname == "heappushpop" || fname == "heapreplace" {
						if types := pythonHeappopPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if types := pythonObjectHeappopPairSlots(valueN, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
							// pair := heappop(list(zip([ba.get()], …))) — object pair under foreign same-leaf.
							pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
						} else if et := pythonHeappopElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						} else if args, ok := pythonCallPositionalArgNodes(valueN); ok && len(args) > 0 {
							// a := heappop([ba.get()])
							if et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					}
					// a := reduce(fn, items) / reduce(fn, items, init)
					if fname == "reduce" {
						if et := pythonReduceElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
							out[lname] = true
						} else if args, ok := pythonCallPositionalArgNodes(valueN); ok && len(args) >= 2 {
							// a := reduce(fn, [ba.get()])
							if et := pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					}
					// a := copy(item) / deepcopy(item) — from copy import copy, deepcopy.
					// a := copy(asdict(box)["a"]) — dict-view field key (fieldOf).
					// a := copy(ba.get()) — method-return under foreign same-leaf.
					if fname == "copy" || fname == "deepcopy" {
						if tn := pythonCopyCallObjectType(valueN, content, typeOf, fieldOf, funcReturns); tn != "" {
							typeOf[lname] = tn
							bindFields(lname, tn)
							if ourReceivers[tn] {
								out[lname] = true
							}
						}
					}
					// xa := ga(box) after ga = attrgetter("a") /
					// a := gi(items) after gi = itemgetter(0).
					// Also a := gi([ba.get()]) / xa := ga(w.get()) under foreign same-leaf.
					if ft := pythonStoredOperatorGetterType(valueN, content, getterOf, fieldOf, elemOf, egElems, typeOf, funcReturns); ft != "" {
						spec := getterOf[fname]
						if !strings.HasSuffix(spec, ":#") {
							typeOf[lname] = ft
							bindFields(lname, ft)
						}
						if ourReceivers[ft] {
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
						case "pop", "popleft", "get", "get_nowait", "setdefault":
							// a := items.pop() / items.pop(0) / d.pop(k)
							// a := items.popleft() (deque)
							// a := d.get(k) / d.get(k, default) / qa.get_nowait()
							// a := box.get("a") / box.pop("a") — TypedDict/record key (fieldOf).
							// a := d.setdefault(k) / d.setdefault(k, default)
							// pair := pairs.pop() when pairs is a pair-iter (pairSlots + shared elemOf).
							obj := ingest.ChildByField(fn, "object")
							if ingest.NodeText(attr, content) == "pop" {
								if types := pythonPopPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
									pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
									break
								}
							}
							if ft := pythonRecordKeyAccessType(valueN, content, fieldOf); ft != "" {
								typeOf[lname] = ft
								bindFields(lname, ft)
								if ourReceivers[ft] {
									out[lname] = true
								}
								break
							}
							// a := asdict(box).get("a") / vars(box).get("a") /
							// box.__dict__.get("a") — dict-view field keys.
							if ft := pythonDictViewKeyAccessType(valueN, content, fieldOf, funcReturns, typeOf); ft != "" {
								typeOf[lname] = ft
								bindFields(lname, ft)
								if ourReceivers[ft] {
									out[lname] = true
								}
								break
							}
							if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							} else if et := pythonHomogeneousObjectDictValue(obj, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								// a := {"k": ba.get()}.get("k") / dict(k=ba.get()).get("k")
								out[lname] = true
							} else if et := pythonMappingProxyObjectValueType(obj, content, elemOf, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								// a := MappingProxyType({"k": ba.get()}).get("k") —
								// proxy of object-dict value under foreign same-leaf.
								out[lname] = true
							} else if et := pythonHomogeneousDictValueCtorElem(obj, content); ourReceivers[et] {
								// a := {"k": A()}.get("k") / dict(k=A()).get("k")
								out[lname] = true
							}
						case "copy", "deepcopy":
							// a := copy.copy(item) / copy.deepcopy(item) — object type of arg.
							// a := copy.copy(asdict(box)["a"]) — dict-view field key (fieldOf).
							// a := copy.copy(ba.get()) — method-return under foreign same-leaf.
							if tn := pythonCopyCallObjectType(valueN, content, typeOf, fieldOf, funcReturns); tn != "" {
								typeOf[lname] = tn
								bindFields(lname, tn)
								if ourReceivers[tn] {
									out[lname] = true
								}
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
						case "__getitem__":
							// a := items.__getitem__(i) / d.__getitem__(k) —
							// same element/value leaf as items[i] / d[k].
							obj := ingest.ChildByField(fn, "object")
							if et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf); ourReceivers[et] {
								out[lname] = true
							} else if et := pythonObjectIterableElemType(obj, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
								out[lname] = true
							}
						}
					}
				}
				// a := itemgetter(0)(items) / operator.itemgetter(0)(items) /
				// pair := itemgetter(0)(pairs) when pairs is a pair-iter.
				// xa := itemgetter("a")(box) / itemgetter("a")(asdict(box)).
				if ft := pythonItemgetterFieldType(valueN, content, fieldOf, funcReturns, typeOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				} else if types := pythonItemgetterPairSlots(valueN, content, elemOf, egElems, typeOf, pairIterSlots); len(types) > 0 {
					pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
				} else if et := pythonItemgetterElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
					out[lname] = true
				} else if et := pythonItemgetterObjectElemType(valueN, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
					// a := itemgetter(0)([ba.get()]) / itemgetter("k")({"k": ba.get()})
					out[lname] = true
				}
				// a := getitem(items, i) / operator.getitem(items, i) —
				// same element/value leaf as items[i] / d[k].
				if et := pythonGetitemElemType(valueN, content, elemOf, egElems, typeOf); ourReceivers[et] {
					out[lname] = true
				} else if et := pythonGetitemObjectElemType(valueN, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
					out[lname] = true
				}
				// xa := attrgetter("a")(box) / attrgetter("a")(replace(box)) /
				// operator.attrgetter("a")(box).
				if ft := pythonAttrgetterFieldType(valueN, content, fieldOf, funcReturns, typeOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				}
				// xa := getattr(box, "a") — builtin field access (same leaf as box.a).
				if ft := pythonGetattrFieldType(valueN, content, fieldOf, funcReturns, typeOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				}
				// new := replace(box) / dataclasses.replace(box) — same object type as
				// first positional arg (fieldOf for new.a.run()). Method-return first
				// arg (replace(ba.get())) peels via funcReturns.
				if tn := pythonReplaceCallObjectType(valueN, content, typeOf, fieldOf, funcReturns); tn != "" {
					typeOf[lname] = tn
					bindFields(lname, tn)
					if ourReceivers[tn] {
						out[lname] = true
					}
				}
				// d := asdict(box) / dataclasses.asdict(box) — dict of field values
				// (fieldOf for d["a"].run()).
				if tn := pythonAsdictCallObjectType(valueN, content, typeOf); tn != "" {
					bindFields(lname, tn)
				}
				// d := vars(box) — same field keys as asdict (obj.__dict__).
				// typeOf path for annotated dataclasses; always also copy SNS fieldOf.
				if tn := pythonVarsCallObjectType(valueN, content, typeOf); tn != "" {
					bindFields(lname, tn)
				}
				// d := vars/asdict/_asdict — instance fieldOf copy (dual-class).
				if src := pythonDictViewObjectLocal(valueN, content); src != "" {
					pythonCopyLocalFieldOf(src, lname, fieldOf)
				}
				if valueN.Type() == "call" {
					// da := SimpleNamespace(k=A()) — same fieldOf bind as plain assignment.
					pythonBindSimpleNamespaceLocal(lname, valueN, content, fieldOf, funcReturns, typeOf)
					pythonBindNamedtupleReplaceLocal(lname, valueN, content, typeOf, fieldOf, fieldNames, fieldIndex, ourReceivers, out)
					pythonBindNamedtupleMakeLocal(lname, valueN, content, typeOf, fieldOf, fieldNames, fieldIndex, ourReceivers, out)
				}
				// t := astuple(box) / dataclasses.astuple(box) — tuple of field
				// values in declaration order (fieldOf["t.#i"] for t[i].run()).
				// Homogeneous field values also record elemOf so for x in t types.
				if tn := pythonAstupleCallObjectType(valueN, content, typeOf); tn != "" {
					pythonBindNamedtupleIndexFields(lname, tn, fieldOrder, fieldIndex, fieldOf)
					if et := pythonAstupleHomogeneousType(valueN, content, fieldOf); et != "" {
						elemOf[lname] = et
					}
				}
			}
			// a := items[0] / a := d[k] — element/value of known collection.
			// pair := pairs[0] / pair := list(zip(...))[0] — pairSlots (+ shared elemOf).
			// a := pairs[0][0] — double subscript slot.
			// xa := box["a"] — TypedDict/record string-key value (fieldOf).
			// Slices fail closed (sequence, not element).
			// xa := asdict(box)["a"] / vars(box)["a"] / box.__dict__["a"] — dict-view keys.
			// xa := astuple(box)[0] — declaration-order index (same leaf as t[0]).
			if valueN.Type() == "subscript" {
				if types := pythonPairSlotsOf(valueN, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots); len(types) > 0 {
					pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
				} else if types := pythonObjectPairSlotsOf(valueN, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
					// pair := list(enumerate([ba.get()]))[0] / list(zip(...))[0]
					pythonBindPairLoopTarget(lname, types, pairSlots, elemOf)
				} else if ft := pythonRecordKeyAccessType(valueN, content, fieldOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				} else if ft := pythonDictViewKeyAccessType(valueN, content, fieldOf, funcReturns, typeOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				} else if ft := pythonAstupleIndexAccessType(valueN, content, fieldOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				} else if ft := pythonNamedtupleIndexType(valueN, content, fieldOf); ft != "" {
					// xa := box[0] — namedtuple positional field (same leaf as box.a).
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				} else if et := pythonSubscriptElemType(valueN, content, elemOf, egElems, typeOf, pairSlots, pairIterSlots, fieldOf); ourReceivers[et] {
					out[lname] = true
				} else if et := pythonObjectPairSlotSubscriptType(valueN, content, funcReturns, typeOf, fieldOf); et != "" {
					// xa := list(enumerate([ba.get()]))[0][1] / list(zip(...))[0][0]
					typeOf[lname] = et
					bindFields(lname, et)
					if ourReceivers[et] {
						out[lname] = true
					}
				} else if et := pythonObjectSubscriptElemType(valueN, content, funcReturns, typeOf, fieldOf); et != "" {
					// xa := sorted([ba.get()])[0] / [ba.get()][0] — method-return collection.
					typeOf[lname] = et
					bindFields(lname, et)
					if ourReceivers[et] {
						out[lname] = true
					}
				}
			}
			// xa := box.a — dataclass/class field access (same as plain assignment).
			// xa := replace(box).a / dataclasses.replace(box).a — field of first arg.
			// d := box.__dict__ — same field keys as vars/asdict (not a field leaf).
			// typeOf path for annotated classes; always also copy SNS fieldOf.
			if valueN.Type() == "attribute" {
				if tn := pythonDunderDictObjectType(valueN, content, typeOf); tn != "" {
					bindFields(lname, tn)
				}
				if src := pythonDictViewObjectLocal(valueN, content); src != "" {
					pythonCopyLocalFieldOf(src, lname, fieldOf)
				}
				if ft := pythonFieldAccessType(valueN, content, fieldOf, funcReturns, typeOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				} else if ft := pythonReplaceFieldAccessType(valueN, content, fieldOf, funcReturns, typeOf); ft != "" {
					typeOf[lname] = ft
					bindFields(lname, ft)
					if ourReceivers[ft] {
						out[lname] = true
					}
				}
			}
			// aa := [A()] / aa := [[A()]] / da := {"k": A()} / da := {"k": [A()]} —
			// collection peels mirror plain assignment so if (aa := [A()]): aa[0].run()
			// / if (da := {"k": A()}): da["k"].run() bind under foreign same-leaf.
			// Scalar Class() / next / etc. already handled above (out/typeOf).
			if et := pythonHomogeneousCtorElem(valueN, content); et != "" {
				elemOf[lname] = et
			} else if et := pythonObjectCollectionElem(valueN, content, funcReturns, typeOf, fieldOf); et != "" {
				// xs := [ba.get()] / xs := [ba.get()] + … / xs := list([ba.get()]) —
				// method-return collection peels (dict after nested peels below).
				elemOf[lname] = et
			} else if et := pythonListConcatElemType(valueN, content, elemOf, egElems, typeOf, lname); et != "" {
				elemOf[lname] = et
			} else if et := pythonListRepeatElemType(valueN, content, elemOf, egElems, typeOf); et != "" {
				elemOf[lname] = et
			} else if nest := pythonNestedHomogeneousCtorElem(valueN, content); nest != "" {
				elemOf["@nested."+lname] = nest
			} else if nest := pythonNestedDictHomogeneousListCtorElem(valueN, content); nest != "" {
				elemOf["@nested."+lname] = nest
			} else if et := pythonHomogeneousObjectDictValue(valueN, content, funcReturns, typeOf, fieldOf); et != "" {
				// da := {"k": ba.get()} — after nested peels.
				elemOf[lname] = et
			} else if et := pythonHomogeneousDictValueCtorElem(valueN, content); et != "" {
				elemOf[lname] = et
			} else if nest := pythonNestedMappingSubscriptElemType(valueN, content, elemOf); nest != "" {
				// row := aa[0] / row := [[A()]][0] when nested — row is list of nest leaf.
				elemOf[lname] = nest
			} else if et := pythonIterableElemType(valueN, content, elemOf, egElems, typeOf); et != "" {
				elemOf[lname] = et
			} else if et := pythonObjectIterableElemType(valueN, content, funcReturns, typeOf, fieldOf); et != "" {
				// it := repeat(ba.get()) / it := cycle([ba.get()]) — method-return
				// object iterables under foreign same-leaf (Class peels above).
				elemOf[lname] = et
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
				// for item in enumerate([ba.get()]) / for pair in zip([ba.get()], …) —
				// object pair-iter under foreign same-leaf (Class peels above).
				if types := pythonObjectEnumerateZipTargetTypes(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
					pythonBindPairLoopTarget(ingest.NodeText(left, content), types, pairSlots, elemOf)
					break
				}
				// for item in d.items() / asdict(...).items() / list(d.items()) —
				// pair local (key untyped, value leaf); use item[1] / k, a = item.
				// Same slots as p = next(...items()). Mixed asdict fields fail closed.
				if types := pythonItemsViewPairSlots(right, content, elemOf, fieldOf); len(types) > 0 {
					pythonBindPairLoopTarget(ingest.NodeText(left, content), types, pairSlots, elemOf)
					break
				}
				// for ga in da.values() when da: defaultdict[str, list[A]] —
				// values are list of A (not A); bind elemOf so ga[0].run() peels.
				if et := pythonNestedMappingValuesElemType(right, content, elemOf); et != "" {
					elemOf[ingest.NodeText(left, content)] = et
					break
				}
				// for row in aa when aa: list[list[A]] — rows are list of A (not A);
				// bind elemOf so for a in row / row[0].run() peels under foreign same-leaf.
				if et := pythonNestedCollectionIdentElemType(right, content, elemOf); et != "" {
					elemOf[ingest.NodeText(left, content)] = et
					break
				}
				if et := pythonIterableElemType(right, content, elemOf, egElems, typeOf); ourReceivers[et] {
					out[ingest.NodeText(left, content)] = true
				} else if et := pythonObjectIterableElemType(right, content, funcReturns, typeOf, fieldOf); ourReceivers[et] {
					// for x in [ba.get()] / for x in (ba.get(),) /
					// for x in list([ba.get()]) / for x in [ba.get()] + [ba.get()] /
					// for x in dict(k=ba.get()).values() /
					// for x in {"k": ba.get()}.values() /
					// for x in ChainMap({"k": ba.get()}).values() —
					// method-return elements / object-dict values under foreign same-leaf.
					out[ingest.NodeText(left, content)] = true
				} else if et := pythonDictViewValuesHomogeneousType(right, content, fieldOf); ourReceivers[et] {
					// for x in asdict(box).values() / vars / __dict__ / d.values()
					// after d = asdict(box) — only when all field types agree.
					out[ingest.NodeText(left, content)] = true
				} else if et := pythonAstupleHomogeneousType(right, content, fieldOf); ourReceivers[et] {
					// for x in astuple(box) / list(astuple(box)) / dataclasses.astuple —
					// only when all declaration-order field types agree.
					out[ingest.NodeText(left, content)] = true
				}
			case "pattern_list", "tuple_pattern":
				targets := pythonPatternIdents(left, content)
				if len(targets) == 0 {
					break
				}
				// for k, ga in da.items() when da: dict[str, list[A]] — value is list of A
				// (not A); bind elemOf so ga[0].run() peels (same as values() nested).
				if et := pythonNestedMappingItemsElemType(right, content, elemOf); et != "" {
					if len(targets) >= 2 {
						elemOf[targets[1]] = et
					}
					break
				}
				// for k, v in d.items() — value type is elemOf[d] (dict value leaf).
				if vt := pythonDictItemsValueType(right, content, elemOf); vt != "" {
					if len(targets) >= 2 && ourReceivers[vt] {
						out[targets[1]] = true
					}
					break
				}
				// for k, x in asdict(box).items() / vars / __dict__ / d.items() after
				// d = asdict(box) — only when all field types agree (homogeneous values).
				if vt := pythonDictViewItemsHomogeneousValueType(right, content, fieldOf); vt != "" {
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
				// for i, a in enumerate([ba.get()]) / for a, _ in zip([ba.get()], …) —
				// object pair-iter under foreign same-leaf.
				if types := pythonObjectEnumerateZipTargetTypes(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
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
		case "call":
			// sorted/min/max/groupby/nlargest/nsmallest/merge/bisect*(..., key=lambda x: x.m()) /
			// nlargest/nsmallest(n, items, lambda x: x.m()) (positional key) /
			// items.sort(key=lambda x: x.m()) /
			// map/filter/takewhile/dropwhile/filterfalse(lambda x: x.m(), items) —
			// untyped unary lambda params from the iterable element type.
			// Without this, x.m() is skipped when a foreign same-leaf method exists.
			pythonBindIterableLambdaParams(n, content, ourReceivers, elemOf, egElems, typeOf, out)
			// fa.set_result(A()) / fb.set_result(B()) — bind Future local → result
			// class leaf so fa.result().run() peels under foreign same-leaf.
			// Foreign results too for shadowing.
			if fut, et := pythonFutureSetResultType(n, content); fut != "" && et != "" {
				futureOf[fut] = et
			}
			// xs.append(A()) / xs.append(ba.get()) / xs.extend([A()]) /
			// xs.insert(0, A()) / xs.add(A()) / deque.extendleft([A()]) —
			// bare list/deque/set mutation (Class() + method-return).
			// da["k"].append/extend/insert — mapping-of-list bucket (defaultdict(list)).
			// Bind elemOf / @nested so xs[0].run() / next(iter(xs)).run() /
			// da["k"][0].run() peel under foreign same-leaf. Foreign too for shadowing.
			if local, et, nest := pythonCollectionMutationElemType(n, content, funcReturns, typeOf, fieldOf); local != "" && et != "" {
				if nest {
					elemOf["@nested."+local] = et
				} else {
					elemOf[local] = et
				}
			}
			// da.setdefault("k", A()) / da.setdefault("k", ba.get()) /
			// da.update({"k": A()}) / da.update(k=A()) — unannotated mapping value
			// mutation. Bind elemOf so da["k"].run() / setdefault return peels under
			// foreign same-leaf. Foreign too for shadowing.
			if local, et := pythonDictMutationValueType(n, content, funcReturns, typeOf, fieldOf); local != "" && et != "" {
				elemOf[local] = et
			}
		case "as_pattern":
			// match `case A() as a`, with `with A() as a`, except `except A as e`.
			// except* is handled above (e is ExceptionGroup, not A).
			// Without this, a.m() is skipped when a foreign same-leaf method exists.
			if name, typ := pythonAsPatternBinding(n, content); name != "" && ourReceivers[typ] {
				out[name] = true
			}
			// with make_a() as a after @contextmanager def make_a(): yield A() —
			// alias is the yielded instance, not the CM object.
			if name, typ := pythonAsPatternCMYieldBinding(n, content, cmYieldOf); name != "" {
				typeOf[name] = typ
				if ourReceivers[typ] {
					out[name] = true
				}
			}
			// with nullcontext(a) as xa / closing(A()) as xa — identity CM peels.
			// with nullcontext(ba.get()) as xa — method-return under foreign same-leaf.
			if name, typ := pythonAsPatternIdentityCMBinding(n, content, typeOf, funcReturns, fieldOf); name != "" {
				typeOf[name] = typ
				bindFields(name, typ)
				if ourReceivers[typ] {
					out[name] = true
				}
			}
		case "class_pattern":
			// match case Box(a=xa, b=xb) / Box(xa, xb): keyword and positional
			// value captures get field types of Box (dataclass / annotated class
			// via fieldIndex; positionals use fieldOrder / namedtuple order).
			pythonBindClassPatternKeywordCaptures(n, content, fieldIndex, fieldOrder, ourReceivers, out)
		case "match_statement":
			// match items: case [a]: / case [a, *rest]: — bind sequence captures
			// from the subject's element type (items: list[A] / xs = [A()] / …).
			// match d: case {"k": a}: — bind mapping value captures from the
			// subject's dict value leaf (d: dict[K, A] / …).
			// match aa: case [[xa, *_], *_]: / match da: case {"k": [xa, *_]}: —
			// nested list/mapping patterns when subject has @nested leaf T
			// (list[list[A]] / dict[str, list[A]]) under foreign same-leaf.
			// match box: case {"a": xa}: — TypedDict/record key-specific value
			// captures via fieldOf (box: Box with a: A; not homogeneous elemOf).
			// match a: case x as xa: / case _ as xa: — bind captures from subject
			// typeOf (a: A typed local / a = A()) under foreign same-leaf.
			// Without this, a.m() is skipped under foreign same-leaf; *rest loops
			// also stay untyped. as_pattern cases still handled above when walked.
			subject := ingest.ChildByField(n, "subject")
			if subject != nil {
				et := pythonIterableElemType(subject, content, elemOf, egElems, typeOf)
				subjLocal := ""
				subjType := ""
				nest := ""
				if subject.Type() == "identifier" {
					subjLocal = ingest.NodeText(subject, content)
					if typeOf != nil {
						subjType = typeOf[subjLocal]
					}
					if elemOf != nil {
						// list[list[A]] / dict[str, list[A]] nested leaf.
						nest = elemOf["@nested."+subjLocal]
					}
				}
				// Homogeneous dict/list path and TypedDict key path are independent.
				if et != "" || nest != "" || subjLocal != "" || subjType != "" {
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
								if et != "" || nest != "" {
									pythonBindMatchSeqPatterns(p, content, et, nest, ourReceivers, out, elemOf)
								}
								if subjLocal != "" {
									pythonBindMatchRecordKeyPatterns(p, content, subjLocal, fieldOf, ourReceivers, out)
									// case SimpleNamespace(k=xa) / case Box(a=xa) from
									// instance fieldOf[subj.field] (SNS / dual-class
									// namedtuple). Type-level fieldIndex alone under-renames
									// dual-class same-field (ba=Box(A()); bb=Box(B())).
									pythonBindClassPatternSubjectFields(p, content, subjLocal, fieldOf, ourReceivers, out)
								}
								if subjType != "" {
									pythonBindMatchSubjectTypeCaptures(p, content, subjType, ourReceivers, out)
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
	walk(root)
	return out, fieldOf, elemOf, typeOf, pairSlots, factoryOf, futureOf, getterOf, funcReturns
}

// pythonBindIterableLambdaParams types untyped lambda parameters when the call
// is sorted/min/max/groupby/nlargest/nsmallest/merge/bisect* with key=lambda or
// key=cmp_to_key(lambda a, b: ...) (keyword or nlargest/nsmallest positional
// 3rd-arg lambda), collection.sort(key=...), map/filter/takewhile/dropwhile/
// filterfalse with a unary lambda, or reduce/accumulate with a bi-lambda, over
// a known iterable element type of ourReceivers. Bare and module-qualified
// forms (itertools./heapq./functools./bisect.) use the leaf callee name.
// Wrong-arity lambdas and non-lambda callables fail closed.
// Foreign element types are not bound (same as for-loop targets).
func pythonBindIterableLambdaParams(call *grammar.Node, content []byte, ourReceivers map[string]bool, elemOf, egElems, typeOf map[string]string, out map[string]bool) {
	if call == nil || call.Type() != "call" || out == nil {
		return
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return
	}
	// items.sort(key=lambda x: ...) / items.sort(key=cmp_to_key(lambda a, b: ...))
	// — element type of the receiver collection. Method form only (not a free
	// function); other attributes fall through to leaf-name matching
	// (itertools.takewhile / heapq.nlargest / heapq.merge / bisect.bisect_left).
	if fn.Type() == "attribute" {
		if attr := ingest.ChildByField(fn, "attribute"); attr != nil && ingest.NodeText(attr, content) == "sort" {
			obj := ingest.ChildByField(fn, "object")
			et := pythonIterableElemType(obj, content, elemOf, egElems, typeOf)
			if !ourReceivers[et] {
				return
			}
			pythonBindKeyArgParams(call, content, out)
			return
		}
	}
	switch pythonSimpleCalleeName(fn, content) {
	case "sorted", "min", "max", "groupby":
		// sorted/min/max/groupby(iterable, key=lambda x: ...) /
		// key=cmp_to_key(lambda a, b: ...) — 1st positional is iterable; key
		// lambda param(s) are that element type (kwargs like reverse= ignored).
		// itertools.groupby same leaf via pythonSimpleCalleeName.
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) == 0 {
			return
		}
		et := pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
		if !ourReceivers[et] {
			return
		}
		pythonBindKeyArgParams(call, content, out)
	case "merge":
		// merge(*iterables, key=lambda x: ..., reverse=...) / heapq.merge(...) —
		// shared element type across positional iterables (same as for-loop
		// targets via pythonChainElemType); key= lambda param is that element
		// type. reverse ignored. Non-lambda key callables fail closed.
		et := pythonChainElemType(call, content, elemOf, egElems, typeOf)
		if !ourReceivers[et] {
			return
		}
		pythonBindKeyArgParams(call, content, out)
	case "bisect_left", "bisect_right", "bisect", "insort_left", "insort_right", "insort":
		// bisect_left(a, x, *, key=lambda e: ...) / bisect.bisect_left(...) /
		// insort_* — 1st positional is the sorted list; key= lambda param is
		// that element type (needle/lo/hi ignored). Non-lambda key fails closed.
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) == 0 {
			return
		}
		et := pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
		if !ourReceivers[et] {
			return
		}
		pythonBindKeyArgParams(call, content, out)
	case "nlargest", "nsmallest":
		// nlargest(n, iterable[, key]) / heapq.nlargest(...) — 2nd positional is
		// iterable; key lambda (key= keyword, key=cmp_to_key(...), or 3rd
		// positional) param(s) are that element type (n ignored). Non-lambda
		// key callables fail closed.
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) < 2 {
			return
		}
		et := pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
		if !ourReceivers[et] {
			return
		}
		if kw := pythonKeywordArgValue(call, content, "key"); kw != nil {
			pythonBindKeyValueParams(kw, content, out)
		} else if len(args) >= 3 && args[2].Type() == "lambda" {
			pythonBindUnaryLambdaParam(args[2], content, out)
		}
	case "map", "filter", "takewhile", "dropwhile", "filterfalse":
		// map/filter/takewhile/dropwhile/filterfalse(lambda x: ..., iterable) —
		// unary lambda param is the 2nd-arg element type. Class-as-callable map
		// and filter(None, ...) have no lambda to bind. itertools.* same leaf.
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) < 2 || args[0].Type() != "lambda" {
			return
		}
		et := pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
		if !ourReceivers[et] {
			return
		}
		pythonBindUnaryLambdaParam(args[0], content, out)
	case "reduce":
		// reduce(lambda a, b: ..., iterable[, init]) / functools.reduce(...) —
		// both bi-lambda params are the iterable element type (same-leaf fold;
		// mirrors assignment result typing via pythonReduceElemType). Non-lambda
		// callables fail closed.
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) < 2 || args[0].Type() != "lambda" {
			return
		}
		et := pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
		if !ourReceivers[et] {
			return
		}
		pythonBindBiLambdaParams(args[0], content, out)
	case "accumulate":
		// accumulate(iterable, lambda a, b: ...) / itertools.accumulate(...) —
		// both bi-lambda params are the 1st-arg element type. func= keyword
		// form also accepted; initial= ignored (same-leaf product case).
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) == 0 {
			return
		}
		et := pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
		if !ourReceivers[et] {
			return
		}
		var lam *grammar.Node
		if len(args) >= 2 && args[1].Type() == "lambda" {
			lam = args[1]
		} else if kw := pythonKeywordArgValue(call, content, "func"); kw != nil && kw.Type() == "lambda" {
			lam = kw
		}
		if lam != nil {
			pythonBindBiLambdaParams(lam, content, out)
		}
	}
}

// pythonBindUnaryLambdaParam records the sole untyped lambda parameter as a
// typed local of ourReceivers. Multi-param / defaulted / typed forms fail closed.
func pythonBindUnaryLambdaParam(lam *grammar.Node, content []byte, out map[string]bool) {
	names := pythonLambdaParamNames(lam, content)
	if len(names) != 1 {
		return
	}
	out[names[0]] = true
}

// pythonBindBiLambdaParams records both untyped bi-lambda parameters as typed
// locals of ourReceivers (same-leaf fold: reduce/accumulate). Wrong arity /
// defaulted / typed forms fail closed.
func pythonBindBiLambdaParams(lam *grammar.Node, content []byte, out map[string]bool) {
	names := pythonLambdaParamNames(lam, content)
	if len(names) != 2 {
		return
	}
	out[names[0]] = true
	out[names[1]] = true
}

// pythonBindKeyArgParams types key= on a call: bare key=lambda (unary) or
// key=cmp_to_key(lambda a, b: ...) / functools.cmp_to_key(...) (bi). Other key
// callables fail closed.
func pythonBindKeyArgParams(call *grammar.Node, content []byte, out map[string]bool) {
	pythonBindKeyValueParams(pythonKeywordArgValue(call, content, "key"), content, out)
}

// pythonBindKeyValueParams types a key= value node (see pythonBindKeyArgParams).
func pythonBindKeyValueParams(val *grammar.Node, content []byte, out map[string]bool) {
	if val == nil || out == nil {
		return
	}
	switch val.Type() {
	case "lambda":
		pythonBindUnaryLambdaParam(val, content, out)
	case "call":
		// cmp_to_key(mycmp) / functools.cmp_to_key(mycmp) — peel bi-lambda.
		fn := ingest.ChildByField(val, "function")
		if pythonSimpleCalleeName(fn, content) != "cmp_to_key" {
			return
		}
		args, ok := pythonCallPositionalArgNodes(val)
		if !ok || len(args) == 0 || args[0].Type() != "lambda" {
			return
		}
		pythonBindBiLambdaParams(args[0], content, out)
	}
}

// pythonLambdaParamNames returns bare identifier parameters of a lambda.
// Typed / defaulted / starred params fail closed (nil).
func pythonLambdaParamNames(lam *grammar.Node, content []byte) []string {
	if lam == nil || lam.Type() != "lambda" {
		return nil
	}
	params := ingest.ChildByField(lam, "parameters")
	if params == nil {
		return nil
	}
	var names []string
	for i := uint32(0); i < params.ChildCount(); i++ {
		ch := params.Child(i)
		switch ch.Type() {
		case ",", "comment":
			continue
		case "identifier":
			names = append(names, ingest.NodeText(ch, content))
		default:
			// default_parameter / typed_parameter / list_splat / dictionary_splat —
			// fail closed (unknown binding shape).
			return nil
		}
	}
	return names
}

// pythonKeywordArgValue returns the value node of keyword_argument name= in a call.
func pythonKeywordArgValue(call *grammar.Node, content []byte, key string) *grammar.Node {
	if call == nil || call.Type() != "call" {
		return nil
	}
	args := ingest.ChildByField(call, "arguments")
	if args == nil {
		return nil
	}
	for i := uint32(0); i < args.ChildCount(); i++ {
		ch := args.Child(i)
		if ch.Type() != "keyword_argument" {
			continue
		}
		nameN := ingest.ChildByField(ch, "name")
		if nameN != nil && ingest.NodeText(nameN, content) == key {
			return ingest.ChildByField(ch, "value")
		}
	}
	return nil
}

// pythonPropertyReturnType recovers (name, type) from a decorated_definition
// that is a @property method with a concrete return annotation:
//
//	@property
//	def item(self) -> A: ...
//
// Other decorators / missing return type / non-function body fail closed.
func pythonPropertyReturnType(decorated *grammar.Node, content []byte) (string, string) {
	if decorated == nil || decorated.Type() != "decorated_definition" {
		return "", ""
	}
	isProp := false
	var fn *grammar.Node
	for i := uint32(0); i < decorated.ChildCount(); i++ {
		ch := decorated.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "decorator":
			// @property / @property()
			for j := uint32(0); j < ch.ChildCount(); j++ {
				c := ch.Child(j)
				if c == nil {
					continue
				}
				if c.Type() == "identifier" && ingest.NodeText(c, content) == "property" {
					isProp = true
				}
				if c.Type() == "call" {
					if f := ingest.ChildByField(c, "function"); f != nil && f.Type() == "identifier" && ingest.NodeText(f, content) == "property" {
						isProp = true
					}
				}
			}
		case "function_definition", "async_function_definition":
			fn = ch
		}
	}
	if !isProp || fn == nil {
		return "", ""
	}
	nameN := ingest.ChildByField(fn, "name")
	retN := ingest.ChildByField(fn, "return_type")
	if nameN == nil || retN == nil {
		return "", ""
	}
	name := ingest.NodeText(nameN, content)
	if name == "" || name == "_" {
		return "", ""
	}
	tn := pythonTypeName(retN, content)
	if tn == "" {
		return "", ""
	}
	return name, tn
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
					if ch.Type() == "assignment" {
						left := ingest.ChildByField(ch, "left")
						typeN := ingest.ChildByField(ch, "type")
						if left == nil || typeN == nil || left.Type() != "identifier" {
							continue
						}
						if tn := pythonTypeName(typeN, content); tn != "" {
							fields[ingest.NodeText(left, content)] = tn
						}
						continue
					}
					// @property def item(self) -> A — property return type leaf so
					// ba.item.run() peels under foreign same-leaf (same path as ba.a).
					if ch.Type() == "decorated_definition" {
						if pname, ptyp := pythonPropertyReturnType(ch, content); pname != "" && ptyp != "" {
							fields[pname] = ptyp
						}
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

// pythonClassFieldOrder maps class type name → annotated field names in
// declaration order (Box with a: A; b: B → ["a","b"]). Used for positional
// match class patterns (`case Box(xa, xb):` → xa is a, xb is b).
func pythonClassFieldOrder(root *grammar.Node, content []byte) map[string][]string {
	out := map[string][]string{}
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
				var names []string
				for i := uint32(0); i < body.ChildCount(); i++ {
					ch := body.Child(i)
					if ch.Type() != "assignment" {
						continue
					}
					left := ingest.ChildByField(ch, "left")
					typeN := ingest.ChildByField(ch, "type")
					if left == nil || typeN == nil || left.Type() != "identifier" {
						continue
					}
					if tn := pythonTypeName(typeN, content); tn != "" {
						names = append(names, ingest.NodeText(left, content))
					}
				}
				if len(names) > 0 {
					out[typeName] = names
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
// Only known namedtuple/annotated types are indexed — dict(k=A()) kwargs are not.
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
// call for a *known* type only: namedtuple factory field names and/or prior
// annotated class fields. Keyword Class() args (Box(a=A(), b=B())) and
// positional Class() args (Box(A(), B()) — order-sensitive via fieldNames)
// refine those fields.
//
// Unknown callees (dict(k=A()) / OrderedDict(k=A()) / OrderedDict(k=frozenset([A()])))
// must not invent fieldIndex["dict"] / fieldIndex["OrderedDict"]: last dual-class
// write would bindFields da.k → B and under-rename A.run (B shadows A).
func pythonIndexNamedtupleCtorFields(call *grammar.Node, typeName string, content []byte, fieldNames []string, fieldIndex map[string]map[string]string) {
	if call == nil || typeName == "" || fieldIndex == nil {
		return
	}
	// Known type only — namedtuple factory list and/or annotated class fields.
	if len(fieldNames) == 0 && len(fieldIndex[typeName]) == 0 {
		return
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return
	}
	// Keyword: Box(a=A(), b=B()) — field names may come from factory or annotations.
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

// pythonMergeFunctionalNamedTupleFields indexes field types from functional
// typing.NamedTuple factories (types live in the call, not a class body):
//
//	Box = NamedTuple("Box", [("a", A), ("b", B)])
//	Box = NamedTuple("Box", a=A, b=B)
//	Box = typing.NamedTuple(...)
//
// Enables box.a.run() / xa = box.a under foreign same-leaf methods.
func pythonMergeFunctionalNamedTupleFields(root *grammar.Node, content []byte, fieldIndex map[string]map[string]string) {
	if root == nil || fieldIndex == nil {
		return
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
				if pythonIsTypingNamedTupleCall(right, content) {
					typeName := ingest.NodeText(left, content)
					if fields := pythonParseFunctionalNamedTupleFields(right, content); len(fields) > 0 {
						if fieldIndex[typeName] == nil {
							fieldIndex[typeName] = map[string]string{}
						}
						for f, t := range fields {
							fieldIndex[typeName][f] = t
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
}

// pythonIsTypingNamedTupleCall reports NamedTuple(...) / typing.NamedTuple(...).
func pythonIsTypingNamedTupleCall(call *grammar.Node, content []byte) bool {
	if call == nil || call.Type() != "call" {
		return false
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return false
	}
	if fn.Type() == "identifier" {
		return ingest.NodeText(fn, content) == "NamedTuple"
	}
	if fn.Type() == "attribute" {
		obj := ingest.ChildByField(fn, "object")
		attr := ingest.ChildByField(fn, "attribute")
		return obj != nil && attr != nil &&
			obj.Type() == "identifier" &&
			ingest.NodeText(obj, content) == "typing" &&
			ingest.NodeText(attr, content) == "NamedTuple"
	}
	return false
}

// pythonParseFunctionalNamedTupleFields returns field→type from a functional
// NamedTuple call: keyword types (a=A) and/or list/tuple of (name, type) pairs.
// Type leaves are bare identifiers only; other forms fail closed.
func pythonParseFunctionalNamedTupleFields(call *grammar.Node, content []byte) map[string]string {
	out := map[string]string{}
	if call == nil {
		return out
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return out
	}
	// Keyword form: NamedTuple("Box", a=A, b=B)
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch.Type() != "keyword_argument" {
			continue
		}
		nameN := ingest.ChildByField(ch, "name")
		valN := ingest.ChildByField(ch, "value")
		if nameN == nil || valN == nil || valN.Type() != "identifier" {
			continue
		}
		out[ingest.NodeText(nameN, content)] = ingest.NodeText(valN, content)
	}
	// List/tuple form: NamedTuple("Box", [("a", A), ("b", B)])
	// Second positional arg is the fields sequence (first is typename string).
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 2 {
		return out
	}
	seq := args[1]
	if seq.Type() != "list" && seq.Type() != "tuple" {
		return out
	}
	for i := uint32(0); i < seq.ChildCount(); i++ {
		pair := seq.Child(i)
		if pair.Type() != "tuple" {
			continue
		}
		var parts []*grammar.Node
		for j := uint32(0); j < pair.ChildCount(); j++ {
			ch := pair.Child(j)
			if ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," || ch.Type() == "comment" {
				continue
			}
			parts = append(parts, ch)
		}
		if len(parts) != 2 {
			continue
		}
		fname := ""
		if parts[0].Type() == "string" {
			_, fname = pythonStringContent(parts[0], content)
		}
		if !pythonIsIdentifier(fname) || parts[1].Type() != "identifier" {
			continue
		}
		out[fname] = ingest.NodeText(parts[1], content)
	}
	return out
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

// pythonBindNamedtupleIndexFields records fieldOf["local.#i"] → T for factory
// namedtuple fields in declaration order (enables box[0].run() / xa = box[0]).
// Only when field names are known from namedtuple(...) and field types from
// same-file constructors; missing types fail closed per index.
func pythonBindNamedtupleIndexFields(local, typeName string, fieldNames map[string][]string, index map[string]map[string]string, fieldOf map[string]string) {
	if local == "" || typeName == "" || fieldNames == nil || index == nil || fieldOf == nil {
		return
	}
	names := fieldNames[typeName]
	fields := index[typeName]
	if len(names) == 0 || fields == nil {
		return
	}
	for i, fname := range names {
		if t := fields[fname]; t != "" {
			fieldOf[local+".#"+fmt.Sprintf("%d", i)] = t
		}
	}
}

// pythonNamedtupleIndexType recovers T from box[0] when box is a typed
// namedtuple local with ordered field types (fieldOf["box.#0"]). Integer
// literal indices only; slices / non-decimal / OOB fail closed.
func pythonNamedtupleIndexType(n *grammar.Node, content []byte, fieldOf map[string]string) string {
	if n == nil || n.Type() != "subscript" || fieldOf == nil {
		return ""
	}
	val := ingest.ChildByField(n, "value")
	if val == nil {
		val = ingest.ChildByField(n, "object")
	}
	sub := ingest.ChildByField(n, "subscript")
	if val == nil || val.Type() != "identifier" || sub == nil || sub.Type() != "integer" {
		return ""
	}
	idxText := ingest.NodeText(sub, content)
	if idxText == "" {
		return ""
	}
	for _, c := range idxText {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return fieldOf[ingest.NodeText(val, content)+".#"+idxText]
}

// pythonFieldAccessType recovers T from box.a when box is a typed local with
// annotated field a of type T. Also BoxA(A()).a (ctor) and ba.self().item
// (method-return peel then property/field) under foreign same-leaf.
// funcReturns/typeOf may be nil (identifier/ctor paths only).
func pythonFieldAccessType(attr *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string) string {
	if attr == nil || attr.Type() != "attribute" || fieldOf == nil {
		return ""
	}
	obj := ingest.ChildByField(attr, "object")
	field := ingest.ChildByField(attr, "attribute")
	if obj == nil || field == nil {
		return ""
	}
	fname := ingest.NodeText(field, content)
	if fname == "" {
		return ""
	}
	// ba.a / ba.item — typed local with field/property leaf.
	if obj.Type() == "identifier" {
		return fieldOf[ingest.NodeText(obj, content)+"."+fname]
	}
	// Nested field: oa.h.h — peel outer attribute type then type-level field leaf
	// (fieldOf["HolderA.h"]=BoxA). Enables oa.h.h.get().run() under foreign
	// same-leaf when assigned ha = oa.h; ha.h.get() already peels.
	if obj.Type() == "attribute" {
		if bt := pythonFieldAccessType(obj, content, fieldOf, funcReturns, typeOf); bt != "" {
			return fieldOf[bt+"."+fname]
		}
	}
	// BoxA(A()).a / BoxA(A()).item — constructor peel via type-level fieldOf
	// entries ("BoxA.a") under foreign same-leaf methods.
	// ba.self().item — method-return peel (funcReturns["BoxA.self"]=BoxA).
	if obj.Type() == "call" {
		if tn := pythonExprClassType(obj, content); tn != "" {
			return fieldOf[tn+"."+fname]
		}
		if tn := pythonCallFuncReturnType(obj, content, funcReturns, typeOf, fieldOf); tn != "" {
			return fieldOf[tn+"."+fname]
		}
	}
	return ""
}

// pythonAttrgetterFieldType recovers T from attrgetter("a")(box) /
// operator.attrgetter("a")(box) / attrgetter("a")(replace(box)) when box is a
// typed local with annotated field a of type T (fieldOf; same leaf as box.a /
// replace(box).a). Also attrgetter("a")(w.get()) / attrgetter("a")(Box(A())) via
// type-level fieldOf["Box.a"] under foreign same-leaf. Single string field only —
// multi-field attrgetter("a","b") returns a tuple and fails closed. Stored
// getters (g = attrgetter("a"); g(box)) use pythonStoredOperatorGetterType.
func pythonAttrgetterFieldType(call *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string) string {
	return pythonOperatorGetterFieldType(call, content, fieldOf, funcReturns, typeOf, "attrgetter")
}

// pythonGetattrFieldType recovers T from getattr(box, "a") when box is a typed
// local with annotated field a of type T (fieldOf; same leaf as box.a /
// attrgetter("a")(box)). Also getattr(w.get(), "a") / getattr(Box(A()), "a") via
// type-level fieldOf under foreign same-leaf, and getattr(SimpleNamespace(k=A()), "k")
// / types.SimpleNamespace(k=A()) — inline SNS kwargs (same leaf as
// vars(SimpleNamespace(...))["k"] / SimpleNamespace(...).k). Exactly two
// positional args: object + string field name. Three-arg getattr(obj, name,
// default), non-string attr names, and other objects fail closed. Bare builtin
// name only — getattr from other modules / getattr stored in a variable are
// not tracked.
func pythonGetattrFieldType(call *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" || ingest.NodeText(fn, content) != "getattr" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 2 {
		return ""
	}
	if args[1].Type() != "string" {
		return ""
	}
	_, field := pythonStringContent(args[1], content)
	if field == "" {
		return ""
	}
	if fieldOf == nil {
		return ""
	}
	if args[0].Type() == "identifier" {
		return fieldOf[ingest.NodeText(args[0], content)+"."+field]
	}
	// getattr(w.get(), "a") / getattr(Box(A()), "a") — method-return / Class()
	// object peels via type-level fieldOf (same leaf as w.get().a / Box(A()).a).
	if args[0].Type() == "call" {
		if tn := pythonCallFuncReturnType(args[0], content, funcReturns, typeOf, fieldOf); tn != "" {
			if t := fieldOf[tn+"."+field]; t != "" {
				return t
			}
		}
		if tn := pythonExprClassType(args[0], content); tn != "" {
			if t := fieldOf[tn+"."+field]; t != "" {
				return t
			}
		}
		// getattr(SimpleNamespace(k=A()), "k") / types.SimpleNamespace — inline SNS.
		fields, _ := pythonSimpleNamespaceFieldTypes(args[0], content)
		if fields != nil {
			return fields[field]
		}
	}
	return ""
}

// pythonItemgetterFieldType recovers T from itemgetter("a")(box) /
// operator.itemgetter("a")(box) / itemgetter("a")(asdict(box)) when box is a
// typed local with annotated field a of type T (fieldOf; same leaf as box["a"] /
// asdict(box)["a"]). Also itemgetter("a")(w.get()) via type-level fieldOf under
// foreign same-leaf. Single string key only — multi-key itemgetter("a","b")
// returns a tuple and fails closed. Numeric itemgetter(i)(collection) uses
// pythonItemgetterElemType instead. Stored getters (g = itemgetter("a"); g(box))
// use pythonStoredOperatorGetterType via getterOf.
func pythonItemgetterFieldType(call *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string) string {
	return pythonOperatorGetterFieldType(call, content, fieldOf, funcReturns, typeOf, "itemgetter")
}

// pythonOperatorGetterFieldType recovers T from name("field")(box) /
// operator.name("field")(box) for name in {attrgetter, itemgetter} via fieldOf.
// Object peels (same field leaf as the bare local):
//
//	attrgetter — box / replace(box) / dataclasses.replace(box) /
//	  w.get() / Box(A()) method-return / Class() via type-level fieldOf /
//	  SimpleNamespace(k=A()) / types.SimpleNamespace(k=A()) inline kwargs
//	  (same leaf as getattr(SimpleNamespace(...), "k"))
//	itemgetter — box / asdict(box) / vars(box) / box.__dict__ /
//	  w.get() / Box(A()) method-return / Class() via type-level fieldOf
func pythonOperatorGetterFieldType(call *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string, name string) string {
	if call == nil || call.Type() != "call" || name == "" {
		return ""
	}
	// Outer call: getter(obj) — function must itself be name(...).
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
		if ingest.NodeText(innerFn, content) != name {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(innerFn, "attribute")
		obj := ingest.ChildByField(innerFn, "object")
		if attr == nil || ingest.NodeText(attr, content) != name {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "operator" {
			return ""
		}
	default:
		return ""
	}
	// Getter must have exactly one positional string arg (the field/key name).
	fieldArgs, ok := pythonCallPositionalArgNodes(fn)
	if !ok || len(fieldArgs) != 1 || fieldArgs[0].Type() != "string" {
		return ""
	}
	_, field := pythonStringContent(fieldArgs[0], content)
	if field == "" {
		return ""
	}
	// Outer call: getter(obj) — exactly one positional arg; peel wrappers that
	// preserve the underlying typed local's field leaves.
	objArgs, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(objArgs) != 1 {
		return ""
	}
	objLocal := pythonOperatorGetterObjectLocal(objArgs[0], content, name)
	if objLocal != "" && fieldOf != nil {
		if t := fieldOf[objLocal+"."+field]; t != "" {
			return t
		}
	}
	// attrgetter("a")(w.get()) / itemgetter("a")(w.get()) / Class() object —
	// type-level fieldOf (same leaf as w.get().a / Box(A()).a under foreign same-leaf).
	if fieldOf != nil && objArgs[0].Type() == "call" {
		if tn := pythonCallFuncReturnType(objArgs[0], content, funcReturns, typeOf, fieldOf); tn != "" {
			if t := fieldOf[tn+"."+field]; t != "" {
				return t
			}
		}
		if tn := pythonExprClassType(objArgs[0], content); tn != "" {
			if t := fieldOf[tn+"."+field]; t != "" {
				return t
			}
		}
	}
	// attrgetter("k")(SimpleNamespace(k=A())) / types.SimpleNamespace — inline SNS.
	if name == "attrgetter" {
		return pythonInlineSimpleNamespaceFieldType(objArgs[0], content, field)
	}
	// itemgetter("k")(vars(SimpleNamespace(k=A()))) / itemgetter("k")(SNS(...).__dict__)
	// — same leaf as vars(SNS(...))["k"] under foreign same-leaf.
	if name == "itemgetter" {
		return pythonInlineSimpleNamespaceDictViewFieldType(objArgs[0], content, field, funcReturns, typeOf, fieldOf)
	}
	return ""
}

// pythonInlineSimpleNamespaceFieldType recovers T from SimpleNamespace(k=A()) /
// types.SimpleNamespace(k=A()) field k under foreign same-leaf. Keyword Class()
// fields only; parenthesized forms peel. Mixed / non-SNS fail closed.
func pythonInlineSimpleNamespaceFieldType(n *grammar.Node, content []byte, field string) string {
	return pythonInlineSimpleNamespaceObjectFieldTypeOn(n, content, field, nil, nil, nil)
}

// pythonInlineSimpleNamespaceObjectFieldType recovers T from
// SimpleNamespace(k=ba.get()).k attribute access under foreign same-leaf.
func pythonInlineSimpleNamespaceObjectFieldType(attr *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if attr == nil || attr.Type() != "attribute" {
		return ""
	}
	fieldN := ingest.ChildByField(attr, "attribute")
	obj := ingest.ChildByField(attr, "object")
	if fieldN == nil || obj == nil {
		return ""
	}
	return pythonInlineSimpleNamespaceObjectFieldTypeOn(obj, content, ingest.NodeText(fieldN, content), funcReturns, typeOf, fieldOf)
}

func pythonInlineSimpleNamespaceObjectFieldTypeOn(n *grammar.Node, content []byte, field string, funcReturns, typeOf, fieldOf map[string]string) string {
	if n == nil || field == "" {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonInlineSimpleNamespaceObjectFieldTypeOn(pythonParenInner(n), content, field, funcReturns, typeOf, fieldOf)
	}
	if n.Type() != "call" {
		return ""
	}
	fields, _ := pythonSimpleNamespaceObjectFieldTypes(n, content, funcReturns, typeOf, fieldOf)
	if fields == nil {
		return ""
	}
	return fields[field]
}

// pythonOperatorGetterObjectLocal recovers the identifier local whose fields are
// accessed by attrgetter/itemgetter. attrgetter peels replace (same object type);
// itemgetter peels asdict/vars/__dict__ (dict-view field keys) or bare identifier
// (TypedDict/record). Cross peels fail closed (attrgetter on asdict, itemgetter
// on replace). Parenthesized forms peel.
func pythonOperatorGetterObjectLocal(n *grammar.Node, content []byte, name string) string {
	if n == nil || name == "" {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonOperatorGetterObjectLocal(pythonParenInner(n), content, name)
	}
	switch name {
	case "attrgetter":
		// box / replace(box) — attribute access on the dataclass object.
		return pythonReplacePeeledObjectLocal(n, content)
	case "itemgetter":
		// box (TypedDict/record) or asdict/vars/__dict__ view of box.
		if n.Type() == "identifier" {
			return ingest.NodeText(n, content)
		}
		return pythonDictViewObjectLocal(n, content)
	}
	return ""
}

// pythonOperatorGetterLocalSpec recovers "attrgetter:FIELD" / "itemgetter:FIELD" /
// "itemgetter:#" from attrgetter("a") / itemgetter(0) / operator.* forms.
// Single positional arg only — multi-field/multi-index getters fail closed.
// Numeric / non-string itemgetter args are element getters ("itemgetter:#";
// index ignored for typing, same as pythonItemgetterElemType).
func pythonOperatorGetterLocalSpec(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	var name string
	switch fn.Type() {
	case "identifier":
		name = ingest.NodeText(fn, content)
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "operator" {
			return ""
		}
		name = ingest.NodeText(attr, content)
	default:
		return ""
	}
	if name != "attrgetter" && name != "itemgetter" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	switch name {
	case "attrgetter":
		if args[0].Type() != "string" {
			return ""
		}
		_, field := pythonStringContent(args[0], content)
		if field == "" {
			return ""
		}
		return "attrgetter:" + field
	case "itemgetter":
		if args[0].Type() == "string" {
			_, field := pythonStringContent(args[0], content)
			if field == "" {
				return ""
			}
			return "itemgetter:" + field
		}
		// Single-index element getter (integer / other); multi-arg already failed.
		return "itemgetter:#"
	}
	return ""
}

// pythonStoredOperatorGetterType recovers T from ga(box) / gi(items) when ga/gi
// is a stored operator getter bound via getterOf (ga = attrgetter("a") /
// gi = itemgetter(0) / operator.* forms). Field getters use fieldOf (same leaf
// as box.a / box["a"]); single-index itemgetter uses collection element type
// (same leaf as items[0]). Also peels method-return / Class object collections
// under foreign same-leaf:
//
//	gi = itemgetter(0); gi([ba.get()]) / gi([A()])
//	gk = itemgetter("k"); gk({"k": ba.get()})
//	ga = attrgetter("a"); ga(w.get()) / ga(Box(A()))
//
// Unknown locals / wrong arity / missing fieldOf fail closed.
func pythonStoredOperatorGetterType(call *grammar.Node, content []byte, getterOf, fieldOf, elemOf, egElems, typeOf, funcReturns map[string]string) string {
	if call == nil || call.Type() != "call" || getterOf == nil {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" {
		return ""
	}
	spec := getterOf[ingest.NodeText(fn, content)]
	if spec == "" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	kind, field, ok := strings.Cut(spec, ":")
	if !ok || kind == "" {
		return ""
	}
	switch kind {
	case "attrgetter":
		if field == "" {
			return ""
		}
		objLocal := pythonOperatorGetterObjectLocal(args[0], content, "attrgetter")
		if objLocal != "" && fieldOf != nil {
			if t := fieldOf[objLocal+"."+field]; t != "" {
				return t
			}
		}
		// ga(w.get()) / ga(Box(A())) after ga = attrgetter("a") — type-level
		// fieldOf under foreign same-leaf (same leaf as inline attrgetter).
		if fieldOf != nil && args[0].Type() == "call" {
			if tn := pythonCallFuncReturnType(args[0], content, funcReturns, typeOf, fieldOf); tn != "" {
				if t := fieldOf[tn+"."+field]; t != "" {
					return t
				}
			}
			if tn := pythonExprClassType(args[0], content); tn != "" {
				if t := fieldOf[tn+"."+field]; t != "" {
					return t
				}
			}
		}
		// ga(SimpleNamespace(k=A())) after ga = attrgetter("k") — inline SNS.
		return pythonInlineSimpleNamespaceFieldType(args[0], content, field)
	case "itemgetter":
		if field == "#" {
			// gi(items) — typed collection element.
			if et := pythonIterableElemType(args[0], content, elemOf, egElems, typeOf); et != "" {
				return et
			}
			// gi([ba.get()]) / gi([A()]) — method-return / Class object collection
			// under foreign same-leaf (same leaf as inline itemgetter(0)([...])).
			return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
		}
		if field == "" {
			return ""
		}
		objLocal := pythonOperatorGetterObjectLocal(args[0], content, "itemgetter")
		if objLocal != "" && fieldOf != nil {
			if t := fieldOf[objLocal+"."+field]; t != "" {
				return t
			}
		}
		// gk(w.get()) / gk(Box(A())) after gk = itemgetter("a") — type-level fieldOf.
		if fieldOf != nil && args[0].Type() == "call" {
			if tn := pythonCallFuncReturnType(args[0], content, funcReturns, typeOf, fieldOf); tn != "" {
				if t := fieldOf[tn+"."+field]; t != "" {
					return t
				}
			}
			if tn := pythonExprClassType(args[0], content); tn != "" {
				if t := fieldOf[tn+"."+field]; t != "" {
					return t
				}
			}
		}
		// gk({"k": ba.get()}) after gk = itemgetter("k") — homogeneous object-dict
		// values under foreign same-leaf (same leaf as inline itemgetter("k")({...})).
		return pythonHomogeneousObjectDictValue(args[0], content, funcReturns, typeOf, fieldOf)
	}
	return ""
}

// pythonCopyCallObjectType recovers T from copy.copy(x) / copy.deepcopy(x) /
// bare copy(x) / deepcopy(x) (from copy import copy, deepcopy) when x is a typed
// object local or Class() ctor (typeOf / ctor name), a field access box.a /
// replace(box).a (fieldOf; same leaf as box.a), a dict-view field key
// access asdict(box)["a"] / vars(box)["a"] / box.__dict__["a"] / .get("a")
// (fieldOf; same leaf as xa = asdict(box)["a"]), next(iter(vars(...).values())) /
// next(asdict(...).values()) first dict-view field,
// list/tuple(vars(...).values())[i] declaration-order index (same leaf as
// next(...values()).run() / list(...values())[i].run()), or a zero-arg
// method return ba.get() under foreign same-leaf (funcReturns; same leaf as
// ba.get().run()). Collection copies use pythonIterableElemType /
// pythonObjectCollectionElem instead. Wrong arity / other modules fail closed.
func pythonCopyCallObjectType(call *grammar.Node, content []byte, typeOf, fieldOf, funcReturns map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		// copy(x) / deepcopy(x) — from copy import copy, deepcopy.
		name := ingest.NodeText(fn, content)
		if name != "copy" && name != "deepcopy" {
			return ""
		}
	case "attribute":
		// copy.copy(x) / copy.deepcopy(x) — module-qualified.
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || obj == nil || obj.Type() != "identifier" {
			return ""
		}
		method := ingest.NodeText(attr, content)
		if method != "copy" && method != "deepcopy" {
			return ""
		}
		if ingest.NodeText(obj, content) != "copy" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	// Prefer structured peels before Class() ctor / typed-local peels:
	// pythonObjectExprType treats any identifier call as a Class ctor, so
	// next(...)/getattr(...)/attrgetter(...) would otherwise bind the bogus leaf
	// "next"/"getattr"/"attrgetter" and fail closed under foreign same-leaf methods.
	// copy.copy(next(iter(vars(SimpleNamespace(k=A())).values()))) /
	// copy.copy(next(asdict(box).values())) — first declaration-order field
	// (same leaf as next(...values()).run()). Only peel when arg is next(...).
	if args[0].Type() == "call" {
		if fn0 := ingest.ChildByField(args[0], "function"); fn0 != nil {
			switch fn0.Type() {
			case "identifier":
				switch ingest.NodeText(fn0, content) {
				case "next":
					if et := pythonAstupleNextFirstField(args[0], content, fieldOf); et != "" {
						return et
					}
				case "getattr":
					// copy.copy(getattr(box, "a")) / getattr(SimpleNamespace(k=A()), "k")
					// — same leaf as getattr(...).run() / attrgetter peels.
					if ft := pythonGetattrFieldType(args[0], content, fieldOf, funcReturns, typeOf); ft != "" {
						return ft
					}
				}
			case "call":
				// copy.copy(attrgetter("k")(SimpleNamespace(...))) /
				// copy.copy(itemgetter("k")(vars(SNS(...)))) — structured getter peels
				// before Class() ctor path (attrgetter/itemgetter would be bogus leaves).
				if ft := pythonAttrgetterFieldType(args[0], content, fieldOf, funcReturns, typeOf); ft != "" {
					return ft
				}
				if ft := pythonItemgetterFieldType(args[0], content, fieldOf, funcReturns, typeOf); ft != "" {
					return ft
				}
			}
		}
	}
	// copy.copy(list(vars(SimpleNamespace(k=A())).values())[0]) /
	// copy.copy(list(asdict(box).values())[i]) — declaration-order index peel.
	if et := pythonAstupleIndexAccessType(args[0], content, fieldOf); et != "" {
		return et
	}
	// copy.copy(box.a) / copy.copy(replace(box).a) — field type of the arg
	// (same leaf as box.a.run() / xa = box.a; xa.run()).
	if ft := pythonFieldAccessType(args[0], content, fieldOf, nil, nil); ft != "" {
		return ft
	}
	if ft := pythonReplaceFieldAccessType(args[0], content, fieldOf, funcReturns, typeOf); ft != "" {
		return ft
	}
	// copy.copy(asdict(box)["a"]) / copy.copy(asdict(box).get("a")) /
	// copy.copy(vars(box)["a"]) / copy.copy(box.__dict__["a"]) — field type of
	// the dict-view key (same leaf as xa = asdict(box)["a"]; xa.run()).
	if ft := pythonDictViewKeyAccessType(args[0], content, fieldOf, funcReturns, typeOf); ft != "" {
		return ft
	}
	// Typed local / Class() ctor (after next/getattr/subscript peels above).
	if tn := pythonObjectExprType(args[0], content, typeOf); tn != "" {
		return tn
	}
	// copy.copy(ba.get()) / copy.deepcopy(ba.get()) — zero-arg method return
	// under foreign same-leaf (Class/typed-local peels above).
	if tn := pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf); tn != "" {
		return tn
	}
	return ""
}

// pythonWeakrefCallObjectType recovers T from:
//
//	weakref.proxy(a).run() / weakref.proxy(A()) / weakref.proxy(ba.get())
//	weakref.ref(a)().run()  — outer call of a ref factory
//	ra() after ra = weakref.ref(a) is handled via factoryOf, not here.
//
// Referent type is the single positional arg's object type (typeOf / Class() /
// zero-arg method return via pythonObjectLeafType under foreign same-leaf).
// Other modules / arity / non-proxy/ref fail closed.
func pythonWeakrefCallObjectType(call *grammar.Node, content []byte, typeOf, funcReturns, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	// weakref.ref(a)() — function is itself a call.
	if fn.Type() == "call" {
		if pythonWeakrefFactoryName(fn, content) != "ref" {
			return ""
		}
		args, ok := pythonCallPositionalArgNodes(fn)
		if !ok || len(args) != 1 {
			return ""
		}
		if tn := pythonObjectExprType(args[0], content, typeOf); tn != "" {
			return tn
		}
		return pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf)
	}
	// weakref.proxy(a) / weakref.proxy(ba.get())
	if pythonWeakrefFactoryName(call, content) != "proxy" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	if tn := pythonObjectExprType(args[0], content, typeOf); tn != "" {
		return tn
	}
	// weakref.proxy(ba.get()) — zero-arg method return under foreign same-leaf
	// (Class/typed-local peels above via pythonObjectExprType).
	return pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonWeakrefFactoryName returns "ref" / "proxy" for weakref.ref / weakref.proxy
// calls (module-qualified only). Other shapes return "".
func pythonWeakrefFactoryName(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil || obj.Type() != "identifier" {
		return ""
	}
	if ingest.NodeText(obj, content) != "weakref" {
		return ""
	}
	name := ingest.NodeText(attr, content)
	if name == "ref" || name == "proxy" {
		return name
	}
	return ""
}

// pythonWeakrefRefFactoryType recovers T from weakref.ref(a) for factory-local
// binding (ra = weakref.ref(a); ra().run()). Proxy is not a factory (returns T
// directly). Non-ref / wrong arity fail closed.
func pythonWeakrefRefFactoryType(call *grammar.Node, content []byte, typeOf map[string]string) string {
	if pythonWeakrefFactoryName(call, content) != "ref" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	return pythonObjectExprType(args[0], content, typeOf)
}

// pythonReplaceCallObjectType recovers T from replace(x) / dataclasses.replace(x)
// (leaf name "replace", same as pythonReplaceKeywordEdits) when the first
// positional arg is a typed object local, Class() ctor, or method-return
// (ba.get()). Return type is the same as that arg (dataclasses.replace).
// Keywords after the object are ignored for typing. Other callees fail closed.
func pythonReplaceCallObjectType(call *grammar.Node, content []byte, typeOf, fieldOf, funcReturns map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || pythonSimpleCalleeName(fn, content) != "replace" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	// Typed local / Class() / conditional arms, then method-return (ba.get()).
	// Class assign already peels via ctor; method-return was UNDER under foreign
	// same-leaf (same leaf as copy.copy(ba.get())).
	return pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonNamedtupleReplaceObjectLocal recovers ba from ba._replace(...).
// Zero or more keyword overrides; positional args fail closed ("").
func pythonNamedtupleReplaceObjectLocal(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil || obj.Type() != "identifier" {
		return ""
	}
	if ingest.NodeText(attr, content) != "_replace" {
		return ""
	}
	// Reject positional overrides (namedtuple _replace is keyword-only).
	if args, ok := pythonCallPositionalArgNodes(call); ok && len(args) > 0 {
		return ""
	}
	return ingest.NodeText(obj, content)
}

// pythonBindNamedtupleReplaceLocal binds ra = ba._replace(a=A()) with instance
// fieldOf copied from ba and keyword overrides applied. Dual-class solid.
func pythonBindNamedtupleReplaceLocal(local string, call *grammar.Node, content []byte, typeOf, fieldOf map[string]string, fieldNames map[string][]string, fieldIndex map[string]map[string]string, ourReceivers, out map[string]bool) bool {
	src := pythonNamedtupleReplaceObjectLocal(call, content)
	if src == "" || local == "" || fieldOf == nil {
		return false
	}
	// Type of replace result is the same namedtuple as src.
	if tn := typeOf[src]; tn != "" {
		typeOf[local] = tn
		pythonBindClassLocalFields(local, tn, fieldIndex, fieldOf)
		pythonBindNamedtupleIndexFields(local, tn, fieldNames, fieldIndex, fieldOf)
	}
	pythonCopyLocalFieldOf(src, local, fieldOf)
	// Keyword overrides: ba._replace(a=A()) → field a is A.
	argList := ingest.ChildByField(call, "arguments")
	if argList != nil {
		for i := uint32(0); i < argList.ChildCount(); i++ {
			ch := argList.Child(i)
			if ch == nil || ch.Type() != "keyword_argument" {
				continue
			}
			nameN := ingest.ChildByField(ch, "name")
			valN := ingest.ChildByField(ch, "value")
			if nameN == nil || nameN.Type() != "identifier" || valN == nil {
				continue
			}
			fname := ingest.NodeText(nameN, content)
			if et := pythonExprClassType(valN, content); et != "" && fname != "" {
				fieldOf[local+"."+fname] = et
			}
		}
	}
	// Index aliases from named field order when known.
	if tn := typeOf[local]; tn != "" {
		if names := fieldNames[tn]; len(names) > 0 {
			for i, fname := range names {
				if t := fieldOf[local+"."+fname]; t != "" {
					fieldOf[local+".#"+fmt.Sprintf("%d", i)] = t
				}
			}
		}
	}
	return true
}

// pythonNamedtupleReplaceFieldAccessType recovers T from ba._replace(...).a /
// ba._replace(a=A()).a — keyword override wins, else fieldOf[ba.a].
func pythonNamedtupleReplaceFieldAccessType(attr *grammar.Node, content []byte, fieldOf map[string]string) string {
	if attr == nil || attr.Type() != "attribute" || fieldOf == nil {
		return ""
	}
	field := ingest.ChildByField(attr, "attribute")
	obj := ingest.ChildByField(attr, "object")
	if field == nil || obj == nil || obj.Type() != "call" {
		return ""
	}
	fname := ingest.NodeText(field, content)
	if fname == "" {
		return ""
	}
	// Keyword override on this _replace call.
	argList := ingest.ChildByField(obj, "arguments")
	if argList != nil {
		for i := uint32(0); i < argList.ChildCount(); i++ {
			ch := argList.Child(i)
			if ch == nil || ch.Type() != "keyword_argument" {
				continue
			}
			nameN := ingest.ChildByField(ch, "name")
			valN := ingest.ChildByField(ch, "value")
			if nameN == nil || ingest.NodeText(nameN, content) != fname || valN == nil {
				continue
			}
			if et := pythonExprClassType(valN, content); et != "" {
				return et
			}
		}
	}
	src := pythonNamedtupleReplaceObjectLocal(obj, content)
	if src == "" {
		return ""
	}
	return fieldOf[src+"."+fname]
}

// pythonNamedtupleMakeCallType recovers (typeName, field→Class) from
// Box._make([A()]) / Box._make((A(), B())) when field order is known.
// First arg must be list/tuple of Class() elems matching field count (or a
// single-field list). Other forms fail closed.
func pythonNamedtupleMakeInstanceFields(call *grammar.Node, content []byte, fieldNames map[string][]string) (typeName string, fields map[string]string) {
	if call == nil || call.Type() != "call" || fieldNames == nil {
		return "", nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return "", nil
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil || obj.Type() != "identifier" {
		return "", nil
	}
	if ingest.NodeText(attr, content) != "_make" {
		return "", nil
	}
	typeName = ingest.NodeText(obj, content)
	names := fieldNames[typeName]
	if len(names) == 0 {
		return "", nil
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return "", nil
	}
	seq := args[0]
	if seq.Type() != "list" && seq.Type() != "tuple" {
		return "", nil
	}
	var elems []*grammar.Node
	for i := uint32(0); i < seq.ChildCount(); i++ {
		ch := seq.Child(i)
		if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		elems = append(elems, ch)
	}
	if len(elems) == 0 || len(elems) > len(names) {
		return "", nil
	}
	fields = map[string]string{}
	for i, el := range elems {
		et := pythonExprClassType(el, content)
		if et == "" {
			return "", nil
		}
		fields[names[i]] = et
	}
	return typeName, fields
}

// pythonBindNamedtupleMakeLocal binds ba = Box._make([A()]) with instance fields.
func pythonBindNamedtupleMakeLocal(local string, call *grammar.Node, content []byte, typeOf, fieldOf map[string]string, fieldNames map[string][]string, fieldIndex map[string]map[string]string, ourReceivers, out map[string]bool) bool {
	tn, fields := pythonNamedtupleMakeInstanceFields(call, content, fieldNames)
	if tn == "" || len(fields) == 0 || local == "" || fieldOf == nil {
		return false
	}
	typeOf[local] = tn
	pythonBindClassLocalFields(local, tn, fieldIndex, fieldOf)
	pythonBindNamedtupleIndexFields(local, tn, fieldNames, fieldIndex, fieldOf)
	for f, t := range fields {
		if f != "" && t != "" {
			fieldOf[local+"."+f] = t
		}
	}
	if names := fieldNames[tn]; len(names) > 0 {
		for i, fname := range names {
			if t := fields[fname]; t != "" {
				fieldOf[local+".#"+fmt.Sprintf("%d", i)] = t
			}
		}
	}
	return true
}

// pythonNamedtupleMakeFieldAccessType recovers T from Box._make([A()]).a when
// the make sequence has a single Class() element (single-field namedtuple —
// dual-class solid). Multi-field inline .field fails closed (use assign bind).
func pythonNamedtupleMakeFieldAccessType(attr *grammar.Node, content []byte) string {
	if attr == nil || attr.Type() != "attribute" {
		return ""
	}
	field := ingest.ChildByField(attr, "attribute")
	obj := ingest.ChildByField(attr, "object")
	if field == nil || obj == nil || obj.Type() != "call" {
		return ""
	}
	if ingest.NodeText(field, content) == "" {
		return ""
	}
	elems := pythonNamedtupleMakeSeqElems(obj, content)
	// Single-element make: only one field value; .a peels that Class leaf.
	if len(elems) != 1 {
		return ""
	}
	return pythonExprClassType(elems[0], content)
}

// pythonNamedtupleMakeIndexType recovers T from Box._make([A(), B()])[i] when
// index i is a decimal integer selecting a Class() sequence element.
func pythonNamedtupleMakeIndexType(sub *grammar.Node, content []byte) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	val := ingest.ChildByField(sub, "value")
	if val == nil {
		val = ingest.ChildByField(sub, "object")
	}
	keyN := ingest.ChildByField(sub, "subscript")
	if val == nil || keyN == nil || val.Type() != "call" {
		return ""
	}
	idx := pythonNonNegIntLiteral(keyN, content)
	if idx < 0 {
		return ""
	}
	elems := pythonNamedtupleMakeSeqElems(val, content)
	if idx >= len(elems) {
		return ""
	}
	return pythonExprClassType(elems[idx], content)
}

// pythonNamedtupleMakeSeqElems recovers Class()-bearing list/tuple elements from
// Box._make([...]) / Box._make((...)). Non-_make / wrong arity fail closed (nil).
func pythonNamedtupleMakeSeqElems(call *grammar.Node, content []byte) []*grammar.Node {
	if call == nil || call.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return nil
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil || obj.Type() != "identifier" {
		return nil
	}
	if ingest.NodeText(attr, content) != "_make" {
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return nil
	}
	seq := args[0]
	if seq.Type() != "list" && seq.Type() != "tuple" {
		return nil
	}
	var elems []*grammar.Node
	for i := uint32(0); i < seq.ChildCount(); i++ {
		ch := seq.Child(i)
		if ch == nil || ch.Type() == "[" || ch.Type() == "]" || ch.Type() == "(" || ch.Type() == ")" || ch.Type() == "," {
			continue
		}
		elems = append(elems, ch)
	}
	return elems
}

// pythonAsdictCallObjectType recovers T from asdict(x) / dataclasses.asdict(x)
// when the first positional arg is a typed object local or Class() ctor.
// asdict returns a dict of field values of that object (field keys via
// bindFields); not the object itself. Keywords (dict_factory=) ignored for
// typing. Other callees / missing first arg fail closed.
func pythonAsdictCallObjectType(call *grammar.Node, content []byte, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || pythonSimpleCalleeName(fn, content) != "asdict" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonObjectExprType(args[0], content, typeOf)
}

// pythonAstupleCallObjectType recovers T from astuple(x) / dataclasses.astuple(x)
// when the first positional arg is a typed object local, Class() ctor, or
// replace(x) / dataclasses.replace(x) of those (return type of replace is the
// dataclass of its first arg — same leaf as astuple(box)).
// Also peels identity wrappers list(astuple(x)) / tuple(astuple(x)) that preserve
// declaration-order field slots (xs = list(astuple(box)); xs[0] / unpack).
// astuple returns a tuple of field values in declaration order (index slots via
// pythonBindNamedtupleIndexFields + fieldOrder); not the object itself.
// Keywords (tuple_factory=) ignored for typing. Other callees / missing first
// arg fail closed.
func pythonAstupleCallObjectType(call *grammar.Node, content []byte, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	name := pythonSimpleCalleeName(fn, content)
	// list(astuple(box)) / tuple(astuple(box)) — same ordered slots as bare astuple.
	if name == "list" || name == "tuple" {
		args, ok := pythonCallPositionalArgNodes(call)
		if !ok || len(args) != 1 || args[0].Type() != "call" {
			return ""
		}
		return pythonAstupleCallObjectType(args[0], content, typeOf)
	}
	if name != "astuple" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	// replace(box) before Class() ctor peel: bare replace(x) is an identifier
	// call and would otherwise look like a ctor named "replace". fieldOf /
	// funcReturns nil here — method-return first args fail closed on this path
	// (astuple(replace(ba.get())) not required; Class/typed-local peels).
	if tn := pythonReplaceCallObjectType(args[0], content, typeOf, nil, nil); tn != "" {
		return tn
	}
	return pythonObjectExprType(args[0], content, typeOf)
}

// pythonAstupleFieldTypes recovers declaration-order field type leaves from
// astuple(box) / dataclasses.astuple(box) for unpack `xa, xb = astuple(box)`
// (same order as t = astuple(box); t[i]). Missing types yield "" slots (fail
// closed per target). Other callees / unknown object type fail closed (nil).
func pythonAstupleFieldTypes(call *grammar.Node, content []byte, typeOf map[string]string, fieldOrder map[string][]string, fieldIndex map[string]map[string]string) []string {
	tn := pythonAstupleCallObjectType(call, content, typeOf)
	if tn == "" || fieldOrder == nil || fieldIndex == nil {
		return nil
	}
	names := fieldOrder[tn]
	fields := fieldIndex[tn]
	if len(names) == 0 || fields == nil {
		return nil
	}
	out := make([]string, len(names))
	for i, fname := range names {
		out[i] = fields[fname]
	}
	return out
}

// pythonVarsCallObjectType recovers T from vars(x) when the first positional
// arg is a typed object local or Class() ctor. vars returns x.__dict__ (field
// keys via bindFields); bare builtin name only — other modules fail closed.
func pythonVarsCallObjectType(call *grammar.Node, content []byte, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	// Bare vars only (builtin); attribute forms are not the builtin.
	if fn == nil || fn.Type() != "identifier" || ingest.NodeText(fn, content) != "vars" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonObjectExprType(args[0], content, typeOf)
}

// pythonDunderDictObjectType recovers T from x.__dict__ when x is a typed object
// local or Class() ctor. __dict__ is the instance attribute dict (field keys via
// bindFields; same leaf as vars(x) / asdict(x)). Other attributes fail closed.
func pythonDunderDictObjectType(attr *grammar.Node, content []byte, typeOf map[string]string) string {
	if attr == nil || attr.Type() != "attribute" {
		return ""
	}
	field := ingest.ChildByField(attr, "attribute")
	obj := ingest.ChildByField(attr, "object")
	if field == nil || obj == nil || ingest.NodeText(field, content) != "__dict__" {
		return ""
	}
	return pythonObjectExprType(obj, content, typeOf)
}

// pythonAstupleIndexAccessType recovers T from astuple(box)[0] /
// dataclasses.astuple(box)[0] / astuple(replace(box))[0] /
// list(astuple(box))[0] / tuple(astuple(box))[0] when box is a typed local with
// declaration-order field 0 of type T (fieldOf["@astuple.box.#0"]; same leaf as
// t = astuple(box); t[0] / box.a). Also list/tuple(asdict(box).values())[i] /
// list(vars(box).values())[i] / list(box.__dict__.values())[i] /
// d = asdict(box); list(d.values())[i] (dict preserves declaration order —
// values()[i] is field i; same leaf as next(asdict(box).values()) / box.a).
// Also list/tuple(vars(SimpleNamespace(...)).values())[i] /
// list(SimpleNamespace(...).__dict__.values())[i] — inline SNS order (no fieldOf).
// First positional arg must be an identifier local or replace(local); bare
// dict_values is not indexable so list/tuple wrap is required; non-decimal
// integer indices and other callees fail closed. Does not treat bare box[0]
// as valid (synthetic @astuple. prefix — dataclasses are not indexable).
func pythonAstupleIndexAccessType(sub *grammar.Node, content []byte, fieldOf map[string]string) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	val := ingest.ChildByField(sub, "value")
	if val == nil {
		val = ingest.ChildByField(sub, "object")
	}
	idxN := ingest.ChildByField(sub, "subscript")
	if val == nil || idxN == nil || idxN.Type() != "integer" {
		return ""
	}
	idxText := ingest.NodeText(idxN, content)
	if idxText == "" {
		return ""
	}
	for _, c := range idxText {
		if c < '0' || c > '9' {
			return ""
		}
	}
	if fieldOf != nil {
		objLocal := pythonAstupleObjectLocal(val, content)
		if objLocal == "" {
			// list/tuple(asdict(box).values())[i] / list(d.values())[i]
			objLocal = pythonDictViewValuesSeqLocal(val, content)
		}
		if objLocal != "" {
			if t := fieldOf["@astuple."+objLocal+".#"+idxText]; t != "" {
				return t
			}
		}
	}
	// list(vars(SimpleNamespace(k=A())).values())[0] — inline SNS order.
	return pythonInlineSimpleNamespaceDictViewValuesIndexType(val, content, idxText)
}

// pythonInlineSimpleNamespaceDictViewValuesIndexType recovers declaration-order
// field i from list/tuple(vars(SimpleNamespace(...)).values()) /
// list(SimpleNamespace(...).__dict__.values()) under foreign same-leaf.
// Bare dict_values is not indexable — list/tuple wrap required. Out-of-range
// / non-inline forms fail closed.
func pythonInlineSimpleNamespaceDictViewValuesIndexType(n *grammar.Node, content []byte, idxText string) string {
	order := pythonInlineSimpleNamespaceDictViewValuesOrder(n, content)
	if len(order) == 0 || idxText == "" {
		return ""
	}
	idx := 0
	for _, c := range idxText {
		if c < '0' || c > '9' {
			return ""
		}
		idx = idx*10 + int(c-'0')
	}
	if idx < 0 || idx >= len(order) {
		return ""
	}
	return order[idx]
}

// pythonInlineSimpleNamespaceDictViewValuesOrder recovers declaration-order
// Class leaves from list/tuple(vars(SimpleNamespace(...)).values()) /
// vars(SimpleNamespace(...)).values() / SimpleNamespace(...).__dict__.values().
// Peels list/tuple wrappers only. Other forms fail closed (nil).
func pythonInlineSimpleNamespaceDictViewValuesOrder(n *grammar.Node, content []byte) []string {
	if n == nil {
		return nil
	}
	if n.Type() == "parenthesized_expression" {
		return pythonInlineSimpleNamespaceDictViewValuesOrder(pythonParenInner(n), content)
	}
	if n.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return nil
	}
	name := pythonSimpleCalleeName(fn, content)
	// list(...values()) / tuple(...values()) — materialize order-preserving sequence.
	if name == "list" || name == "tuple" {
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 1 {
			return nil
		}
		return pythonInlineSimpleNamespaceDictViewValuesOrder(args[0], content)
	}
	if name != "values" || fn.Type() != "attribute" {
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(n)
	if !ok || len(args) != 0 {
		return nil
	}
	obj := ingest.ChildByField(fn, "object")
	call := pythonInlineSimpleNamespaceDictViewCall(obj, content)
	if call == nil {
		return nil
	}
	_, order := pythonSimpleNamespaceFieldTypes(call, content)
	if len(order) == 0 {
		return nil
	}
	return order
}

// pythonHomogeneousAstupleFieldType returns the shared field type when every
// declaration-order slot fieldOf["@astuple.local.#i"] agrees. Empty or mixed
// slots fail closed (""). Used for for-x-in asdict(...).values() / astuple(...)
// when values are homogeneous (mixed dataclass fields correctly stay unbound).
func pythonHomogeneousAstupleFieldType(local string, fieldOf map[string]string) string {
	if local == "" || fieldOf == nil {
		return ""
	}
	prefix := "@astuple." + local + ".#"
	var shared string
	count := 0
	for i := 0; ; i++ {
		t := fieldOf[prefix+fmt.Sprintf("%d", i)]
		if t == "" {
			break
		}
		count++
		if shared == "" {
			shared = t
		} else if t != shared {
			return ""
		}
	}
	if count == 0 {
		return ""
	}
	return shared
}

// pythonAstupleHomogeneousType recovers the shared element type of
// astuple(box) / dataclasses.astuple(box) / list/tuple/iter...(astuple(box))
// when all declaration-order field types agree (fieldOf @astuple.*.#i). Peels
// identity wrappers list/tuple/iter/reversed/sorted/set/frozenset/filter that
// preserve the field tuple. Mixed field types and non-astuple forms fail closed
// (""). Same leaf as for x in asdict(box).values() when values are uniform.
func pythonAstupleHomogeneousType(n *grammar.Node, content []byte, fieldOf map[string]string) string {
	if n == nil || fieldOf == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonAstupleHomogeneousType(pythonParenInner(n), content, fieldOf)
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	name := pythonSimpleCalleeName(fn, content)
	// Peel element-preserving wrappers around an astuple field tuple.
	switch name {
	case "list", "tuple", "iter", "reversed", "sorted", "set", "frozenset":
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) == 0 {
			return ""
		}
		return pythonAstupleHomogeneousType(args[0], content, fieldOf)
	case "filter":
		// filter(pred, iterable) — pred only selects; keep field element type.
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) < 2 {
			return ""
		}
		return pythonAstupleHomogeneousType(args[1], content, fieldOf)
	}
	local := pythonAstupleObjectLocal(n, content)
	if local == "" {
		return ""
	}
	return pythonHomogeneousAstupleFieldType(local, fieldOf)
}

// pythonNextItemsValueType recovers the value leaf from next(...items()) unpack
// (see pythonCallItemsValueType). Same leaf as for k, x in d.items() /
// for k, x in asdict(...).items() when values are homogeneous.
func pythonNextItemsValueType(call *grammar.Node, content []byte, elemOf, fieldOf map[string]string) string {
	return pythonCallItemsValueType(call, content, elemOf, fieldOf, "next")
}

// pythonMinMaxItemsValueType recovers the value leaf from min/max(...items()) unpack:
// k, x = min(asdict(pair).items(), key=...) / max(d.items()) when values are
// homogeneous / typed-dict. Single-positional form only (min(a, b) fails closed).
// Same leaf as next(...items()) / for k, x in ...items().
func pythonMinMaxItemsValueType(call *grammar.Node, content []byte, elemOf, fieldOf map[string]string) string {
	return pythonCallItemsValueType(call, content, elemOf, fieldOf, "min", "max")
}

// pythonCallItemsValueType recovers the value leaf from next/min/max of an items()
// view used as a pair source:
//   - k, a = next(d.items()) / next(iter(d.items())) when d is a known dict
//     (elemOf stores the value leaf from dict[K, V])
//   - k, x = next(asdict(pair).items()) / min(asdict(pair).items(), key=...) /
//     vars(pair).items() / pair.__dict__.items() / d.items() after d = asdict(pair)
//     when all declaration-order field types agree (fieldOf @astuple.*.#i)
//
// Peels identity wrappers iter/list/tuple on the items view. Default/key kwargs
// ignored. Key slot stays untyped; mixed asdict fields and non-items forms fail
// closed (""). callees lists accepted bare function names (next / min / max).
// For min/max, requires exactly one positional arg (same as element typing).
func pythonCallItemsValueType(call *grammar.Node, content []byte, elemOf, fieldOf map[string]string, callees ...string) string {
	if call == nil || call.Type() != "call" || len(callees) == 0 {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" {
		return ""
	}
	fname := ingest.NodeText(fn, content)
	okName := false
	for _, c := range callees {
		if fname == c {
			okName = true
			break
		}
	}
	if !okName {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	// min/max multi-arg form fails closed (not an items fold).
	if (fname == "min" || fname == "max") && len(args) != 1 {
		return ""
	}
	n := args[0]
	// Peel identity wrappers that preserve items() pair yields: iter / list / tuple.
	for {
		if n == nil {
			return ""
		}
		if n.Type() == "parenthesized_expression" {
			n = pythonParenInner(n)
			continue
		}
		if n.Type() != "call" {
			break
		}
		wfn := ingest.ChildByField(n, "function")
		name := pythonSimpleCalleeName(wfn, content)
		if name != "iter" && name != "list" && name != "tuple" {
			break
		}
		wargs, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(wargs) == 0 {
			return ""
		}
		n = wargs[0]
	}
	if vt := pythonDictItemsValueType(n, content, elemOf); vt != "" {
		return vt
	}
	return pythonDictViewItemsHomogeneousValueType(n, content, fieldOf)
}

// pythonItemsCallPairSlots returns ["", valueType] for next/min/max(...items())
// when the value leaf is known (key untyped). Used to bind pairSlots on
// p = next(...items()) so later k, x = p / p[1] type. Empty when unknown.
func pythonItemsCallPairSlots(call *grammar.Node, content []byte, elemOf, fieldOf map[string]string) []string {
	vt := pythonNextItemsValueType(call, content, elemOf, fieldOf)
	if vt == "" {
		vt = pythonMinMaxItemsValueType(call, content, elemOf, fieldOf)
	}
	if vt == "" {
		return nil
	}
	return []string{"", vt}
}

// pythonItemsViewPairSlots returns ["", valueType] for d.items() /
// asdict(box).items() / vars(box).items() / box.__dict__.items() /
// list/tuple/iter(...items()) when the value leaf is known (key untyped).
// Used to bind pairSlots on for item in d.items() so item[1] / k, a = item type
// (same leaf as p = next(...items())). Mixed asdict fields and non-items forms
// fail closed (nil).
func pythonItemsViewPairSlots(n *grammar.Node, content []byte, elemOf, fieldOf map[string]string) []string {
	if n == nil {
		return nil
	}
	// Peel identity wrappers that preserve items() pair yields: iter / list / tuple.
	for {
		if n == nil {
			return nil
		}
		if n.Type() == "parenthesized_expression" {
			n = pythonParenInner(n)
			continue
		}
		if n.Type() != "call" {
			break
		}
		wfn := ingest.ChildByField(n, "function")
		name := pythonSimpleCalleeName(wfn, content)
		if name != "iter" && name != "list" && name != "tuple" {
			break
		}
		wargs, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(wargs) == 0 {
			return nil
		}
		n = wargs[0]
	}
	if vt := pythonDictItemsValueType(n, content, elemOf); vt != "" {
		return []string{"", vt}
	}
	if vt := pythonDictViewItemsHomogeneousValueType(n, content, fieldOf); vt != "" {
		return []string{"", vt}
	}
	return nil
}

// pythonItemsCallSubscriptValueType returns the value leaf of
// next(...items())[1] / min(...items())[1] / (next(...items()))[1].
// Index must be integer literal 1 (value slot); [0]/other fail closed.
func pythonItemsCallSubscriptValueType(sub *grammar.Node, content []byte, elemOf, fieldOf map[string]string) string {
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
	if vt := pythonNextItemsValueType(val, content, elemOf, fieldOf); vt != "" {
		return vt
	}
	return pythonMinMaxItemsValueType(val, content, elemOf, fieldOf)
}

// pythonItemsIndexValueType recovers T from the deep items stack
// list/tuple(...items())[i][1] (and parenthesized forms):
//   - list(asdict(box).items())[0][1] / tuple(vars(box).items())[0][1] /
//     list(box.__dict__.items())[0][1] / d = asdict(box); list(d.items())[0][1]
//     → fieldOf[@astuple.local.#i] (declaration-order; same leaf as
//     list(...values())[i] / asdict(box)["field_i"])
//   - list(d.items())[0][1] when d: dict[K, A] → elemOf[d] (homogeneous values;
//     any non-slice index yields the same value leaf)
//
// Outer value-slot index must be integer literal 1; pair index i is a
// non-negative integer literal. [0] (key), slices, and unknown forms fail closed.
func pythonItemsIndexValueType(sub *grammar.Node, content []byte, elemOf, fieldOf map[string]string) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	valIdx := ingest.ChildByField(sub, "subscript")
	if valIdx == nil || valIdx.Type() != "integer" || ingest.NodeText(valIdx, content) != "1" {
		return ""
	}
	pairExpr := ingest.ChildByField(sub, "value")
	for pairExpr != nil && pairExpr.Type() == "parenthesized_expression" {
		pairExpr = pythonParenInner(pairExpr)
	}
	if pairExpr == nil || pairExpr.Type() != "subscript" {
		return ""
	}
	for i := uint32(0); i < pairExpr.ChildCount(); i++ {
		if pairExpr.Child(i).Type() == "slice" {
			return ""
		}
	}
	pairIdxN := ingest.ChildByField(pairExpr, "subscript")
	if pairIdxN == nil || pairIdxN.Type() != "integer" {
		return ""
	}
	pairIdx := ingest.NodeText(pairIdxN, content)
	if pairIdx == "" {
		return ""
	}
	for _, c := range pairIdx {
		if c < '0' || c > '9' {
			return ""
		}
	}
	seq := ingest.ChildByField(pairExpr, "value")
	for seq != nil && seq.Type() == "parenthesized_expression" {
		seq = pythonParenInner(seq)
	}
	if seq == nil {
		return ""
	}
	// asdict/vars/__dict__ items — index-aware field leaf (mixed OK).
	if local := pythonDictViewItemsSeqLocal(seq, content); local != "" && fieldOf != nil {
		if ft := fieldOf["@astuple."+local+".#"+pairIdx]; ft != "" {
			return ft
		}
	}
	// list(vars(SimpleNamespace(...)).items())[i][1] — declaration-order field i
	// of inline SNS (same leaf as list(...values())[i]).
	if t := pythonInlineSimpleNamespaceItemsSeqIndexType(seq, content, pairIdx); t != "" {
		return t
	}
	// list(dict.fromkeys(keys, A()).items())[i][1] — value leaf A (index ignored).
	if t := pythonFromkeysItemsSeqValueType(seq, content); t != "" {
		return t
	}
	// typed dict list(d.items())[i][1] — homogeneous value leaf (index ignored).
	if vt := pythonDictItemsSeqValueType(seq, content, elemOf); vt != "" {
		return vt
	}
	return ""
}

// pythonInlineSimpleNamespaceItemsSeqIndexType recovers field type at index i
// from list/tuple(vars(SimpleNamespace(...)).items()) / list(SNS(...).__dict__.items()).
// Same leaf as list(vars(SNS).values())[i]. Other forms fail closed.
func pythonInlineSimpleNamespaceItemsSeqIndexType(seq *grammar.Node, content []byte, pairIdx string) string {
	itemsCall := pythonItemsSeqItemsCall(seq, content)
	if itemsCall == nil {
		return ""
	}
	fn := ingest.ChildByField(itemsCall, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
	snsCall := pythonInlineSimpleNamespaceDictViewCall(obj, content)
	if snsCall == nil {
		return ""
	}
	_, order := pythonSimpleNamespaceFieldTypes(snsCall, content)
	if len(order) == 0 {
		return ""
	}
	idx := 0
	for _, c := range pairIdx {
		if c < '0' || c > '9' {
			return ""
		}
		idx = idx*10 + int(c-'0')
	}
	if idx < 0 || idx >= len(order) {
		return ""
	}
	return order[idx]
}

// pythonFromkeysItemsSeqValueType recovers A from list/tuple(dict.fromkeys(keys, A()).items()).
func pythonFromkeysItemsSeqValueType(seq *grammar.Node, content []byte) string {
	itemsCall := pythonItemsSeqItemsCall(seq, content)
	if itemsCall == nil {
		return ""
	}
	fn := ingest.ChildByField(itemsCall, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
	return pythonDictFromkeysValueType(obj, content)
}

// pythonItemsSeqItemsCall peels list/tuple wrappers to the underlying .items() call.
func pythonItemsSeqItemsCall(n *grammar.Node, content []byte) *grammar.Node {
	if n == nil {
		return nil
	}
	if n.Type() == "parenthesized_expression" {
		return pythonItemsSeqItemsCall(pythonParenInner(n), content)
	}
	if n.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return nil
	}
	name := pythonSimpleCalleeName(fn, content)
	if name == "list" || name == "tuple" {
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 1 {
			return nil
		}
		return pythonItemsSeqItemsCall(args[0], content)
	}
	if name == "items" && fn.Type() == "attribute" {
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 0 {
			return nil
		}
		return n
	}
	return nil
}

// pythonDictViewItemsSeqLocal recovers the identifier local whose
// declaration-order field slots are exposed by list/tuple of a dict-view
// items() chain: list(asdict(box).items()) / tuple(vars(box).items()) /
// list(box.__dict__.items()) / list(d.items()) when d = asdict(box) /
// vars(box) / box.__dict__ (bindFields records @astuple.d.#i). Peels
// list/tuple wrappers only (bare dict_items is not indexable). Pair index i
// then value slot [1] is field i — same leaf as list(...values())[i]. Other
// forms fail closed ("").
func pythonDictViewItemsSeqLocal(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonDictViewItemsSeqLocal(pythonParenInner(n), content)
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	name := pythonSimpleCalleeName(fn, content)
	// list(...items()) / tuple(...items()) — materialize order-preserving sequence.
	if name == "list" || name == "tuple" {
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 1 {
			return ""
		}
		return pythonDictViewItemsSeqLocal(args[0], content)
	}
	if name != "items" || fn.Type() != "attribute" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(n)
	if !ok || len(args) != 0 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
	// Assigned dict-view local: d.items() after d = asdict(box) / vars / __dict__.
	if obj.Type() == "identifier" {
		return ingest.NodeText(obj, content)
	}
	return pythonDictViewObjectLocal(obj, content)
}

// pythonDictItemsSeqValueType recovers the homogeneous value leaf of
// list/tuple(d.items()) when d is a known dict/mapping local (elemOf).
// Peels list/tuple wrappers; bare d.items() without materialization fails
// closed for index chains (dict_items is not indexable). Same leaf as
// next(d.items()) unpack value / for k, a in d.items().
func pythonDictItemsSeqValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil || elemOf == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonDictItemsSeqValueType(pythonParenInner(n), content, elemOf)
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	name := pythonSimpleCalleeName(fn, content)
	if name == "list" || name == "tuple" {
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 1 {
			return ""
		}
		return pythonDictItemsSeqValueType(args[0], content, elemOf)
	}
	return pythonDictItemsValueType(n, content, elemOf)
}

// pythonDictViewItemsHomogeneousValueType recovers the shared value type of
// asdict(box).items() / vars(box).items() / box.__dict__.items() /
// d.items() after d = asdict(box) / vars / __dict__ /
// vars(SimpleNamespace(...)).items() / SimpleNamespace(...).__dict__.items() /
// dict.fromkeys(keys, A()).items() when all declaration-order field types agree
// (fieldOf @astuple.*.#i or inline SNS/fromkeys value). Mixed field types and
// non-dict-view forms fail closed (""). Same leaf as for k, x in d.items() with
// d: dict[K, A] when values are uniform (key slot stays untyped).
func pythonDictViewItemsHomogeneousValueType(n *grammar.Node, content []byte, fieldOf map[string]string) string {
	if n == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonDictViewItemsHomogeneousValueType(pythonParenInner(n), content, fieldOf)
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	if pythonSimpleCalleeName(fn, content) != "items" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(n)
	if !ok || len(args) != 0 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
	// Inline vars(SimpleNamespace(k=A())).items() — no fieldOf local needed.
	if t := pythonInlineSimpleNamespaceDictViewHomogeneousType(obj, content); t != "" {
		return t
	}
	// dict.fromkeys(keys, A()).items() — value leaf is A (same as .values()).
	if t := pythonDictFromkeysValueType(obj, content); t != "" {
		return t
	}
	if fieldOf == nil {
		return ""
	}
	var local string
	if obj.Type() == "identifier" {
		// Assigned dict-view local: bindFields recorded @astuple.d.#i.
		local = ingest.NodeText(obj, content)
	} else {
		local = pythonDictViewObjectLocal(obj, content)
	}
	return pythonHomogeneousAstupleFieldType(local, fieldOf)
}

// pythonDictViewValuesHomogeneousType recovers the shared element type of
// asdict(box).values() / vars(box).values() / box.__dict__.values() /
// d.values() after d = asdict(box) / vars / __dict__ when all declaration-order
// field types agree (fieldOf @astuple.*.#i). Peels identity wrappers
// list/tuple/iter/reversed/sorted/set/frozenset/filter that preserve the
// values view. Mixed field types and non-dict-view forms fail closed ("").
// Same leaf as for x in d.values() with d: dict[K, A] when values are uniform.
func pythonDictViewValuesHomogeneousType(n *grammar.Node, content []byte, fieldOf map[string]string) string {
	if n == nil || fieldOf == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonDictViewValuesHomogeneousType(pythonParenInner(n), content, fieldOf)
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	name := pythonSimpleCalleeName(fn, content)
	// Peel element-preserving wrappers around a values() view.
	switch name {
	case "list", "tuple", "iter", "reversed", "sorted", "set", "frozenset":
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) == 0 {
			return ""
		}
		return pythonDictViewValuesHomogeneousType(args[0], content, fieldOf)
	case "filter":
		// filter(pred, iterable) — pred only selects; keep values element type.
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) < 2 {
			return ""
		}
		return pythonDictViewValuesHomogeneousType(args[1], content, fieldOf)
	case "values":
		// asdict(box).values() / vars(box).values() / box.__dict__.values() /
		// d.values() after d = asdict(box) / vars / __dict__ /
		// vars(SimpleNamespace(...)).values() / SimpleNamespace(...).__dict__.values().
		if fn.Type() != "attribute" {
			return ""
		}
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 0 {
			return ""
		}
		obj := ingest.ChildByField(fn, "object")
		if obj == nil {
			return ""
		}
		var local string
		if obj.Type() == "identifier" {
			// Assigned dict-view local: bindFields recorded @astuple.d.#i.
			local = ingest.NodeText(obj, content)
		} else {
			local = pythonDictViewObjectLocal(obj, content)
		}
		if t := pythonHomogeneousAstupleFieldType(local, fieldOf); t != "" {
			return t
		}
		// Inline vars(SimpleNamespace(k=A())).values() — no fieldOf local needed.
		return pythonInlineSimpleNamespaceDictViewHomogeneousType(obj, content)
	}
	return ""
}

// pythonDictViewValuesFieldTypes recovers declaration-order field types from
// asdict(box).values() / vars(box).values() / box.__dict__.values() /
// list/tuple(...values()) / d.values() after d = asdict(box) / vars / __dict__
// (fieldOf @astuple.*.#i). Dict preserves declaration order so values() unpack
// slots match astuple unpack / list(...values())[i]. Empty / non-dict-view
// forms fail closed (nil).
func pythonDictViewValuesFieldTypes(n *grammar.Node, content []byte, fieldOf map[string]string) []string {
	if n == nil || fieldOf == nil {
		return nil
	}
	if n.Type() == "parenthesized_expression" {
		return pythonDictViewValuesFieldTypes(pythonParenInner(n), content, fieldOf)
	}
	local := pythonDictViewValuesSeqLocal(n, content)
	if local == "" {
		return nil
	}
	prefix := "@astuple." + local + ".#"
	var out []string
	for i := 0; ; i++ {
		t := fieldOf[prefix+fmt.Sprintf("%d", i)]
		if t == "" {
			break
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pythonDictViewValuesSeqLocal recovers the identifier local whose
// declaration-order field slots are exposed by list/tuple of a dict-view
// values() chain: list(asdict(box).values()) / tuple(vars(box).values()) /
// list(box.__dict__.values()) / list(d.values()) when d = asdict(box) /
// vars(box) / box.__dict__ (bindFields records @astuple.d.#i). Peels
// list/tuple wrappers only (bare dict_values is not indexable). Same leaves
// as list(astuple(box))[i] / next(asdict(box).values()). Other forms fail
// closed ("").
func pythonDictViewValuesSeqLocal(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonDictViewValuesSeqLocal(pythonParenInner(n), content)
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	name := pythonSimpleCalleeName(fn, content)
	// list(...values()) / tuple(...values()) — materialize order-preserving sequence.
	if name == "list" || name == "tuple" {
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 1 {
			return ""
		}
		return pythonDictViewValuesSeqLocal(args[0], content)
	}
	if name != "values" || fn.Type() != "attribute" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(n)
	if !ok || len(args) != 0 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil {
		return ""
	}
	// Assigned dict-view local: d.values() after d = asdict(box) / vars / __dict__.
	// bindFields(d, Box) records @astuple.d.#i (not d.#i — dicts stay non-indexable).
	if obj.Type() == "identifier" {
		return ingest.NodeText(obj, content)
	}
	return pythonDictViewObjectLocal(obj, content)
}

// pythonAstupleObjectLocal recovers the identifier local whose declaration-order
// field values are exposed by astuple(x) / dataclasses.astuple(x). Accepts bare
// identifier locals and replace(local) / dataclasses.replace(local) (same object
// type as local). Also peels identity wrappers list(astuple(...)) /
// tuple(astuple(...)) that preserve declaration-order slots (same index leaves
// as bare astuple). Other forms fail closed ("").
func pythonAstupleObjectLocal(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonAstupleObjectLocal(pythonParenInner(n), content)
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	name := pythonSimpleCalleeName(fn, content)
	// list(astuple(box)) / tuple(astuple(box)) — same ordered slots as bare astuple.
	if name == "list" || name == "tuple" {
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 1 {
			return ""
		}
		return pythonAstupleObjectLocal(args[0], content)
	}
	if name != "astuple" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(n)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonReplacePeeledObjectLocal(args[0], content)
}

// pythonReplacePeeledObjectLocal recovers an identifier local from box or
// replace(box) / dataclasses.replace(box) (return type of replace is the same
// dataclass as its first positional arg). Parenthesized forms peel. Other
// shapes fail closed ("").
func pythonReplacePeeledObjectLocal(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "identifier":
		return ingest.NodeText(n, content)
	case "parenthesized_expression":
		return pythonReplacePeeledObjectLocal(pythonParenInner(n), content)
	case "call":
		fn := ingest.ChildByField(n, "function")
		if fn == nil || pythonSimpleCalleeName(fn, content) != "replace" {
			return ""
		}
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) == 0 || args[0].Type() != "identifier" {
			return ""
		}
		return ingest.NodeText(args[0], content)
	}
	return ""
}

// pythonDictViewKeyAccessType recovers T from asdict(box)["a"] / vars(box)["a"] /
// box.__dict__["a"] / asdict(box).get("a") / vars(box).get("a") /
// box.__dict__.get("a") (also setdefault/pop) when box is a typed local with
// annotated field a of type T (fieldOf; same leaf as d = asdict(box); d["a"] /
// d.get("a") / box.a). Also peels inline vars(SimpleNamespace(k=A()))["k"] /
// SimpleNamespace(k=A()).__dict__["k"] / ba._asdict()["a"] (namedtuple) under
// foreign same-leaf. First positional arg / object is an identifier local, an
// inline SimpleNamespace(...), or ba._asdict(); non-string keys and other
// callees fail closed. dataclasses.asdict accepted (leaf name asdict); vars is
// bare builtin only.
func pythonDictViewKeyAccessType(n *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "subscript":
		val := ingest.ChildByField(n, "value")
		if val == nil {
			val = ingest.ChildByField(n, "object")
		}
		keyN := ingest.ChildByField(n, "subscript")
		if val == nil || keyN == nil || keyN.Type() != "string" {
			return ""
		}
		_, key := pythonStringContent(keyN, content)
		if key == "" {
			return ""
		}
		if t := pythonDictViewKeyFromSource(val, content, fieldOf, funcReturns, typeOf, key); t != "" {
			return t
		}
	case "call":
		// asdict(box).get("a") / vars(box).pop("a") / box.__dict__.setdefault("a")
		// / ba._asdict().get("a")
		fn := ingest.ChildByField(n, "function")
		if fn == nil || fn.Type() != "attribute" {
			return ""
		}
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || obj == nil {
			return ""
		}
		switch ingest.NodeText(attr, content) {
		case "get", "setdefault", "pop":
			args, ok := pythonCallPositionalArgNodes(n)
			if !ok || len(args) == 0 || args[0].Type() != "string" {
				return ""
			}
			_, key := pythonStringContent(args[0], content)
			if key == "" {
				return ""
			}
			if t := pythonDictViewKeyFromSource(obj, content, fieldOf, funcReturns, typeOf, key); t != "" {
				return t
			}
		}
	}
	return ""
}

// pythonDictViewKeyFromSource recovers field key type T from a dict-view source
// expression (asdict(box) / vars(box) / box.__dict__ / ba._asdict() /
// vars(SimpleNamespace(...)) / SimpleNamespace(...).__dict__).
// funcReturns/typeOf peel method-return SNS kwargs under foreign same-leaf.
func pythonDictViewKeyFromSource(src *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string, key string) string {
	if src == nil || key == "" {
		return ""
	}
	// Inline vars(SimpleNamespace(k=A()/ba.get())) / SimpleNamespace(...).__dict__ —
	// field types from kwargs (no local fieldOf needed).
	if t := pythonInlineSimpleNamespaceDictViewFieldType(src, content, key, funcReturns, typeOf, fieldOf); t != "" {
		return t
	}
	if fieldOf == nil {
		return ""
	}
	objLocal := pythonDictViewObjectLocal(src, content)
	if objLocal == "" {
		return ""
	}
	return fieldOf[objLocal+"."+key]
}

// pythonInlineSimpleNamespaceDictViewCall recovers the SimpleNamespace(...) /
// types.SimpleNamespace(...) call from vars(SimpleNamespace(...)) /
// SimpleNamespace(...).__dict__ under foreign same-leaf. Other forms fail closed.
func pythonInlineSimpleNamespaceDictViewCall(src *grammar.Node, content []byte) *grammar.Node {
	if src == nil {
		return nil
	}
	switch src.Type() {
	case "call":
		fn := ingest.ChildByField(src, "function")
		// vars(SimpleNamespace(...)) — bare vars only.
		if fn == nil || fn.Type() != "identifier" || ingest.NodeText(fn, content) != "vars" {
			return nil
		}
		args, ok := pythonCallPositionalArgNodes(src)
		if !ok || len(args) == 0 || args[0].Type() != "call" {
			return nil
		}
		return args[0]
	case "attribute":
		// SimpleNamespace(...).__dict__
		field := ingest.ChildByField(src, "attribute")
		obj := ingest.ChildByField(src, "object")
		if field == nil || obj == nil || ingest.NodeText(field, content) != "__dict__" {
			return nil
		}
		if obj.Type() != "call" {
			return nil
		}
		return obj
	default:
		return nil
	}
}

// pythonInlineSimpleNamespaceDictViewFieldType recovers T from
// vars(SimpleNamespace(k=A())) / vars(types.SimpleNamespace(k=A())) /
// SimpleNamespace(k=A()).__dict__ field key k under foreign same-leaf.
// Keyword fields only; Class() and method-return / typed-local values peel via
// pythonSimpleNamespaceObjectFieldTypes. Mixed / non-object values fail closed.
func pythonInlineSimpleNamespaceDictViewFieldType(src *grammar.Node, content []byte, key string, funcReturns, typeOf, fieldOf map[string]string) string {
	if src == nil || key == "" {
		return ""
	}
	call := pythonInlineSimpleNamespaceDictViewCall(src, content)
	if call == nil {
		return ""
	}
	fields, _ := pythonSimpleNamespaceObjectFieldTypes(call, content, funcReturns, typeOf, fieldOf)
	if fields == nil {
		return ""
	}
	return fields[key]
}

// pythonInlineSimpleNamespaceDictViewHomogeneousType recovers the shared value
// type of vars(SimpleNamespace(k=A())) / SimpleNamespace(...).__dict__ when all
// declaration-order field types agree. Enables for x in vars(SNS(...)).values()
// under foreign same-leaf. Mixed fields fail closed. Class()-only peels (nil maps).
func pythonInlineSimpleNamespaceDictViewHomogeneousType(src *grammar.Node, content []byte) string {
	return pythonInlineSimpleNamespaceDictViewHomogeneousTypeEx(src, content, nil, nil, nil)
}

// pythonInlineSimpleNamespaceDictViewHomogeneousTypeEx peels method-return SNS
// kwargs under foreign same-leaf (vars(SimpleNamespace(k=ba.get())).values()).
func pythonInlineSimpleNamespaceDictViewHomogeneousTypeEx(src *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	call := pythonInlineSimpleNamespaceDictViewCall(src, content)
	if call == nil {
		return ""
	}
	_, order := pythonSimpleNamespaceObjectFieldTypes(call, content, funcReturns, typeOf, fieldOf)
	if len(order) == 0 {
		return ""
	}
	shared := order[0]
	for _, t := range order[1:] {
		if t != shared {
			return ""
		}
	}
	return shared
}

// pythonInlineSimpleNamespaceDictViewFirstFieldType recovers the first
// declaration-order field type of vars(SimpleNamespace(k=A())) /
// SimpleNamespace(...).__dict__. Enables next(iter(vars(SNS(...)).values()))
// under foreign same-leaf (same leaf as next(asdict(box).values()) for field 0).
func pythonInlineSimpleNamespaceDictViewFirstFieldType(src *grammar.Node, content []byte) string {
	call := pythonInlineSimpleNamespaceDictViewCall(src, content)
	if call == nil {
		return ""
	}
	_, order := pythonSimpleNamespaceFieldTypes(call, content)
	if len(order) == 0 {
		return ""
	}
	return order[0]
}

// pythonDictViewObjectLocal recovers the identifier local whose field keys are
// exposed by asdict(x) / dataclasses.asdict(x) / vars(x) / x.__dict__ /
// x._asdict() (namedtuple). Other forms fail closed ("").
func pythonDictViewObjectLocal(n *grammar.Node, content []byte) string {
	if n == nil {
		return ""
	}
	switch n.Type() {
	case "call":
		fn := ingest.ChildByField(n, "function")
		// ba._asdict() — zero-arg namedtuple dict view of instance fields.
		// dataclasses.asdict falls through to leaf-name "asdict" below.
		if fn != nil && fn.Type() == "attribute" {
			attr := ingest.ChildByField(fn, "attribute")
			obj := ingest.ChildByField(fn, "object")
			if attr != nil && obj != nil && obj.Type() == "identifier" &&
				ingest.NodeText(attr, content) == "_asdict" {
				// Require zero-arg call.
				if args, ok := pythonCallPositionalArgNodes(n); ok && len(args) == 0 {
					return ingest.NodeText(obj, content)
				}
				// No positional args field / empty argument_list.
				argList := ingest.ChildByField(n, "arguments")
				if argList == nil {
					return ingest.NodeText(obj, content)
				}
				for i := uint32(0); i < argList.ChildCount(); i++ {
					ch := argList.Child(i)
					if ch == nil || ch.Type() == "(" || ch.Type() == ")" {
						continue
					}
					return ""
				}
				return ingest.NodeText(obj, content)
			}
		}
		name := pythonSimpleCalleeName(fn, content)
		switch name {
		case "asdict":
			// bare asdict / dataclasses.asdict
		case "vars":
			// Bare builtin only (attribute forms are not the builtin).
			if fn == nil || fn.Type() != "identifier" {
				return ""
			}
		default:
			return ""
		}
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) == 0 || args[0].Type() != "identifier" {
			return ""
		}
		return ingest.NodeText(args[0], content)
	case "attribute":
		field := ingest.ChildByField(n, "attribute")
		obj := ingest.ChildByField(n, "object")
		if field == nil || obj == nil || obj.Type() != "identifier" {
			return ""
		}
		if ingest.NodeText(field, content) != "__dict__" {
			return ""
		}
		return ingest.NodeText(obj, content)
	}
	return ""
}

// pythonReplaceFieldAccessType recovers T from replace(box).a /
// dataclasses.replace(box).a / replace(box, a=A()).a / replace(box, a=ba.get()).a
// when box is a typed local with annotated field a of type T (fieldOf; same leaf
// as box.a), or when a keyword override peels to Class() or zero-arg method
// return (override wins — same leaf as ba._replace(a=A()).a /
// ba._replace(a=ba.get()).a). First positional must be an identifier local for
// fieldOf fallback; keyword-only object / non-replace callees fail closed.
func pythonReplaceFieldAccessType(attr *grammar.Node, content []byte, fieldOf, funcReturns, typeOf map[string]string) string {
	if attr == nil || attr.Type() != "attribute" {
		return ""
	}
	obj := ingest.ChildByField(attr, "object")
	field := ingest.ChildByField(attr, "attribute")
	if obj == nil || field == nil || obj.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(obj, "function")
	if fn == nil || pythonSimpleCalleeName(fn, content) != "replace" {
		return ""
	}
	fname := ingest.NodeText(field, content)
	if fname == "" {
		return ""
	}
	// Keyword override: replace(box, a=A()).a / replace(box, a=ba.get()).a → A
	// (same as namedtuple _replace; method-return under foreign same-leaf).
	argList := ingest.ChildByField(obj, "arguments")
	if argList != nil {
		for i := uint32(0); i < argList.ChildCount(); i++ {
			ch := argList.Child(i)
			if ch == nil || ch.Type() != "keyword_argument" {
				continue
			}
			nameN := ingest.ChildByField(ch, "name")
			valN := ingest.ChildByField(ch, "value")
			if nameN == nil || ingest.NodeText(nameN, content) != fname || valN == nil {
				continue
			}
			if et := pythonExprClassType(valN, content); et != "" {
				return et
			}
			if et := pythonObjectLeafType(valN, content, funcReturns, typeOf, fieldOf); et != "" {
				return et
			}
		}
	}
	if fieldOf == nil {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(obj)
	if !ok || len(args) == 0 || args[0].Type() != "identifier" {
		return ""
	}
	return fieldOf[ingest.NodeText(args[0], content)+"."+fname]
}

// pythonRecordKeyAccessType recovers T from box["a"] / box.get("a") /
// box.pop("a") / box.setdefault("a"[, default]) when box is a typed local with
// annotated field a of type T (TypedDict / dataclass-style string keys via
// fieldOf). Only identifier receivers and string-literal keys; non-string keys,
// multi-arg pop without a string first arg, and unknown fields fail closed.
// Homogeneous dict value typing stays on the elemOf path.
func pythonRecordKeyAccessType(n *grammar.Node, content []byte, fieldOf map[string]string) string {
	if n == nil || fieldOf == nil {
		return ""
	}
	switch n.Type() {
	case "subscript":
		val := ingest.ChildByField(n, "value")
		if val == nil {
			val = ingest.ChildByField(n, "object")
		}
		sub := ingest.ChildByField(n, "subscript")
		if val == nil || val.Type() != "identifier" || sub == nil || sub.Type() != "string" {
			return ""
		}
		_, key := pythonStringContent(sub, content)
		if key == "" {
			return ""
		}
		return fieldOf[ingest.NodeText(val, content)+"."+key]
	case "call":
		fn := ingest.ChildByField(n, "function")
		if fn == nil || fn.Type() != "attribute" {
			return ""
		}
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || obj == nil || obj.Type() != "identifier" {
			return ""
		}
		switch ingest.NodeText(attr, content) {
		case "get", "setdefault", "pop":
			// First positional arg must be a string key; default/other args ignored.
			args, ok := pythonCallPositionalArgNodes(n)
			if !ok || len(args) == 0 || args[0].Type() != "string" {
				return ""
			}
			_, key := pythonStringContent(args[0], content)
			if key == "" {
				return ""
			}
			return fieldOf[ingest.NodeText(obj, content)+"."+key]
		}
	}
	return ""
}

// pythonBindClassPatternKeywordCaptures binds value captures from
// `case Box(a=xa, b=xb):` keyword_patterns and `case Box(xa, xb):` positional
// patterns using annotated fields of Box (fieldIndex). Positionals map by
// declaration order (fieldOrder / namedtuple field names). Unknown fields and
// excess positionals fail closed.
// Grammar often wraps `a=xa as x` as as_pattern(keyword_pattern(a=xa), x)
// rather than keyword_pattern(a=(xa as x)); both alias and inner capture bind.
func pythonBindClassPatternKeywordCaptures(n *grammar.Node, content []byte, fieldIndex map[string]map[string]string, fieldOrder map[string][]string, ourReceivers, out map[string]bool) {
	if n == nil || n.IsNull() || n.Type() != "class_pattern" || fieldIndex == nil {
		return
	}
	typeName := pythonClassPatternTypeName(n, content)
	if typeName == "" {
		return
	}
	fields := fieldIndex[typeName]
	if len(fields) == 0 {
		return
	}
	var order []string
	if fieldOrder != nil {
		order = fieldOrder[typeName]
	}
	pos := 0
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch.Type() != "case_pattern" {
			continue
		}
		payload := pythonMatchPatternInner(ch)
		if payload == nil {
			continue
		}
		// Optional outer as: case Box(a=xa as x) / Box(xa as x) → as_pattern(..., x).
		var outerAlias string
		if payload.Type() == "as_pattern" {
			var left *grammar.Node
			seenAs := false
			for j := uint32(0); j < payload.ChildCount(); j++ {
				c := payload.Child(j)
				if c.Type() == "as" {
					seenAs = true
					continue
				}
				if !seenAs {
					left = c
					continue
				}
				switch c.Type() {
				case "identifier":
					outerAlias = ingest.NodeText(c, content)
				case "as_pattern_target":
					if id := ingest.ChildByType(c, "identifier"); id != nil {
						outerAlias = ingest.NodeText(id, content)
					}
				}
			}
			if left == nil {
				continue
			}
			// Unwrap case_pattern wrapper on the left if present.
			if left.Type() == "case_pattern" || left.Type() == "pattern" {
				left = pythonMatchPatternInner(left)
			}
			if left == nil {
				continue
			}
			payload = left
		}
		if payload.Type() != "keyword_pattern" {
			// Positional capture: ith non-keyword pattern → order[i] field type.
			if pos < len(order) {
				key := order[pos]
				ft := fields[key]
				if ft != "" && ourReceivers[ft] {
					pythonBindMatchMapValueCaptures(payload, content, ft, ourReceivers, out)
					if outerAlias != "" {
						out[outerAlias] = true
					}
				}
			}
			pos++
			continue
		}
		// keyword_pattern: <field ident> = <value pattern>
		var key string
		var valuePat *grammar.Node
		for j := uint32(0); j < payload.ChildCount(); j++ {
			c := payload.Child(j)
			if c.Type() == "=" {
				continue
			}
			if key == "" && c.Type() == "identifier" {
				key = ingest.NodeText(c, content)
				continue
			}
			valuePat = c
		}
		if key == "" {
			continue
		}
		ft := fields[key]
		if ft == "" || !ourReceivers[ft] {
			continue
		}
		if valuePat != nil {
			pythonBindMatchMapValueCaptures(valuePat, content, ft, ourReceivers, out)
		}
		if outerAlias != "" {
			out[outerAlias] = true
		}
	}
}

// pythonBindMatchRecordKeyPatterns binds capture names from match dict_pattern
// value slots when the subject is a TypedDict/record local with key-specific
// field types in fieldOf (`match box: case {"a": xa}:` → xa typed as field a).
// Only string-literal keys; capture keys (`{k: a}`), non-string keys, unknown
// fields, and **rest fail closed. Reuses pythonBindMatchMapValueCaptures per key.
func pythonBindMatchRecordKeyPatterns(n *grammar.Node, content []byte, local string, fieldOf map[string]string, ourReceivers, out map[string]bool) {
	if n == nil || n.IsNull() || local == "" || fieldOf == nil {
		return
	}
	switch n.Type() {
	case "dict_pattern":
		// Children alternate field=key / field=value (string key + case_pattern value).
		// Multi-pair patterns have multiple key/value field children; pair by walk order.
		var pendingKey string
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			switch n.FieldNameForChild(i) {
			case "key":
				pendingKey = ""
				if ch.Type() == "string" {
					_, pendingKey = pythonStringContent(ch, content)
				}
				// Capture keys / non-string keys fail closed (no field leaf).
			case "value":
				if pendingKey != "" {
					if ft := fieldOf[local+"."+pendingKey]; ft != "" {
						pythonBindMatchMapValueCaptures(ch, content, ft, ourReceivers, out)
					}
				}
				pendingKey = ""
			}
		}
		return
	case "case_pattern", "pattern", "as_pattern":
		// Unwrap; for as_pattern only the left pattern can contain a dict_pattern.
		// (Whole-match alias typing is not record-key specific.)
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() == "as" || ch.Type() == "as_pattern_target" {
				continue
			}
			pythonBindMatchRecordKeyPatterns(ch, content, local, fieldOf, ourReceivers, out)
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		pythonBindMatchRecordKeyPatterns(n.Child(i), content, local, fieldOf, ourReceivers, out)
	}
}

// pythonBindMatchSeqPatterns binds capture names from match list/tuple/mapping
// patterns when the match subject has element/value type et and optional nest
// leaf (list[list[A]] / dict[str, list[A]] → nest "A").
// Sequence: fixed slots → out (if ourReceivers[et]); *rest → elemOf (including
// foreign, for shadowing). When nest is set and et is not a receiver (container
// name like "list"), simple fixed slots bind as list-of-nest (elemOf[row]=nest)
// so row[0].run() peels; nested list/tuple slots bind captures as nest leaf.
// Mapping: value captures in case {"k": a} → out; nested list value patterns
// case {"k": [xa, *_]} bind xa as nest. **rest fails closed (mapping, not an
// element). Class patterns inside the sequence/value fail closed (other binders).
// TypedDict key-specific captures use pythonBindMatchRecordKeyPatterns instead.
func pythonBindMatchSeqPatterns(n *grammar.Node, content []byte, et, nest string, ourReceivers, out map[string]bool, elemOf map[string]string) {
	if n == nil || n.IsNull() {
		return
	}
	if et == "" && nest == "" {
		return
	}
	switch n.Type() {
	case "list_pattern", "tuple_pattern":
		fixed, star, ok := pythonMatchSeqPatternCaptures(n, content)
		if ok {
			for _, name := range fixed {
				if et != "" && ourReceivers[et] {
					out[name] = true
				} else if nest != "" && elemOf != nil {
					// case [row, *_] on list[list[A]] — row is list of nest.
					elemOf[name] = nest
				}
			}
			if star != "" && elemOf != nil {
				if et != "" && ourReceivers[et] {
					// Foreign element types too — shadow prior same-name collections.
					elemOf[star] = et
				} else if nest != "" {
					// *rest on nested collection: rows are list-of-nest.
					// for row in rest peels via @nested (same as for row in aa).
					elemOf["@nested."+star] = nest
				} else if et != "" {
					elemOf[star] = et
				}
			}
			return
		}
		// Nested sequence slots: case [[xa, *_], *_] when nest is the leaf T.
		if nest == "" {
			return
		}
		pythonBindMatchNestedSeqSlots(n, content, nest, ourReceivers, out, elemOf)
		return
	case "dict_pattern":
		// Mapping values are case_pattern/pattern children; keys are string /
		// integer / dotted_name (capture keys) and are not value-typed.
		// **rest (splat_pattern) is a mapping — fail closed.
		// Nested list values: case {"k": [xa, *_]} when nest is leaf T.
		// Nested map values: case {"outer": {"k": xa}} when nest is leaf T
		// (dict-of-dict / OrderedDict(outer=OrderedDict(k=A()))).
		// Whole list value captures: case {"k": row} / {"k": row as r} when nest
		// is leaf T — row is list of nest (elemOf; same as case [row] on list[list[A]]).
		for i := uint32(0); i < n.ChildCount(); i++ {
			ch := n.Child(i)
			if ch.Type() != "case_pattern" && ch.Type() != "pattern" {
				continue
			}
			if nest != "" && pythonMatchPatternIsSeq(ch) {
				// Value is list/tuple of nest — bind inner captures as nest leaf.
				pythonBindMatchSeqPatterns(ch, content, nest, "", ourReceivers, out, elemOf)
				continue
			}
			if nest != "" && pythonMatchPatternIsDict(ch) {
				// Value is mapping of nest leaf — case {"outer": {"k": xa}}.
				pythonBindMatchSeqPatterns(ch, content, nest, "", ourReceivers, out, elemOf)
				continue
			}
			if nest != "" {
				// case {"k": row} on dict[str, list[A]] / da={"k":[A()]} — row is
				// list of nest (not nest leaf itself). Foreign too for shadowing.
				// Also case {"outer": inner} on dict-of-dict — inner is mapping of nest.
				pythonBindMatchNestedMapValueListCaptures(ch, content, nest, elemOf)
				continue
			}
			if et != "" {
				pythonBindMatchMapValueCaptures(ch, content, et, ourReceivers, out)
			}
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
			pythonBindMatchSeqPatterns(ch, content, et, nest, ourReceivers, out, elemOf)
		}
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		pythonBindMatchSeqPatterns(n.Child(i), content, et, nest, ourReceivers, out, elemOf)
	}
}

// pythonMatchPatternIsSeq reports whether n (possibly wrapped) is a list/tuple
// pattern. Used for nested mapping value peels: case {"k": [xa, *_]}.
func pythonMatchPatternIsSeq(n *grammar.Node) bool {
	if n == nil || n.IsNull() {
		return false
	}
	for n != nil && (n.Type() == "case_pattern" || n.Type() == "pattern") {
		inner := pythonMatchPatternInner(n)
		if inner == nil {
			return false
		}
		n = inner
	}
	if n == nil {
		return false
	}
	return n.Type() == "list_pattern" || n.Type() == "tuple_pattern"
}

// pythonMatchPatternIsDict reports whether n (possibly wrapped) is a dict
// pattern. Used for nested mapping-of-mapping peels: case {"outer": {"k": xa}}.
func pythonMatchPatternIsDict(n *grammar.Node) bool {
	if n == nil || n.IsNull() {
		return false
	}
	for n != nil && (n.Type() == "case_pattern" || n.Type() == "pattern") {
		inner := pythonMatchPatternInner(n)
		if inner == nil {
			return false
		}
		n = inner
	}
	if n == nil {
		return false
	}
	return n.Type() == "dict_pattern"
}

// pythonBindClassPatternSubjectFields binds keyword (and ordered index) captures
// from a class_pattern using instance fieldOf[subj.field] of the match subject:
//
//	da = SimpleNamespace(k=A()); match da: case SimpleNamespace(k=xa): xa.run()
//	ba = Box(A()); match ba: case Box(a=xa): / case Box(xa):
//
// Type-level fieldIndex alone is last-writer-wins under dual-class same-field
// names. Recurses into nested patterns. Unknown fields / missing fieldOf fail closed.
func pythonBindClassPatternSubjectFields(n *grammar.Node, content []byte, subjLocal string, fieldOf map[string]string, ourReceivers, out map[string]bool) {
	if n == nil || n.IsNull() || subjLocal == "" || fieldOf == nil {
		return
	}
	if n.Type() == "class_pattern" {
		pythonBindClassPatternSubjectFieldsOne(n, content, subjLocal, fieldOf, ourReceivers, out)
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		pythonBindClassPatternSubjectFields(n.Child(i), content, subjLocal, fieldOf, ourReceivers, out)
	}
}

func pythonBindClassPatternSubjectFieldsOne(n *grammar.Node, content []byte, subjLocal string, fieldOf map[string]string, ourReceivers, out map[string]bool) {
	if n == nil || n.Type() != "class_pattern" {
		return
	}
	pos := 0
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch.Type() != "case_pattern" {
			continue
		}
		payload := pythonMatchPatternInner(ch)
		if payload == nil {
			continue
		}
		var outerAlias string
		if payload.Type() == "as_pattern" {
			var left *grammar.Node
			seenAs := false
			for j := uint32(0); j < payload.ChildCount(); j++ {
				c := payload.Child(j)
				if c.Type() == "as" {
					seenAs = true
					continue
				}
				if !seenAs {
					left = c
					continue
				}
				switch c.Type() {
				case "identifier":
					outerAlias = ingest.NodeText(c, content)
				case "as_pattern_target":
					if id := ingest.ChildByType(c, "identifier"); id != nil {
						outerAlias = ingest.NodeText(id, content)
					}
				}
			}
			if left == nil {
				continue
			}
			if left.Type() == "case_pattern" || left.Type() == "pattern" {
				left = pythonMatchPatternInner(left)
			}
			if left == nil {
				continue
			}
			payload = left
		}
		if payload.Type() != "keyword_pattern" {
			// Positional: ith non-keyword → fieldOf[subj.#i] (namedtuple order).
			ft := fieldOf[subjLocal+".#"+fmt.Sprintf("%d", pos)]
			if ft != "" && ourReceivers[ft] {
				pythonBindMatchMapValueCaptures(payload, content, ft, ourReceivers, out)
				if outerAlias != "" {
					out[outerAlias] = true
				}
			}
			pos++
			continue
		}
		// keyword_pattern: <field ident> = <value pattern>
		var key string
		var valuePat *grammar.Node
		for j := uint32(0); j < payload.ChildCount(); j++ {
			c := payload.Child(j)
			if c.Type() == "=" {
				continue
			}
			if key == "" && c.Type() == "identifier" {
				key = ingest.NodeText(c, content)
				continue
			}
			valuePat = c
		}
		if key == "" {
			continue
		}
		ft := fieldOf[subjLocal+"."+key]
		if ft == "" || !ourReceivers[ft] {
			continue
		}
		if valuePat != nil {
			pythonBindMatchMapValueCaptures(valuePat, content, ft, ourReceivers, out)
		}
		if outerAlias != "" {
			out[outerAlias] = true
		}
	}
}

// pythonBindMatchNestedSeqSlots walks a list/tuple pattern whose flat capture
// parse failed (nested list/class slots). Simple identifier slots bind as
// list-of-nest (elemOf); nested list/tuple slots bind captures as nest leaf.
// Wildcards / *_ skip; class/or/value patterns fail closed for that slot.
func pythonBindMatchNestedSeqSlots(n *grammar.Node, content []byte, nest string, ourReceivers, out map[string]bool, elemOf map[string]string) {
	if n == nil || nest == "" {
		return
	}
	switch n.Type() {
	case "list_pattern", "tuple_pattern":
		// ok
	default:
		return
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "(", ")", "[", "]", ",", "comment", "_":
			continue
		case "splat_pattern":
			// *rest / *_ on outer nested collection.
			if id := ingest.ChildByType(ch, "identifier"); id != nil && elemOf != nil {
				elemOf["@nested."+ingest.NodeText(id, content)] = nest
			}
			continue
		case "case_pattern", "pattern":
			inner := pythonMatchPatternInner(ch)
			if inner == nil {
				continue
			}
			if inner.Type() == "_" {
				continue
			}
			// Nested *rest wrapped in case_pattern.
			if inner.Type() == "splat_pattern" {
				if id := ingest.ChildByType(inner, "identifier"); id != nil && elemOf != nil {
					elemOf["@nested."+ingest.NodeText(id, content)] = nest
				}
				continue
			}
			switch inner.Type() {
			case "list_pattern", "tuple_pattern":
				// case [[xa, *_], …] — inner sequence of nest leaf.
				pythonBindMatchSeqPatterns(inner, content, nest, "", ourReceivers, out, elemOf)
			case "identifier", "dotted_name":
				name := pythonMatchCaptureName(inner, content)
				if name != "" && elemOf != nil {
					elemOf[name] = nest
				}
			case "as_pattern":
				// row as r / [xa, *_] as row — bind left + alias as list-of-nest
				// or recurse into nested sequence left.
				var left *grammar.Node
				var alias string
				seenAs := false
				for j := uint32(0); j < inner.ChildCount(); j++ {
					c := inner.Child(j)
					if c.Type() == "as" {
						seenAs = true
						continue
					}
					if !seenAs {
						left = c
						continue
					}
					switch c.Type() {
					case "identifier":
						alias = ingest.NodeText(c, content)
					case "as_pattern_target":
						if id := ingest.ChildByType(c, "identifier"); id != nil {
							alias = ingest.NodeText(id, content)
						}
					}
				}
				if left != nil {
					for left != nil && (left.Type() == "case_pattern" || left.Type() == "pattern") {
						innerL := pythonMatchPatternInner(left)
						if innerL == nil {
							break
						}
						left = innerL
					}
				}
				if left != nil {
					switch left.Type() {
					case "list_pattern", "tuple_pattern":
						pythonBindMatchSeqPatterns(left, content, nest, "", ourReceivers, out, elemOf)
						// Whole nested sequence alias is list-of-nest, not leaf.
						if alias != "" && elemOf != nil {
							elemOf[alias] = nest
						}
					case "identifier", "dotted_name":
						name := pythonMatchCaptureName(left, content)
						if elemOf != nil {
							if name != "" {
								elemOf[name] = nest
							}
							if alias != "" {
								elemOf[alias] = nest
							}
						}
					}
				}
			default:
				// class/or/value — fail closed for this slot.
			}
		case "identifier", "dotted_name":
			name := pythonMatchCaptureName(ch, content)
			if name != "" && elemOf != nil {
				elemOf[name] = nest
			}
		default:
			// Unknown slot shape — fail closed for this slot only.
		}
	}
}

// pythonBindMatchNestedMapValueListCaptures binds simple captures (and capture-as
// aliases) from a mapping value pattern when the subject is a mapping of list/set
// of nest leaf T (elemOf["@nested."+subj] = nest). Captures are list-of-nest
// (elemOf[row]=nest), not nest leaf — enables case {"k": row}: row[0].run() /
// case {"k": row as r}: r[0].run() under foreign same-leaf (same leaf as
// case [row] on list[list[A]]). Nested class/list/or patterns fail closed here
// (list values use pythonBindMatchSeqPatterns; class patterns stay on as_pattern).
func pythonBindMatchNestedMapValueListCaptures(n *grammar.Node, content []byte, nest string, elemOf map[string]string) {
	if n == nil || n.IsNull() || nest == "" || elemOf == nil {
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
		if name != "" {
			elemOf[name] = nest
		}
	case "as_pattern":
		// `row as r` inside a mapping value: both bind as list-of-nest (PEP 634).
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
		if alias != "" {
			elemOf[alias] = nest
		}
		if left != nil {
			for left != nil && (left.Type() == "case_pattern" || left.Type() == "pattern") {
				inner := pythonMatchPatternInner(left)
				if inner == nil {
					break
				}
				left = inner
			}
		}
		if left != nil {
			switch left.Type() {
			case "identifier", "dotted_name":
				name := pythonMatchCaptureName(left, content)
				if name != "" {
					elemOf[name] = nest
				}
			default:
				// Nested class/list/or left of `as` — fail closed for left only.
			}
		}
	default:
		// Nested class/list/or/value — fail closed (list handled above).
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
// `case (a, b):` / `case [a, *_]:` / `case [_, a]:`). Match grammar wraps
// captures in case_pattern and uses splat_pattern (not list_splat_pattern).
// Non-binding wildcards `_` and `*_` are skipped (do not fail closed). Nested
// class/or/value patterns still fail closed.
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
	sawStar := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "(", ")", "[", "]", ",", "comment":
			continue
		case "_":
			// Bare wildcard slot (some grammar shapes): non-binding.
			continue
		case "case_pattern", "pattern":
			inner := pythonMatchPatternInner(ch)
			if inner == nil {
				return nil, "", false
			}
			// Wildcard `_` is non-binding: case [_, a] still types `a`.
			if inner.Type() == "_" {
				continue
			}
			// `a as x` inside a sequence slot: both bind (PEP 634). Nested
			// left patterns (class/list) fail closed here — nested list slots
			// use pythonBindMatchNestedSeqSlots instead.
			if inner.Type() == "as_pattern" {
				leftName, alias, okAs := pythonMatchAsCaptureNames(inner, content)
				if !okAs {
					return nil, "", false
				}
				if leftName != "" {
					fixed = append(fixed, leftName)
				}
				if alias != "" {
					fixed = append(fixed, alias)
				}
				continue
			}
			name, isStar, okCap := pythonMatchCaptureOrStar(inner, content)
			if !okCap {
				return nil, "", false
			}
			if isStar {
				if sawStar {
					return nil, "", false
				}
				sawStar = true
				// name may be "" for non-binding `*_`.
				star = name
			} else {
				fixed = append(fixed, name)
			}
		case "splat_pattern":
			// Bare *rest / *_ child (some grammar shapes).
			if sawStar {
				return nil, "", false
			}
			sawStar = true
			id := ingest.ChildByType(ch, "identifier")
			if id != nil {
				star = ingest.NodeText(id, content)
				break
			}
			// `*_` — non-binding star rest (star stays empty).
			if !pythonSplatPatternIsWildcard(ch) {
				return nil, "", false
			}
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

// pythonMatchAsCaptureNames returns left capture + alias from `a as x` when the
// left side is a simple identifier capture (not class/list/or). Used for
// sequence slots `case [a as x]:` / nested map values `case {"k": [xa as x, *_]}`.
func pythonMatchAsCaptureNames(n *grammar.Node, content []byte) (left, alias string, ok bool) {
	if n == nil || n.Type() != "as_pattern" {
		return "", "", false
	}
	var leftN *grammar.Node
	seenAs := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch.Type() == "as" {
			seenAs = true
			continue
		}
		if !seenAs {
			leftN = ch
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
	if leftN != nil {
		for leftN != nil && (leftN.Type() == "case_pattern" || leftN.Type() == "pattern") {
			inner := pythonMatchPatternInner(leftN)
			if inner == nil {
				return "", "", false
			}
			leftN = inner
		}
	}
	if leftN == nil {
		return "", "", false
	}
	switch leftN.Type() {
	case "identifier", "dotted_name":
		left = pythonMatchCaptureName(leftN, content)
		if left == "" {
			return "", "", false
		}
	case "_":
		// `_ as x` — only alias binds.
	default:
		// Nested class/list/or left — fail closed for flat seq capture parse.
		return "", "", false
	}
	if left == "" && alias == "" {
		return "", "", false
	}
	return left, alias, true
}

// pythonMatchCaptureOrStar returns a simple capture name, or *rest via splat_pattern.
// Non-binding `*_` returns ok with empty name and isStar=true (caller skips binding).
func pythonMatchCaptureOrStar(n *grammar.Node, content []byte) (name string, isStar bool, ok bool) {
	if n == nil {
		return "", false, false
	}
	switch n.Type() {
	case "splat_pattern":
		id := ingest.ChildByType(n, "identifier")
		if id == nil {
			// `*_` — non-binding star rest (grammar: splat_pattern with `_` child).
			if pythonSplatPatternIsWildcard(n) {
				return "", true, true
			}
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
		// (Bare `_` is handled by pythonMatchSeqPatternCaptures before calling here.)
		return "", false, false
	}
}

// pythonSplatPatternIsWildcard reports whether splat_pattern is non-binding `*_`
// (has a `_` child and no identifier capture).
func pythonSplatPatternIsWildcard(n *grammar.Node) bool {
	if n == nil || n.Type() != "splat_pattern" {
		return false
	}
	if ingest.ChildByType(n, "identifier") != nil {
		return false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		if n.Child(i).Type() == "_" {
			return true
		}
	}
	return false
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
// items: list[A]; next(x for x in items) → A for identity genexps). Used for both
// assignment (`a = next(...)`) and direct chains (`next(...).run()`). Fails closed
// on splat args or empty call. Default arg is ignored (result may be union with
// default at runtime; we still bind the element type). Heterogeneous astuple field
// tuples are handled separately by pythonAstupleNextFirstField (first field only;
// not shared with choice/min which are not first-element).
func pythonNextElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonAstupleNextFirstField recovers the first declaration-order field type from
// next(astuple(box)) / next(iter(astuple(box))) / next(list(astuple(box))) /
// next(tuple(astuple(box))) / dataclasses.astuple / astuple(replace(box)) forms,
// and next(asdict(box).values()) / next(iter(asdict(box).values())) /
// vars(box).values() / box.__dict__.values() (dict preserves declaration order;
// first value is field 0 — same leaf as next(astuple(box))),
// plus assigned dict-view locals: d = asdict(box); next(d.values()) /
// d = vars(box); next(iter(d.values())) / d = box.__dict__; next(d.values())
// (bindFields on d records @astuple.d.#i — same leaf as next(asdict(box).values())).
// next always yields the first tuple/dict-view element — same leaf as astuple(box)[0].
// Peels identity order-preserving wrappers iter/list/tuple only (not reversed).
// Fails closed when the iterable is not an astuple/dict-view chain or field 0 is unknown.
// Intentionally not used for choice/min (not first-element semantics).
func pythonAstupleNextFirstField(call *grammar.Node, content []byte, fieldOf map[string]string) string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonAstupleFirstFieldType(args[0], content, fieldOf)
}

// pythonAstupleFirstFieldType recovers the first declaration-order field type
// from an expression that is:
//   - astuple(local) / dataclasses.astuple(local) / astuple(replace(local))
//     → fieldOf["@astuple.local.#0"]
//   - t / list(t) / iter(t) when t = astuple(box) (or list/tuple(astuple(box)))
//     → fieldOf["t.#0"] (index slots bound on assignment)
//   - asdict(local).values() / vars(local).values() / local.__dict__.values()
//     → fieldOf["@astuple.local.#0"] (dict preserves declaration order; first
//     value is field 0 — same leaf as next(astuple(local)) / local.a)
//   - d.values() when d = asdict(box) / vars(box) / box.__dict__ (or walrus)
//     → fieldOf["@astuple.d.#0"] (bindFields on the assigned dict-view local)
//   - order-preserving wrappers of those (iter/list/tuple)
//
// Same leaf as astuple(local)[0] / t[0] / box.a for first field.
func pythonAstupleFirstFieldType(n *grammar.Node, content []byte, fieldOf map[string]string) string {
	if n == nil || fieldOf == nil {
		return ""
	}
	if n.Type() == "parenthesized_expression" {
		return pythonAstupleFirstFieldType(pythonParenInner(n), content, fieldOf)
	}
	// t = astuple(box); next(t) / next(iter(t)) — assigned field-tuple local
	// with index slots fieldOf["t.#0"] (also list(astuple)/tuple(astuple) assigns).
	if n.Type() == "identifier" {
		return fieldOf[ingest.NodeText(n, content)+".#0"]
	}
	if n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil {
		return ""
	}
	// Peel identity wrappers that preserve declaration order: iter / list / tuple.
	// reversed would yield last-first and fails closed here.
	name := pythonSimpleCalleeName(fn, content)
	switch name {
	case "iter", "list", "tuple":
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) == 0 {
			return ""
		}
		return pythonAstupleFirstFieldType(args[0], content, fieldOf)
	case "values":
		// asdict(box).values() / vars(box).values() / box.__dict__.values() —
		// zero-arg values view; first yield is declaration-order field 0.
		// d = asdict(box); d.values() / d = vars(box); d.values() /
		// d = box.__dict__; d.values() — assigned dict-view local (bindFields
		// recorded @astuple.d.#i; same leaf as next(asdict(box).values())).
		// vars(SimpleNamespace(k=A())).values() / SimpleNamespace(...).__dict__.values()
		// — inline SNS (no fieldOf local; first kw field).
		if fn.Type() != "attribute" {
			return ""
		}
		args, ok := pythonCallPositionalArgNodes(n)
		if !ok || len(args) != 0 {
			return ""
		}
		obj := ingest.ChildByField(fn, "object")
		if obj == nil {
			return ""
		}
		// Assigned dict-view local: d.values() after d = asdict(box) / vars / __dict__.
		// bindFields(d, Box) records @astuple.d.#0 (not d.#0 — dicts stay non-indexable).
		if obj.Type() == "identifier" {
			return fieldOf["@astuple."+ingest.NodeText(obj, content)+".#0"]
		}
		local := pythonDictViewObjectLocal(obj, content)
		if local != "" {
			if t := fieldOf["@astuple."+local+".#0"]; t != "" {
				return t
			}
		}
		// Inline vars(SimpleNamespace(k=A())).values() — first declaration-order field.
		return pythonInlineSimpleNamespaceDictViewFirstFieldType(obj, content)
	}
	// astuple(box) / dataclasses.astuple(box) / list(astuple(box)) via ObjectLocal.
	local := pythonAstupleObjectLocal(n, content)
	if local == "" {
		return ""
	}
	return fieldOf["@astuple."+local+".#0"]
}

// pythonMinMaxElemType recovers the element type of min(iterable) / max(iterable)
// (optional key=/default= kwargs ignored). Only the single-positional-arg form is
// handled — min(a, b) / max(x, y, z) compare discrete values and fail closed.
// Also covers min/max(asdict(box).values()) / min/max(astuple(box)) when all
// declaration-order field types agree (homogeneous values; mixed fail closed —
// same leaf as for x in asdict(...).values() / for x in astuple(...)).
func pythonMinMaxElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf, fieldOf map[string]string) string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	if et := pythonIterableElemType(args[0], content, elemOf, egElems, typeOf); et != "" {
		return et
	}
	// min/max(asdict(box).values() / vars / __dict__ / d.values() after d=asdict) —
	// only when all field types agree (homogeneous values view).
	if et := pythonDictViewValuesHomogeneousType(args[0], content, fieldOf); et != "" {
		return et
	}
	// min/max(astuple(box) / list(astuple(box)) / dataclasses.astuple) — same gate.
	if et := pythonAstupleHomogeneousType(args[0], content, fieldOf); et != "" {
		return et
	}
	return ""
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

// pythonGetitemElemType recovers the element type of getitem(collection, key) /
// operator.getitem(collection, key). Same leaf as collection[key] / d[k].
// Bare getitem (from operator import getitem) and module-qualified
// operator.getitem are accepted; other receivers fail closed. Requires at least
// two positional args (collection, key); key is ignored for typing except when
// the collection is a dict-view of inline SNS (string key peels field type).
func pythonGetitemElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "getitem" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "getitem" {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "operator" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 2 {
		return ""
	}
	// getitem(vars(SimpleNamespace(k=A())), "k") — same leaf as vars(SNS)["k"].
	// Class()-only peels here (nil maps); method-return SNS uses object getitem path.
	if args[1].Type() == "string" {
		if _, key := pythonStringContent(args[1], content); key != "" {
			if t := pythonInlineSimpleNamespaceDictViewFieldType(args[0], content, key, nil, typeOf, nil); t != "" {
				return t
			}
		}
	}
	// getitem(collection, key) — element/value type of the collection.
	return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonGetitemObjectElemType recovers T from getitem([ba.get()], i) /
// operator.getitem([ba.get()], i) when the collection peels as an object
// iterable under foreign same-leaf (Class()-only peels via pythonGetitemElemType).
func pythonGetitemObjectElemType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "getitem" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "getitem" {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "operator" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) < 1 {
		return ""
	}
	return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonItemgetterElemType recovers the element type of
// itemgetter(i)(collection) / operator.itemgetter(i)(collection).
// Single-index itemgetter applied to a known collection yields one element
// (same as collection[i]). Multi-index itemgetter(i, j, ...) returns a tuple
// and fails closed. Bare itemgetter (from operator import itemgetter) and
// module-qualified operator.itemgetter are accepted; other receivers fail closed.
// Stored getters (g = itemgetter(0); a = g(items)) use pythonStoredOperatorGetterType.
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

// pythonItemgetterObjectElemType recovers T from itemgetter(i)([ba.get()]) /
// operator.itemgetter(i)([ba.get()]) / itemgetter("k")({"k": ba.get()}) when the
// collection peels as an object iterable or homogeneous object-dict under foreign
// same-leaf (Class()-only peels via pythonItemgetterElemType).
func pythonItemgetterObjectElemType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
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
	idxArgs, ok := pythonCallPositionalArgNodes(fn)
	if !ok || len(idxArgs) != 1 {
		return ""
	}
	collArgs, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(collArgs) != 1 {
		return ""
	}
	if et := pythonObjectIterableElemType(collArgs[0], content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// itemgetter("k")({"k": ba.get()}) — homogeneous object-dict values (same
	// leaf as Class dict peels via pythonDictStarCopyValueType pair Class()).
	return pythonHomogeneousObjectDictValue(collArgs[0], content, funcReturns, typeOf, fieldOf)
}

// pythonPartialBoundMethodReturnType recovers T from partial(ba.get) /
// functools.partial(ba.get) when ba is a typed local and get is a known instance
// method return (funcReturns). Used by partial(ba.get)().run() and
// pa = partial(ba.get); pa().run() under foreign same-leaf. Extra partial args
// fail closed; Class partial peels stay on pythonPartialFactoryClassType.
func pythonPartialBoundMethodReturnType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" || funcReturns == nil {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "partial" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr == nil || ingest.NodeText(attr, content) != "partial" {
			return ""
		}
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "functools" {
			return ""
		}
	default:
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	// Sole bound-method arg: partial(ba.get) — no pre-bound call args.
	if !ok || len(args) != 1 || args[0].Type() != "attribute" {
		return ""
	}
	// Reconstruct ba.get() return type (attribute is the bound method, not a call).
	// Reuse pythonCallFuncReturnType by wrapping as a zero-arg call shape is hard;
	// peel attribute receiver + method name against typeOf / funcReturns.
	attrN := args[0]
	obj := ingest.ChildByField(attrN, "object")
	attr := ingest.ChildByField(attrN, "attribute")
	if obj == nil || attr == nil || attr.Type() != "identifier" {
		return ""
	}
	mname := ingest.NodeText(attr, content)
	if mname == "" {
		return ""
	}
	for obj != nil && !obj.IsNull() {
		switch obj.Type() {
		case "parenthesized_expression":
			obj = pythonParenInner(obj)
		case "named_expression":
			obj = ingest.ChildByField(obj, "value")
		default:
			goto partialObjPeeled
		}
	}
partialObjPeeled:
	if obj == nil || obj.IsNull() {
		return ""
	}
	if obj.Type() == "identifier" {
		oname := ingest.NodeText(obj, content)
		if rt := funcReturns[oname+"."+mname]; rt != "" {
			return rt
		}
		if typeOf != nil {
			if tn := typeOf[oname]; tn != "" {
				if rt := funcReturns[tn+"."+mname]; rt != "" {
					return rt
				}
			}
		}
	}
	if obj.Type() == "call" {
		if rt := pythonCallFuncReturnType(obj, content, funcReturns, typeOf, fieldOf); rt != "" {
			if t := funcReturns[rt+"."+mname]; t != "" {
				return t
			}
		}
	}
	return ""
}

// pythonPartialBoundMethodCallResultType recovers T from partial(ba.get)() /
// functools.partial(ba.get)() under foreign same-leaf (outer zero-arg apply of
// a bound-method partial factory).
func pythonPartialBoundMethodCallResultType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "call" {
		return ""
	}
	return pythonPartialBoundMethodReturnType(fn, content, funcReturns, typeOf, fieldOf)
}

// pythonSubscriptElemType recovers the element type of items[i] / d[k] when the
// subscripted value is a known collection (elemOf / wrappers / literals).
// Also covers d.popitem()[1] / (d.popitem())[1] (pair value leaf; see
// pythonDictPopitemSubscriptValueType) and item[i] when item is a known
// enumerate/zip pair local (pairSlots). Fails closed on slices (items[a:b] /
// items[:]) — those yield sequences, not elements.
func pythonSubscriptElemType(sub *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, pairSlots, pairIterSlots map[string][]string, fieldOf map[string]string) string {
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
	// next(...items())[1] / min(...items())[1] — items pair value leaf.
	if et := pythonItemsCallSubscriptValueType(sub, content, elemOf, fieldOf); et != "" {
		return et
	}
	// list(...items())[i][1] / tuple(...items())[i][1] — deep items stack value
	// at declaration-order index i (asdict) or homogeneous typed-dict value.
	if et := pythonItemsIndexValueType(sub, content, elemOf, fieldOf); et != "" {
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
		// ca.most_common() / ca.most_common(n) — (elem, count) pairs; count untyped.
		if types := pythonMostCommonPairSlots(right, content, elemOf); len(types) > 0 {
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

// pythonObjectNextPairSlots recovers pair slots for next(enumerate([ba.get()])) /
// next(zip([ba.get()], …)) under foreign same-leaf (object pair-iters).
func pythonObjectNextPairSlots(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
	return pythonObjectPairIterSlotsOf(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonObjectPairIterSlotsOf peels object enumerate/zip pair-iters and identity
// list/tuple/iter/reversed/sorted/filter wrappers (same wrappers as
// pythonPairIterSlotsOf Class path). Also combinations/permutations/batched
// with literal r/n over object iterables under foreign same-leaf.
func pythonObjectPairIterSlotsOf(right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
	if right == nil {
		return nil
	}
	for right != nil && right.Type() == "parenthesized_expression" {
		right = pythonParenInner(right)
	}
	if right == nil || right.Type() != "call" {
		return nil
	}
	if types := pythonObjectEnumerateZipTargetTypes(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
		return types
	}
	// combinations/permutations/combinations_with_replacement/batched with
	// literal r/n over object iterables → homogeneous pair slots.
	if types := pythonObjectCombBatchedPairSlots(right, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
		return types
	}
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
		if len(args) == 0 {
			return nil
		}
		return pythonObjectPairIterSlotsOf(args[0], content, funcReturns, typeOf, fieldOf)
	case "filter":
		if len(args) < 2 {
			return nil
		}
		return pythonObjectPairIterSlotsOf(args[1], content, funcReturns, typeOf, fieldOf)
	}
	return nil
}

// pythonObjectCombBatchedPairSlots is the object-iter form of
// pythonCombBatchedPairSlots: combinations/permutations/batched over
// [ba.get(), …] with literal r/n → [elem]*n. Enables
// list(combinations([ba.get(), ba.get()], 2))[0][0].run() under foreign same-leaf.
func pythonObjectCombBatchedPairSlots(right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
	et := pythonObjectCombPermElemType(right, content, funcReturns, typeOf, fieldOf)
	if et == "" {
		et = pythonObjectBatchedElemType(right, content, funcReturns, typeOf, fieldOf)
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

// pythonObjectCombPermElemType recovers shared T from combinations/permutations/
// combinations_with_replacement of an object iterable (1st arg).
func pythonObjectCombPermElemType(right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
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
	return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonObjectBatchedElemType recovers T from batched/itertools.batched of an
// object iterable (1st arg; n/strict ignored for element typing).
func pythonObjectBatchedElemType(right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
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
	return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonObjectPairSlotsOf recovers per-slot types for an object pair expression:
// next(enumerate([ba.get()])) / next(zip([ba.get()], …)),
// min/max/choice/heappop/itemgetter/pop of list(zip([ba.get()], …)),
// list(enumerate([ba.get()]))[0] / list(zip([ba.get()], …))[0]
// (index into a pair-yielding sequence yields one pair with those slots).
// Parenthesized forms accepted. Slices fail closed. Same shapes as
// pythonPairSlotsOf Class path for solid object peels.
func pythonObjectPairSlotsOf(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
	case "call":
		// next(enumerate([ba.get()])) / next(zip([ba.get()], …))
		if types := pythonObjectNextPairSlots(n, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
			return types
		}
		// min/max(list(zip([ba.get()], …)))
		if types := pythonObjectMinMaxPairSlots(n, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
			return types
		}
		// choice/random.choice(list(zip([ba.get()], …)))
		if types := pythonObjectChoicePairSlots(n, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
			return types
		}
		// heappop/heapq.heappop(list(zip([ba.get()], …)))
		if types := pythonObjectHeappopPairSlots(n, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
			return types
		}
		// itemgetter(0)(list(zip([ba.get()], …)))
		if types := pythonObjectItemgetterPairSlots(n, content, funcReturns, typeOf, fieldOf); len(types) > 0 {
			return types
		}
		// list(zip([ba.get()], …)).pop()
		return pythonObjectPopPairSlots(n, content, funcReturns, typeOf, fieldOf)
	case "subscript":
		// list(enumerate([ba.get()]))[0] / list(zip([ba.get()], …))[0] /
		// (list(enumerate([ba.get()])))[0] — one pair from a pair-yielding sequence.
		for i := uint32(0); i < n.ChildCount(); i++ {
			if n.Child(i).Type() == "slice" {
				return nil
			}
		}
		val := ingest.ChildByField(n, "value")
		return pythonObjectPairIterSlotsOf(val, content, funcReturns, typeOf, fieldOf)
	default:
		return nil
	}
}

// pythonObjectMinMaxPairSlots is the object-iter form of pythonMinMaxPairSlots.
func pythonObjectMinMaxPairSlots(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
	return pythonObjectPairIterSlotsOf(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonObjectChoicePairSlots is the object-iter form of pythonChoicePairSlots.
func pythonObjectChoicePairSlots(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
	return pythonObjectPairIterSlotsOf(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonObjectHeappopPairSlots is the object-iter form of pythonHeappopPairSlots.
func pythonObjectHeappopPairSlots(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
	return pythonObjectPairIterSlotsOf(args[0], content, funcReturns, typeOf, fieldOf)
}

// pythonObjectItemgetterPairSlots is the object-iter form of pythonItemgetterPairSlots.
func pythonObjectItemgetterPairSlots(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
	if call == nil || call.Type() != "call" {
		return nil
	}
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
	idxArgs, ok := pythonCallPositionalArgNodes(fn)
	if !ok || len(idxArgs) != 1 {
		return nil
	}
	collArgs, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(collArgs) != 1 {
		return nil
	}
	return pythonObjectPairIterSlotsOf(collArgs[0], content, funcReturns, typeOf, fieldOf)
}

// pythonObjectPopPairSlots is the object-iter form of pythonPopPairSlots.
func pythonObjectPopPairSlots(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
	return pythonObjectPairIterSlotsOf(obj, content, funcReturns, typeOf, fieldOf)
}

// pythonObjectPairSlotSubscriptType returns the slot type for
// list(enumerate([ba.get()]))[0][1] / list(zip([ba.get()], …))[0][0] /
// next(enumerate([ba.get()]))[1] when i is a non-negative integer literal and
// the pair peels via object pair-iters. Untyped slots (enumerate index), OOB
// indices, and non-literal indices fail closed.
func pythonObjectPairSlotSubscriptType(sub *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
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
	slots := pythonObjectPairSlotsOf(ingest.ChildByField(sub, "value"), content, funcReturns, typeOf, fieldOf)
	if idx < 0 || idx >= len(slots) {
		return ""
	}
	return slots[idx]
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
// {**da} / {**da, "k": A()} (dictionary star-copy of typed mapping values),
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
		if et := pythonHomogeneousCtorElem(right, content); et != "" {
			return et
		}
		// [*xs, A()] / [*xs, *ys] — star-list peels (no self-target outside assign).
		if right.Type() == "list" {
			return pythonHomogeneousSplatListCtorElem(right, content, elemOf, egElems, typeOf, "")
		}
		return ""
	case "dictionary":
		// {**da} / {**da, **db} / {**da, "k": A()} — star-copy of typed mapping
		// values (same leaf as da.copy() / da["k"]). Enables {**da}["k"].run().
		return pythonDictStarCopyValueType(right, content, elemOf)
	case "binary_operator":
		// xs + [A()] / [A()] + xs / xs + ys — list concat element type when sides
		// agree; empty list/tuple is a wildcard. Self-target untyped arms only in
		// assignment path (pythonListConcatElemType with target name).
		if et := pythonListConcatElemType(right, content, elemOf, egElems, typeOf, ""); et != "" {
			return et
		}
		// [A()] * n / n * [A()] / (A(),) * n — list/tuple repeat preserves element
		// type (integer arm ignored). Annotated list[A] locals already peel; this
		// covers unannotated [A()] * 2 under foreign same-leaf.
		if et := pythonListRepeatElemType(right, content, elemOf, egElems, typeOf); et != "" {
			return et
		}
		// da | ea / da | {"j": A()} / {} | {"k": A()} — dict merge value leaf
		// (PEP 584). Empty dict is a wildcard. Nested list values fail closed here
		// (use pythonDictMergeNestedValueType / nested subscript peels).
		return pythonDictMergeValueType(right, content, elemOf)
	case "subscript":
		// xs[:n] / xs[i:j] / xs[:] / xs[::step] — slice of a collection preserves
		// element type (as_[:1][0].run() / sa = as_[:1]; sa[0].run()).
		idx := ingest.ChildByField(right, "subscript")
		if idx != nil && idx.Type() == "slice" {
			return pythonIterableElemType(ingest.ChildByField(right, "value"), content, elemOf, egElems, typeOf)
		}
		// ChainMap(da).maps[0] / ca.maps[0] — maps list element is a mapping with
		// the same scalar value leaf as the ChainMap (maps[0]["k"].run()).
		if et := pythonChainMapMapsIndexValueType(right, content, elemOf); et != "" {
			return et
		}
		// da["k"] when da: defaultdict[str, list[A]] — value is list of A;
		// iterating / further [0] peels A (elemOf["@nested."+da]).
		if nest := pythonNestedMappingSubscriptElemType(right, content, elemOf); nest != "" {
			return nest
		}
		// Other non-slice subscripts are elements, not collections — fail closed.
		return ""
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
	case "conditional_expression":
		// (aa if c else ca)[0] / for a in (aa if c else ca) — both arms agree on
		// collection element type under foreign same-leaf methods.
		return pythonConditionalIterableElemType(right, content, elemOf, egElems, typeOf)
	case "generator_expression", "list_comprehension", "set_comprehension":
		// next(x for x in items) / for a in [x for x in items] — identity body only.
		return pythonComprehensionElemType(right, content, elemOf, egElems, typeOf)
	case "call":
		// ChainMap(da) / ChainMap(da, ea) / ChainMap({"k": A()}) — mapping value
		// leaf for subscript peels (same model as elemOf[da] for dict locals).
		// Nested list-value maps fail closed here (pythonChainMapLocalNestedValueType).
		if et := pythonChainMapLocalValueType(right, content, elemOf); et != "" {
			return et
		}
		// MappingProxyType(da) / types.MappingProxyType(da) — read-only mapping of
		// same value leaf as da (or homogeneous dict literal of Class()).
		if et := pythonMappingProxyValueType(right, content, elemOf); et != "" {
			return et
		}
		// ChainMap(da).new_child() / ChainMap(da).new_child({"j": A()}) scalar.
		if et := pythonChainMapNewChildValueType(right, content, elemOf); et != "" {
			return et
		}
		// Literal ChainMap({"k": A()}) / ChainMap(OrderedDict(k=A())) without locals.
		if et := pythonHomogeneousDictValueCtorElem(right, content); et != "" &&
			pythonSimpleCalleeName(ingest.ChildByField(right, "function"), content) == "ChainMap" {
			return et
		}
		// reversed(xs) / sorted(xs) / list(xs) / tuple(xs) / set(xs) /
		// frozenset(xs) / iter(xs) / deque(xs) / Counter(xs) — element type
		// equals the wrapped iterable. Nested wrappers recurse.
		// filter(pred, xs) — element type equals xs (pred only selects).
		// map(A, xs) — element type is A when first arg is a Class identifier;
		// other map callables fail closed (unknown result type).
		// chain(xs, ys) / islice(xs, n) — itertools helpers (bare or imported).
		// gen_a() after same-file def gen_a(): yield A() — yield type via
		// elemOf["@yield.gen_a"] (see pythonSameFileGeneratorYields).
		if fn := ingest.ChildByField(right, "function"); fn != nil && fn.Type() == "identifier" {
			if elemOf != nil {
				if et := elemOf["@yield."+ingest.NodeText(fn, content)]; et != "" {
					return et
				}
			}
			switch ingest.NodeText(fn, content) {
			case "reversed", "sorted", "list", "tuple", "set", "frozenset", "iter", "deque", "Counter", "UserList", "copy", "deepcopy":
				// Counter(iterable) keys are the iterable elements (product case).
				// UserList(iterable) / UserList([A()]) — element type of 1st arg
				// (collections.UserList ABC; same leaf as list(...)).
				// frozenset(iterable) same as set (immutable). Mapping/kwargs
				// constructors fail closed when untyped / non-iterable.
				// copy(xs) / deepcopy(xs) — from copy import copy, deepcopy; preserve
				// the arg's element type (same as copy.copy(xs) module-qualified).
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) == 0 {
					return ""
				}
				// copy/deepcopy require exactly one positional arg (object/collection).
				if name := ingest.NodeText(fn, content); (name == "copy" || name == "deepcopy") && len(args) != 1 {
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
				// map(lambda x: x, iterable) — identity lambda preserves iterable element type
				// (list(map(lambda x: x, items))[0].run() under foreign same-leaf).
				// Other callables fail closed (unknown result type).
				args, ok := pythonCallPositionalArgNodes(right)
				if !ok || len(args) == 0 {
					return ""
				}
				if args[0].Type() == "identifier" {
					return ingest.NodeText(args[0], content)
				}
				// Identity map only (not starmap — multi-arg unpack is not identity of one elem).
				if ingest.NodeText(fn, content) == "map" && len(args) >= 2 && pythonIsIdentityLambda(args[0], content) {
					return pythonIterableElemType(args[1], content, elemOf, egElems, typeOf)
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
				case "copy", "deepcopy", "elements":
					// copy.copy(xs) / copy.deepcopy(xs) — module-qualified; preserve
					// the arg's element type (same as items.copy() for collections).
					// items.copy() / Counter.elements() — zero-arg; element type of receiver.
					// Other receivers / arity fail closed.
					args, ok := pythonCallPositionalArgNodes(right)
					if !ok {
						return ""
					}
					obj := ingest.ChildByField(fn, "object")
					method := ingest.NodeText(attr, content)
					if (method == "copy" || method == "deepcopy") &&
						obj != nil && obj.Type() == "identifier" &&
						ingest.NodeText(obj, content) == "copy" {
						if len(args) != 1 {
							return ""
						}
						return pythonIterableElemType(args[0], content, elemOf, egElems, typeOf)
					}
					if method == "deepcopy" {
						return ""
					}
					if len(args) != 0 {
						return ""
					}
					return pythonIterableElemType(obj, content, elemOf, egElems, typeOf)
				case "values":
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || elemOf == nil {
						return ""
					}
					for objN != nil && objN.Type() == "parenthesized_expression" {
						objN = pythonParenInner(objN)
					}
					if objN == nil {
						return ""
					}
					// d.values() when d is a bare mapping local.
					// Scalar mapping values (dict[str, A]) stay on elemOf[d].
					// Nested list values (defaultdict[str, list[A]]) are not scalar —
					// for ga in da.values() binds via pythonNestedMappingValuesElemType.
					if objN.Type() == "identifier" {
						obj := ingest.NodeText(objN, content)
						if nest := elemOf["@nested."+obj]; nest != "" {
							// Nested list values are not scalar A — fail closed here.
							return ""
						}
						return elemOf[obj]
					}
					// la[0].values() when la: list[dict[str, A]] — la[0] peels as a
					// mapping/container of A via @nested; values yield A under foreign
					// same-leaf. {**da}.values() star-copy peels via dictionary case.
					// (da | ea).values() / ChainMap(da).values() — scalar merge / ChainMap.
					if objN.Type() == "subscript" || objN.Type() == "dictionary" {
						return pythonIterableElemType(objN, content, elemOf, egElems, typeOf)
					}
					if objN.Type() == "binary_operator" {
						return pythonDictMergeValueType(objN, content, elemOf)
					}
					if objN.Type() == "call" {
						if et := pythonChainMapLocalValueType(objN, content, elemOf); et != "" {
							return et
						}
						if et := pythonMappingProxyValueType(objN, content, elemOf); et != "" {
							return et
						}
						if et := pythonHomogeneousDictValueCtorElem(objN, content); et != "" &&
							pythonSimpleCalleeName(ingest.ChildByField(objN, "function"), content) == "ChainMap" {
							return et
						}
						// dict.fromkeys(keys, A()).values() / dict.fromkeys(keys, value=A()).values()
						// — value leaf is A (same as d = dict.fromkeys(...); d.values()).
						if et := pythonDictFromkeysObjectValueType(objN, content, nil, nil, nil); et != "" {

							return et
						}
					}
					return ""
				case "get", "setdefault", "pop":
					// da.get("k") / da.pop("k") when da: defaultdict[str, list[A]] —
					// value is list of A; da.get("k")[0].run() peels via subscript.
					// {**da}.get("k") / da.copy().get("k") — nested star-copy / copy.
					// (da | ea).get("k") / ChainMap(da).get("k") — nested merge / ChainMap.
					// Scalar dict[str, A] get stays on assignment/collection-access
					// (elemOf leaf) — not an iterable of A here.
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || elemOf == nil {
						return ""
					}
					for objN != nil && objN.Type() == "parenthesized_expression" {
						objN = pythonParenInner(objN)
					}
					if objN == nil {
						return ""
					}
					switch objN.Type() {
					case "identifier":
						return elemOf["@nested."+ingest.NodeText(objN, content)]
					case "dictionary":
						return pythonDictStarCopyNestedValueType(objN, content, elemOf)
					case "binary_operator":
						return pythonDictMergeNestedValueType(objN, content, elemOf)
					case "call":
						if nest := pythonNestedMappingCopyCallType(objN, content, elemOf); nest != "" {
							return nest
						}
						if nest := pythonChainMapLocalNestedValueType(objN, content, elemOf); nest != "" {
							return nest
						}
						return pythonNestedDictHomogeneousListCtorElem(objN, content)
					}
					return ""
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
				case "deque", "Counter", "UserList":
					// collections.deque(iterable[, maxlen]) /
					// collections.Counter(iterable) /
					// collections.UserList(iterable) — element of 1st arg.
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
		// xs.data / da.data — UserList/UserDict underlying container shares
		// element/value type of the collection local (xs.data[0].run() /
		// for a in xs.data / d = xs.data; d[0].run() under foreign same-leaf).
		// ChainMap(da, ea).parents / ca.parents — remaining maps ChainMap with
		// same scalar value leaf (parents["k"].run() under foreign same-leaf).
		obj := ingest.ChildByField(right, "object")
		attr := ingest.ChildByField(right, "attribute")
		if obj == nil || attr == nil {
			return ""
		}
		switch ingest.NodeText(attr, content) {
		case "exceptions":
			if obj.Type() != "identifier" || egElems == nil {
				return ""
			}
			return egElems[ingest.NodeText(obj, content)]
		case "data":
			if obj.Type() != "identifier" || elemOf == nil {
				return ""
			}
			return elemOf[ingest.NodeText(obj, content)]
		case "parents":
			// ChainMap.parents is itself a ChainMap of remaining maps.
			return pythonChainMapExprScalarValueType(obj, content, elemOf)
		}
		return ""
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

// pythonListConcatElemType recovers T from list-concat binary + expressions:
//
//	xs + [A()] / [A()] + xs / xs + ys / [] + [A()] / (xs + [A()]) + [A()]
//
// Arms must agree when typed. Empty list/tuple literals are wildcards. When
// selfTarget is non-empty (assignment `xs = xs + [A()]` / `xs += …`), an untyped
// identifier arm equal to selfTarget is also a wildcard. Non-+ operators and
// mismatched leaves fail closed.
func pythonListConcatElemType(n *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, selfTarget string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil || n.Type() != "binary_operator" {
		return ""
	}
	op := ingest.ChildByField(n, "operator")
	if op == nil || ingest.NodeText(op, content) != "+" {
		return ""
	}
	left := ingest.ChildByField(n, "left")
	right := ingest.ChildByField(n, "right")
	if left == nil || right == nil {
		return ""
	}
	etL := pythonListConcatArmElemType(left, content, elemOf, egElems, typeOf, selfTarget)
	etR := pythonListConcatArmElemType(right, content, elemOf, egElems, typeOf, selfTarget)
	wildL := pythonListConcatArmWildcard(left, content, elemOf, selfTarget)
	wildR := pythonListConcatArmWildcard(right, content, elemOf, selfTarget)
	if etL != "" && etR != "" {
		if etL == etR {
			return etL
		}
		return ""
	}
	if etL != "" && wildR {
		return etL
	}
	if etR != "" && wildL {
		return etR
	}
	return ""
}

// pythonListConcatArmElemType types one + arm: nested +, homogeneous Class()
// list/tuple, known iterable, or "".
func pythonListConcatArmElemType(n *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, selfTarget string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if n.Type() == "binary_operator" {
		return pythonListConcatElemType(n, content, elemOf, egElems, typeOf, selfTarget)
	}
	if et := pythonHomogeneousCtorElem(n, content); et != "" {
		return et
	}
	if et := pythonHomogeneousSplatListCtorElem(n, content, elemOf, egElems, typeOf, selfTarget); et != "" {
		return et
	}
	return pythonIterableElemType(n, content, elemOf, egElems, typeOf)
}

// pythonListConcatArmWildcard reports empty collection literals or untyped
// selfTarget identifiers (xs = xs + [A()] before xs is typed).
func pythonListConcatArmWildcard(n *grammar.Node, content []byte, elemOf map[string]string, selfTarget string) bool {
	if n == nil {
		return false
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return false
	}
	if pythonIsEmptyCollectionLiteral(n) {
		return true
	}
	if selfTarget != "" && n.Type() == "identifier" {
		name := ingest.NodeText(n, content)
		if name == selfTarget {
			if elemOf == nil || elemOf[name] == "" {
				return true
			}
		}
	}
	return false
}

// pythonListRepeatElemType recovers T from list/tuple repetition:
//
//	[A()] * n / n * [A()] / (A(),) * n / xs * n when xs peels to element T
//
// One arm must be an integer literal; the other is a list/tuple source of T.
// Non-* operators / non-integer counters / mixed leaves fail closed.
func pythonListRepeatElemType(n *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil || n.Type() != "binary_operator" {
		return ""
	}
	op := ingest.ChildByField(n, "operator")
	if op == nil || ingest.NodeText(op, content) != "*" {
		return ""
	}
	left := ingest.ChildByField(n, "left")
	right := ingest.ChildByField(n, "right")
	if left == nil || right == nil {
		return ""
	}
	leftInt := pythonIsIntegerLiteral(left, content)
	rightInt := pythonIsIntegerLiteral(right, content)
	if leftInt == rightInt {
		return ""
	}
	coll := right
	if rightInt {
		coll = left
	}
	for coll != nil && coll.Type() == "parenthesized_expression" {
		coll = pythonParenInner(coll)
	}
	if coll == nil {
		return ""
	}
	if et := pythonHomogeneousCtorElem(coll, content); et != "" {
		return et
	}
	return pythonIterableElemType(coll, content, elemOf, egElems, typeOf)
}

// pythonIsIntegerLiteral reports integer / unary +/- integer expressions.
func pythonIsIntegerLiteral(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return false
	}
	if n.Type() == "integer" {
		return true
	}
	if n.Type() == "unary_operator" {
		arg := ingest.ChildByField(n, "argument")
		if arg == nil {
			for i := uint32(0); i < n.ChildCount(); i++ {
				ch := n.Child(i)
				if ch.Type() == "integer" {
					return true
				}
			}
			return false
		}
		return pythonIsIntegerLiteral(arg, content)
	}
	return false
}

// pythonChainMapNewChildValueType recovers T from ChainMap(da).new_child() /
// ChainMap(da).new_child({"j": A()}) / ca.new_child() when parent and optional
// child agree on scalar mapping value leaf T. Zero-arg new_child preserves
// parent; one positional mapping arg merges when leaves agree. Nested list
// values use pythonChainMapNewChildNestedValueType. Mixed / kwargs / splat fail closed.
func pythonChainMapNewChildValueType(call *grammar.Node, content []byte, elemOf map[string]string) string {
	parent, child, ok := pythonChainMapNewChildParts(call, content)
	if !ok {
		return ""
	}
	pt := pythonChainMapExprScalarValueType(parent, content, elemOf)
	if pt == "" {
		return ""
	}
	if child == nil {
		return pt
	}
	ct := pythonChainMapChildScalarValueType(child, content, elemOf)
	if ct == "" || ct != pt {
		return ""
	}
	return pt
}

// pythonChainMapNewChildNestedValueType recovers T from ChainMap(da).new_child() /
// ChainMap(da).new_child({"j": [A()]}) / ca.new_child() when parent and optional
// child agree on nested mapping-of-list/set leaf T.
func pythonChainMapNewChildNestedValueType(call *grammar.Node, content []byte, elemOf map[string]string) string {
	parent, child, ok := pythonChainMapNewChildParts(call, content)
	if !ok {
		return ""
	}
	pt := pythonChainMapExprNestedValueType(parent, content, elemOf)
	if pt == "" {
		return ""
	}
	if child == nil {
		return pt
	}
	ct := pythonChainMapChildNestedValueType(child, content, elemOf)
	if ct == "" || ct != pt {
		return ""
	}
	return pt
}

// pythonChainMapNewChildParts returns (parent, childOrNil, ok) for
// parent.new_child([child]). Only the new_child attribute form is accepted.
func pythonChainMapNewChildParts(call *grammar.Node, content []byte) (parent, child *grammar.Node, ok bool) {
	if call == nil || call.Type() != "call" {
		return nil, nil, false
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return nil, nil, false
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "new_child" {
		return nil, nil, false
	}
	parent = ingest.ChildByField(fn, "object")
	if parent == nil {
		return nil, nil, false
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return parent, nil, true
	}
	var args []*grammar.Node
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument", "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return nil, nil, false
		default:
			args = append(args, ch)
		}
	}
	switch len(args) {
	case 0:
		return parent, nil, true
	case 1:
		return parent, args[0], true
	default:
		return nil, nil, false
	}
}

func pythonChainMapExprScalarValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if et := pythonChainMapLocalValueType(n, content, elemOf); et != "" {
		return et
	}
	if et := pythonChainMapNewChildValueType(n, content, elemOf); et != "" {
		return et
	}
	if n.Type() == "call" {
		if et := pythonHomogeneousDictValueCtorElem(n, content); et != "" &&
			pythonSimpleCalleeName(ingest.ChildByField(n, "function"), content) == "ChainMap" {
			return et
		}
	}
	if n.Type() == "identifier" && elemOf != nil {
		if elemOf["@nested."+ingest.NodeText(n, content)] != "" {
			return ""
		}
		return elemOf[ingest.NodeText(n, content)]
	}
	return ""
}

func pythonChainMapExprNestedValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if et := pythonChainMapLocalNestedValueType(n, content, elemOf); et != "" {
		return et
	}
	if et := pythonChainMapNewChildNestedValueType(n, content, elemOf); et != "" {
		return et
	}
	if n.Type() == "call" {
		if et := pythonNestedDictHomogeneousListCtorElem(n, content); et != "" &&
			pythonSimpleCalleeName(ingest.ChildByField(n, "function"), content) == "ChainMap" {
			return et
		}
	}
	if n.Type() == "identifier" && elemOf != nil {
		return elemOf["@nested."+ingest.NodeText(n, content)]
	}
	return ""
}

func pythonChainMapChildScalarValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if n.Type() == "identifier" && elemOf != nil {
		if elemOf["@nested."+ingest.NodeText(n, content)] != "" {
			return ""
		}
		return elemOf[ingest.NodeText(n, content)]
	}
	return pythonHomogeneousDictValueCtorElem(n, content)
}

func pythonChainMapChildNestedValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if n.Type() == "identifier" && elemOf != nil {
		return elemOf["@nested."+ingest.NodeText(n, content)]
	}
	return pythonNestedDictHomogeneousListCtorElem(n, content)
}

// pythonHomogeneousSplatListCtorElem recovers T from star-lists of Class() and
// typed splats:
//
//	[*xs, A()] / [A(), *xs] / [*xs, *ys] (same T) / [*xs, A(), A()]
//
// selfTarget untyped identifiers in splats are wildcards (xs = [*xs, A()]).
// Pure Class()-only lists return "" (use pythonHomogeneousCtorElem). Mixed
// leaves / unknown non-self splats fail closed.
func pythonHomogeneousSplatListCtorElem(n *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string, selfTarget string) string {
	if n == nil || n.Type() != "list" {
		return ""
	}
	var elem string
	saw := false
	sawSplat := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		switch ch.Type() {
		case "[", "]", ",", "comment":
			continue
		case "list_splat":
			sawSplat = true
			var val *grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				c := ch.Child(j)
				if c.Type() == "*" {
					continue
				}
				val = c
				break
			}
			if val == nil {
				return ""
			}
			for val != nil && val.Type() == "parenthesized_expression" {
				val = pythonParenInner(val)
			}
			if val == nil {
				return ""
			}
			if pythonIsEmptyCollectionLiteral(val) {
				continue
			}
			et := ""
			if val.Type() == "identifier" {
				name := ingest.NodeText(val, content)
				if elemOf != nil {
					et = elemOf[name]
				}
				if et == "" && selfTarget != "" && name == selfTarget {
					// xs = [*xs, A()] — untyped self splat is wildcard.
					continue
				}
				if et == "" {
					return ""
				}
			} else {
				et = pythonIterableElemType(val, content, elemOf, egElems, typeOf)
				if et == "" {
					return ""
				}
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
				return ""
			}
		case "call":
			et := pythonClassCtorName(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
				return ""
			}
		default:
			return ""
		}
	}
	if !sawSplat || !saw {
		return ""
	}
	return elem
}

// pythonMostCommonPairSlots recovers (elem, count) slots from
// ca.most_common() / ca.most_common(n) when ca has a known element type
// (Counter keys). Count slot is untyped (""). Enables
// `for a, _ in ca.most_common(): a.run()` under foreign same-leaf.
func pythonMostCommonPairSlots(call *grammar.Node, content []byte, elemOf map[string]string) []string {
	if call == nil || call.Type() != "call" || elemOf == nil {
		return nil
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return nil
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "most_common" {
		return nil
	}
	obj := ingest.ChildByField(fn, "object")
	if obj == nil || obj.Type() != "identifier" {
		return nil
	}
	et := elemOf[ingest.NodeText(obj, content)]
	if et == "" {
		return nil
	}
	return []string{et, ""}
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
// yielded by groupby(iterable[, key]) / itertools.groupby(...), or of an
// assigned groupby local (`groups = groupby(items)` → elemOf["@groupby.groups"]).
// Yields (key, group) pairs; group iterates elements of the 1st positional arg
// (key function ignored). Bare or itertools-qualified only; other forms fail closed.
// Identity wrappers iter/list/tuple(groupby(...)) peel (same leaf as bare).
// Not registered in pythonIterableElemType — bare `for x in groupby(xs)` yields
// pairs, not elements. Assigned locals live under @groupby.* so
// `_, ga = next(groups)` / `for k, g in groups` type under foreign same-leaf.
func pythonGroupbyGroupElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if right == nil {
		return ""
	}
	for right != nil && right.Type() == "parenthesized_expression" {
		right = pythonParenInner(right)
	}
	if right == nil {
		return ""
	}
	// groups = groupby(items); next(groups) / for k, g in groups.
	if right.Type() == "identifier" {
		if elemOf == nil {
			return ""
		}
		return elemOf["@groupby."+ingest.NodeText(right, content)]
	}
	if right.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		switch ingest.NodeText(fn, content) {
		case "groupby":
			// fall through to arg peel below
		case "iter", "list", "tuple":
			// groups = iter(groupby(items)) / list(groupby(...)) / tuple(...).
			args, ok := pythonCallPositionalArgNodes(right)
			if !ok || len(args) == 0 {
				return ""
			}
			return pythonGroupbyGroupElemType(args[0], content, elemOf, egElems, typeOf)
		default:
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

// pythonNextGroupbyGroupElemType recovers the group-iterator element type from
// next(groupby(iterable[, key])) / next(itertools.groupby(...)) /
// next(groups) when groups is an assigned groupby local.
// next yields one (key, group) pair; group iterates elements of the groupby
// iterable (key= ignored). Used for unpack `_, ga = next(groupby(items))` so
// subsequent next(ga) / for a in ga type under foreign same-leaf methods.
// Default arg of next is ignored. Other next args fail closed.
func pythonNextGroupbyGroupElemType(call *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "identifier" || ingest.NodeText(fn, content) != "next" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	return pythonGroupbyGroupElemType(args[0], content, elemOf, egElems, typeOf)
}

// pythonNextGroupbyGroupSubElemType recovers the group-iterator element type from
// next(groupby(...))[1] / next(groups)[1] / (next(groupby(...)))[1].
// Index must be integer literal 1 (group slot of the (key, group) pair).
// [0] (key) and other indices fail closed. Result is a group iterator of the
// groupby iterable's elements (same leaf as `_, ga = next(groupby(...))`).
func pythonNextGroupbyGroupSubElemType(sub *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
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
	return pythonNextGroupbyGroupElemType(val, content, elemOf, egElems, typeOf)
}

// pythonTeeElemType recovers the element type of each independent iterator
// returned by tee(iterable[, n]) / itertools.tee(...).
// tee yields a tuple of n iterators (default n=2); each iterator yields elements
// of the 1st positional arg (n ignored). Bare or itertools-qualified only.
// Not registered in pythonIterableElemType — bare `for x in tee(xs)` yields
// iterators, not elements. Callers bind unpack targets into elemOf so nested
// `for a in it1` / next(it1) / it1.__next__() type.
// Class()/typed-local peels via pythonIterableElemType; method-return object
// iterables (tee([ba.get()])) via pythonObjectIterableElemType under foreign
// same-leaf when funcReturns is provided.
func pythonTeeElemType(right *grammar.Node, content []byte, elemOf, egElems, typeOf, funcReturns, fieldOf map[string]string) string {
	arg := pythonTeeFirstArg(right, content)
	if arg == nil {
		return ""
	}
	if et := pythonIterableElemType(arg, content, elemOf, egElems, typeOf); et != "" {
		return et
	}
	// it1, it2 = tee([ba.get()]) / itertools.tee([ba.get()][, n]) — method-return
	// object collection under foreign same-leaf (Class peels above).
	return pythonObjectIterableElemType(arg, content, funcReturns, typeOf, fieldOf)
}

// pythonTeeFirstArg returns the 1st positional arg of tee(...) / itertools.tee(...)
// or nil when the call is not a resolvable bare/itertools-qualified tee form.
func pythonTeeFirstArg(right *grammar.Node, content []byte) *grammar.Node {
	if right == nil || right.Type() != "call" {
		return nil
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil {
		return nil
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "tee" {
			return nil
		}
	case "attribute":
		objN := ingest.ChildByField(fn, "object")
		attrN := ingest.ChildByField(fn, "attribute")
		if objN == nil || attrN == nil || objN.Type() != "identifier" {
			return nil
		}
		if ingest.NodeText(objN, content) != "itertools" {
			return nil
		}
		if ingest.NodeText(attrN, content) != "tee" {
			return nil
		}
	default:
		return nil
	}
	args, ok := pythonCallPositionalArgNodes(right)
	if !ok || len(args) == 0 {
		return nil
	}
	return args[0]
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
	return pythonDictFromkeysObjectValueType(right, content, nil, nil, nil)
}

// pythonDictFromkeysObjectValueType peels dict.fromkeys value as Class() or
// method-return / typed-local leaf (ba.get() → A) under foreign same-leaf.
func pythonDictFromkeysObjectValueType(right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
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
	// 2nd positional — dict.fromkeys(keys, A()) / dict.fromkeys(keys, ba.get())
	if args, ok := pythonCallPositionalArgNodes(right); ok && len(args) >= 2 {
		if name := pythonObjectLeafType(args[1], content, funcReturns, typeOf, fieldOf); name != "" {
			return name
		}
	}
	// keyword value= — dict.fromkeys(keys, value=A()) / value=ba.get()
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
		return pythonObjectLeafType(valN, content, funcReturns, typeOf, fieldOf)
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
// (A() → "A"), conditional when both arms agree, or parenthesized form.
// Other shapes fail closed.
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
	case "conditional_expression":
		return pythonConditionalExprType(n, content, typeOf)
	case "parenthesized_expression":
		return pythonObjectExprType(pythonParenInner(n), content, typeOf)
	}
	return ""
}

// pythonConditionalArms returns (body, alternative) of a conditional_expression.
// tree-sitter python has no named fields — children are positional:
// body, "if", condition, "else", alternative.
func pythonConditionalArms(n *grammar.Node) (body, alt *grammar.Node) {
	if n == nil || n.Type() != "conditional_expression" {
		return nil, nil
	}
	seenIf, seenElse := false, false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "if":
			seenIf = true
		case "else":
			seenElse = true
		case "comment":
			continue
		default:
			if !seenIf && body == nil {
				body = ch
			} else if seenIf && !seenElse {
				// condition — ignore
			} else if seenElse && alt == nil {
				alt = ch
			}
		}
	}
	return body, alt
}

// pythonConditionalExprType recovers T from (a if c else x) when both arms peel
// to the same Class leaf via typed locals / Class() / nested conditionals.
// Mixed / untyped arms fail closed.
func pythonConditionalExprType(n *grammar.Node, content []byte, typeOf map[string]string) string {
	body, alt := pythonConditionalArms(n)
	t1 := pythonObjectExprType(body, content, typeOf)
	t2 := pythonObjectExprType(alt, content, typeOf)
	if t1 != "" && t1 == t2 {
		return t1
	}
	return ""
}

// pythonConditionalIterableElemType recovers collection element type from
// (aa if c else ca) when both arms share the same element leaf. Used for
// (aa if c else ca)[0].run() / for a in (aa if c else ca) under foreign same-leaf.
func pythonConditionalIterableElemType(n *grammar.Node, content []byte, elemOf, egElems, typeOf map[string]string) string {
	body, alt := pythonConditionalArms(n)
	t1 := pythonIterableElemType(body, content, elemOf, egElems, typeOf)
	t2 := pythonIterableElemType(alt, content, elemOf, egElems, typeOf)
	if t1 != "" && t1 == t2 {
		return t1
	}
	return ""
}

// pythonIsIdentityLambda reports whether n is `lambda x: x` / `lambda x: (x)`
// (single param; body is that param, optionally parenthesized). Other shapes
// fail closed (defaults / multi-param / transforming body).
func pythonIsIdentityLambda(n *grammar.Node, content []byte) bool {
	if n == nil || n.Type() != "lambda" {
		return false
	}
	params := ingest.ChildByField(n, "parameters")
	body := ingest.ChildByField(n, "body")
	if body == nil {
		return false
	}
	// Single identifier param only.
	var paramName string
	if params != nil {
		count := 0
		for i := uint32(0); i < params.ChildCount(); i++ {
			ch := params.Child(i)
			if ch == nil {
				continue
			}
			switch ch.Type() {
			case ",", "comment":
				continue
			case "identifier":
				count++
				if count == 1 {
					paramName = ingest.NodeText(ch, content)
				}
			default:
				// default_parameter / typed / splat — fail closed.
				return false
			}
		}
		if count != 1 || paramName == "" {
			return false
		}
	} else {
		return false
	}
	// Unwrap parenthesized body: lambda x: (x)
	for body != nil && body.Type() == "parenthesized_expression" {
		body = pythonParenInner(body)
	}
	if body == nil || body.Type() != "identifier" {
		return false
	}
	return ingest.NodeText(body, content) == paramName
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

// pythonComprehensionElemType recovers the element type of a generator/list/set
// comprehension:
//
//   - Identity: `x for x in items` / `[x for x in items if x]` — element type of
//     the iterable (body name matches the single for-target).
//   - Class() body: `[A() for _ in range(1)]` / `(A() for _ in xs)` — element type
//     is A, independent of the iterable (enables xs = [A() for …]; xs[0].run()
//     and next(A() for …).run() under foreign same-leaf).
//
// Nested fors and other transforming bodies (`f(x) for x in items`) fail closed.
// if_clause is ignored (filter does not change element type).
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
	if body == nil {
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
	if left == nil || right == nil {
		return ""
	}
	// Identity: body name must match the for-target (`x for x in items`).
	if body.Type() == "identifier" && left.Type() == "identifier" &&
		ingest.NodeText(body, content) == ingest.NodeText(left, content) {
		return pythonIterableElemType(right, content, elemOf, egElems, typeOf)
	}
	// Body is Class() / Class(...) — element type is the Class name
	// (`[A() for _ in range(1)]` / next(A() for _ in items)).
	// Typed-local / Class() peels via pythonObjectExprType for paren/ternary.
	if ct := pythonClassCtorName(body, content); ct != "" {
		return ct
	}
	if tn := pythonObjectExprType(body, content, typeOf); tn != "" {
		return tn
	}
	return ""
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

// pythonObjectLeafListTypes recovers per-slot Class leaves from a list/tuple/
// expression_list of object leaves (Class() / typed local / method-return):
// [ba.get()] / (ba.get(),) / ba.get(), / [A(), ba.get()]. Enables
// [mrA] = [ba.get()] / (mrA,) = (ba.get(),) / mrA, = [ba.get()] unpack under
// foreign same-leaf. Mixed empty / non-object slots fail closed.
func pythonObjectLeafListTypes(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
		default:
			et := pythonObjectLeafType(ch, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return nil
			}
			out = append(out, et)
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

// pythonObjectSubscriptElemType recovers T from coll[i] when coll peels as an
// object collection of T (not a slice). Enables
// xa = sorted([ba.get()])[0] / xa = [ba.get()][0] / xa = list([ba.get()])[0]
// under foreign same-leaf (direct coll[i].run() already peels via
// pythonShouldRenameAttr). Slices fail closed (sequence, not element).
func pythonObjectSubscriptElemType(sub *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	// Slice subscript (items[1:3] / items[:]) fails closed — not an element.
	for i := uint32(0); i < sub.ChildCount(); i++ {
		ch := sub.Child(i)
		if ch != nil && ch.Type() == "slice" {
			return ""
		}
	}
	val := ingest.ChildByField(sub, "value")
	if val == nil {
		return ""
	}
	if et := pythonObjectCollectionElem(val, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// list(Counter([ba.get()]).elements())[0] / list(Counter([ba.get()]))[0] /
	// list(filterfalse(..., [ba.get()]))[0] — method-return iterable wrappers
	// under foreign same-leaf (Class peels via pythonIterableElemType).
	if et := pythonObjectIterableElemType(val, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// {"k": ba.get()}["k"] / dict(k=ba.get())["k"] — object-dict value peel.
	if et := pythonHomogeneousObjectDictValue(val, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// MappingProxyType({"k": ba.get()})["k"] / types.MappingProxyType(...)["k"] —
	// proxy of object-dict value (inline already peels via pythonShouldRenameAttr;
	// assigned form needs typeOf bind under foreign same-leaf).
	if et := pythonMappingProxyObjectValueType(val, content, nil, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// ({"k": ba.get()} | {})["k"] / ({} | {"k": ba.get()})["k"] — object-dict
	// merge value (inline already peels via pythonShouldRenameAttr; assigned
	// form needs typeOf bind under foreign same-leaf).
	return pythonObjectDictMergeValueType(val, content, funcReturns, typeOf, fieldOf)
}

// pythonObjectCollectionElem recovers element type T from list/tuple/set
// expressions built of method-return / typed-local / Class() object leaves
// under foreign same-leaf:
//
//	[ba.get()] / (ba.get(),) / {ba.get()} / [a]
//	[ba.get()] + [ba.get()] / [] + [ba.get()]  (empty list/tuple wildcard)
//	[ba.get()] * n / n * [ba.get()]            (list/tuple repeat)
//	[*[ba.get()]] / [ba.get(), *[ba.get()]]    (star-list of object leaves)
//	[ba.get()] or [ba.get()] / [ba.get()] and [ba.get()]
//	list([ba.get()]) / tuple(...) / set(...) / deque(...) / sorted(...)
//	parenthesized / walrus / ternary when both arms agree
//
// Dict literals are intentionally excluded — use pythonHomogeneousObjectDictValue
// after nested dict peels so da = {"k": frozenset([A()])} still binds @nested
// (pythonClassCtorName would otherwise treat frozenset as a scalar leaf).
//
// Enables ([ba.get()]+[ba.get()])[0].run() / list([ba.get()])[0].run() /
// ([ba.get()] or [ba.get()])[0].run() / ([ba.get()]*2)[0].run() /
// [*[ba.get()]][0].run() and assigned forms. Mixed leaves /
// empty non-wildcard / non-object shapes fail closed.
func pythonObjectCollectionElem(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	for n != nil && !n.IsNull() {
		switch n.Type() {
		case "parenthesized_expression":
			n = pythonParenInner(n)
		case "named_expression":
			// (xs := [ba.get()])[0].run() — walrus value is the collection.
			n = ingest.ChildByField(n, "value")
		default:
			goto peeled
		}
	}
peeled:
	if n == nil || n.IsNull() {
		return ""
	}
	switch n.Type() {
	case "list", "tuple", "set":
		return pythonHomogeneousObjectElem(n, content, funcReturns, typeOf, fieldOf)
	case "binary_operator":
		op := ingest.ChildByField(n, "operator")
		if op == nil {
			return ""
		}
		switch ingest.NodeText(op, content) {
		case "+":
			// [ba.get()] + [ba.get()] / [] + [ba.get()] — list concat.
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			etL := pythonObjectCollectionElem(left, content, funcReturns, typeOf, fieldOf)
			etR := pythonObjectCollectionElem(right, content, funcReturns, typeOf, fieldOf)
			wildL := pythonIsEmptyCollectionLiteral(left)
			wildR := pythonIsEmptyCollectionLiteral(right)
			if etL != "" && etR != "" {
				if etL == etR {
					return etL
				}
				return ""
			}
			if etL != "" && wildR {
				return etL
			}
			if etR != "" && wildL {
				return etR
			}
			return ""
		case "*":
			// [ba.get()] * n / n * [ba.get()] / (ba.get(),) * n — list/tuple
			// repeat preserves element type (integer arm ignored).
			left := ingest.ChildByField(n, "left")
			right := ingest.ChildByField(n, "right")
			if left == nil || right == nil {
				return ""
			}
			leftInt := pythonIsIntegerLiteral(left, content)
			rightInt := pythonIsIntegerLiteral(right, content)
			if leftInt == rightInt {
				return ""
			}
			coll := right
			if rightInt {
				coll = left
			}
			return pythonObjectCollectionElem(coll, content, funcReturns, typeOf, fieldOf)
		default:
			return ""
		}
	case "boolean_operator":
		// [ba.get()] or [ba.get()] / [ba.get()] and [ba.get()] — both arms
		// agree (empty collection is a wildcard). Mixed fail closed.
		left := ingest.ChildByField(n, "left")
		right := ingest.ChildByField(n, "right")
		etL := pythonObjectCollectionElem(left, content, funcReturns, typeOf, fieldOf)
		etR := pythonObjectCollectionElem(right, content, funcReturns, typeOf, fieldOf)
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
	case "call":
		// list([ba.get()]) / tuple(...) / set(...) / deque(...) / UserList(...) /
		// copy([ba.get()]) / deepcopy([ba.get()]) — wrappers preserve element
		// type of the first positional arg.
		// copy.copy([ba.get()]) / copy.deepcopy([ba.get()]) — module-qualified.
		// [ba.get()].copy() — zero-arg shallow copy preserves element type.
		fn := ingest.ChildByField(n, "function")
		if fn == nil {
			return ""
		}
		if fn.Type() == "identifier" {
			switch ingest.NodeText(fn, content) {
			case "list", "tuple", "set", "frozenset", "iter", "reversed", "sorted", "deque", "UserList", "copy", "deepcopy":
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) == 0 {
					return ""
				}
				// copy/deepcopy require exactly one positional arg.
				if name := ingest.NodeText(fn, content); (name == "copy" || name == "deepcopy") && len(args) != 1 {
					return ""
				}
				return pythonObjectCollectionElem(args[0], content, funcReturns, typeOf, fieldOf)
			}
			return ""
		}
		if fn.Type() == "attribute" {
			attr := ingest.ChildByField(fn, "attribute")
			if attr == nil {
				return ""
			}
			method := ingest.NodeText(attr, content)
			obj := ingest.ChildByField(fn, "object")
			// copy.copy([ba.get()]) / copy.deepcopy([ba.get()]) — module-qualified
			// collection copy (element type of the single arg).
			if (method == "copy" || method == "deepcopy") &&
				obj != nil && obj.Type() == "identifier" &&
				ingest.NodeText(obj, content) == "copy" {
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) != 1 {
					return ""
				}
				return pythonObjectCollectionElem(args[0], content, funcReturns, typeOf, fieldOf)
			}
			// [ba.get()].copy() — zero-arg shallow copy preserves element type.
			if method != "copy" {
				return ""
			}
			args, ok := pythonCallPositionalArgNodes(n)
			if !ok || len(args) != 0 {
				return ""
			}
			return pythonObjectCollectionElem(obj, content, funcReturns, typeOf, fieldOf)
		}
		return ""
	case "conditional_expression":
		// ([ba.get()] if c else [ba.get()])[0].run() — both arms agree.
		body, alt := pythonConditionalArms(n)
		t1 := pythonObjectCollectionElem(body, content, funcReturns, typeOf, fieldOf)
		t2 := pythonObjectCollectionElem(alt, content, funcReturns, typeOf, fieldOf)
		if t1 != "" && t1 == t2 {
			return t1
		}
		return ""
	case "subscript":
		// [ba.get()][:] / [ba.get()][i:j] — slice preserves element type so
		// [ba.get()][:][0].run() peels under foreign same-leaf (Class() peels
		// via pythonIterableElemType on the slice value).
		hasSlice := false
		for i := uint32(0); i < n.ChildCount(); i++ {
			if n.Child(i).Type() == "slice" {
				hasSlice = true
				break
			}
		}
		if !hasSlice {
			return ""
		}
		return pythonObjectCollectionElem(ingest.ChildByField(n, "value"), content, funcReturns, typeOf, fieldOf)
	}
	return ""
}

// pythonObjectDictMergeValueType recovers T from a dict merge expression whose
// arms peel to the same object-dict value leaf via pythonHomogeneousObjectDictValue:
//
//	{"k": ba.get()} | {"j": ba.get()} / {} | {"k": ba.get()}
//	dict(k=ba.get()) | {"j": a}
//
// Enables ({"k": ba.get()} | {"j": ba.get()})["j"].run() under foreign same-leaf
// (Class()-only peels live in pythonDictMergeValueType). Empty dict is a
// wildcard. Mixed leaves / non-object-dict arms fail closed.
func pythonObjectDictMergeValueType(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil || n.Type() != "binary_operator" {
		return ""
	}
	op := ingest.ChildByField(n, "operator")
	if op == nil || ingest.NodeText(op, content) != "|" {
		return ""
	}
	left := ingest.ChildByField(n, "left")
	right := ingest.ChildByField(n, "right")
	if left == nil || right == nil {
		return ""
	}
	etL := pythonObjectDictMergeArmValueType(left, content, funcReturns, typeOf, fieldOf)
	etR := pythonObjectDictMergeArmValueType(right, content, funcReturns, typeOf, fieldOf)
	wildL := pythonDictMergeArmWildcard(left, content)
	wildR := pythonDictMergeArmWildcard(right, content)
	if etL != "" && etR != "" {
		if etL == etR {
			return etL
		}
		return ""
	}
	if etL != "" && wildR {
		return etL
	}
	if etR != "" && wildL {
		return etR
	}
	return ""
}

// pythonObjectDictMergeArmValueType types one | arm as an object-dict value leaf:
// nested |, object-dict ctor/literal, or "".
func pythonObjectDictMergeArmValueType(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if n.Type() == "binary_operator" {
		return pythonObjectDictMergeValueType(n, content, funcReturns, typeOf, fieldOf)
	}
	if typeOf != nil && n.Type() == "identifier" {
		// Typed local of Class leaf is a scalar value, not a mapping arm.
		return ""
	}
	if et := pythonHomogeneousObjectDictValue(n, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	return ""
}

// pythonHomogeneousObjectElem recovers T from [ba.get()] / (ba.get(),) / [a] /
// [*[ba.get()]] / [ba.get(), *[ba.get()]] when every element peels to the same
// Class leaf via Class() ctor, method/factory return, typed local, parenthesized/
// walrus/ternary forms, or list_splat of an object collection of the same T.
// Enables [ba.get()][0].run() / [*[ba.get()]][0].run() / xs = [ba.get()];
// xs[0].run() under foreign same-leaf (pythonHomogeneousCtorElem only peels
// Class() identifiers; pythonHomogeneousSplatListCtorElem only peels Class()
// and typed-local splats). Mixed leaves / empty / non-object elements fail closed.
func pythonHomogeneousObjectElem(collection *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
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
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "[", "]", "(", ")", "{", "}", ",", "comment":
			continue
		case "list_splat":
			// *[ba.get()] / *([ba.get()] + [ba.get()]) — peel collection elem of splat arg.
			var val *grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				c := ch.Child(j)
				if c.Type() == "*" {
					continue
				}
				val = c
				break
			}
			if val == nil {
				return ""
			}
			if pythonIsEmptyCollectionLiteral(val) {
				// *[] is a wildcard (no type introduced).
				continue
			}
			t := pythonObjectCollectionElem(val, content, funcReturns, typeOf, fieldOf)
			if t == "" {
				return ""
			}
			if !saw {
				elem = t
				saw = true
				continue
			}
			if t != elem {
				return ""
			}
		default:
			t := pythonObjectLeafType(ch, content, funcReturns, typeOf, fieldOf)
			if t == "" {
				return ""
			}
			if !saw {
				elem = t
				saw = true
				continue
			}
			if t != elem {
				return ""
			}
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// pythonHomogeneousObjectDictValue recovers T from mapping constructors whose
// values peel to the same Class leaf via pythonObjectLeafType:
//
//	{"k": ba.get()} / {"k": ba.get(), "j": a}
//	dict(k=ba.get()) / OrderedDict(k=ba.get()) / UserDict(k=ba.get())
//	dict({"k": ba.get()}) / dict([("k", ba.get())])
//	{k: ba.get() for k in ...}
//
// Enables {"k": ba.get()}["k"].run() / {"k": ba.get()}.get("k").run() /
// dict(k=ba.get())["k"].run() / da = dict(k=ba.get()); da["k"].run() /
// da = {k: ba.get() for k in ...}; da["k"].run() under foreign same-leaf
// (Class()-only peels live in pythonHomogeneousDictValueCtorElem).
// Parenthesized / walrus wrappers accepted. Empty / mixed / splat / non-pair
// fail closed. Call after nested dict peels on assign so {"k": frozenset([A()])}
// / dict(k=frozenset([A()])) is not bound as scalar "frozenset".
func pythonHomogeneousObjectDictValue(collection *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	for collection != nil && !collection.IsNull() {
		switch collection.Type() {
		case "parenthesized_expression":
			collection = pythonParenInner(collection)
		case "named_expression":
			collection = ingest.ChildByField(collection, "value")
		default:
			goto peeled
		}
	}
peeled:
	if collection == nil || collection.IsNull() {
		return ""
	}
	switch collection.Type() {
	case "dictionary":
		return pythonHomogeneousObjectDictLiteralValue(collection, content, funcReturns, typeOf, fieldOf)
	case "dictionary_comprehension":
		return pythonHomogeneousObjectDictCompValue(collection, content, funcReturns, typeOf, fieldOf)
	case "call":
		return pythonHomogeneousObjectDictCallValue(collection, content, funcReturns, typeOf, fieldOf)
	default:
		return ""
	}
}

// pythonHomogeneousObjectDictCompValue recovers T from
// `{k: ba.get() for k in ...}` when the pair value peels to a Class leaf via
// pythonObjectLeafType. Nested fors / non-object values fail closed (mirrors
// Class-only pythonHomogeneousDictCompValueCtorElem).
func pythonHomogeneousObjectDictCompValue(comp *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if comp == nil || comp.Type() != "dictionary_comprehension" {
		return ""
	}
	forCount := 0
	var pair *grammar.Node
	for i := uint32(0); i < comp.ChildCount(); i++ {
		ch := comp.Child(i)
		switch ch.Type() {
		case "for_in_clause":
			forCount++
		case "pair":
			if pair == nil {
				pair = ch
			}
		}
	}
	// Nested fors fail closed (value type still recoverable, but keep product-solid).
	if forCount != 1 || pair == nil {
		return ""
	}
	return pythonObjectLeafType(ingest.ChildByField(pair, "value"), content, funcReturns, typeOf, fieldOf)
}

// pythonHomogeneousObjectDictLiteralValue recovers T from {"k": ba.get()} when
// every pair value peels to the same Class leaf.
func pythonHomogeneousObjectDictLiteralValue(collection *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if collection == nil || collection.Type() != "dictionary" {
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < collection.ChildCount(); i++ {
		ch := collection.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "{", "}", ",", "comment":
			continue
		case "pair":
			val := ingest.ChildByField(ch, "value")
			if val == nil {
				return ""
			}
			t := pythonObjectLeafType(val, content, funcReturns, typeOf, fieldOf)
			if t == "" {
				return ""
			}
			if !saw {
				elem = t
				saw = true
				continue
			}
			if t != elem {
				return ""
			}
		default:
			// Splat / comprehension / unknown — fail closed.
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// pythonHomogeneousObjectDictCallValue recovers T from dict(k=ba.get()) /
// OrderedDict(k=ba.get()) / UserDict(k=ba.get()) / dict({"k": ba.get()}) /
// dict([("k", ba.get())]) / ChainMap({"k": ba.get()}) when every value peels
// to the same Class leaf.
func pythonHomogeneousObjectDictCallValue(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	name := pythonSimpleCalleeName(ingest.ChildByField(call, "function"), content)
	switch name {
	case "dict", "OrderedDict", "UserDict":
		// ok
	case "ChainMap":
		// ChainMap({"k": ba.get()}) / ChainMap(dict(k=ba.get()), {"m": a}) —
		// every positional map peels to the same object-dict value leaf.
		return pythonHomogeneousObjectChainMapValue(call, content, funcReturns, typeOf, fieldOf)
	default:
		return ""
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	var positionals []*grammar.Node
	var keywords []*grammar.Node
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument":
			keywords = append(keywords, ch)
		case "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		default:
			positionals = append(positionals, ch)
		}
	}
	// All-keyword form: dict(k=ba.get()) / OrderedDict(k=ba.get(), m=a)
	if len(positionals) == 0 {
		if len(keywords) == 0 {
			return ""
		}
		var elem string
		saw := false
		for _, kw := range keywords {
			et := pythonObjectLeafType(ingest.ChildByField(kw, "value"), content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
				return ""
			}
		}
		if !saw {
			return ""
		}
		return elem
	}
	// Single positional only (no kwargs): dict({"k": ba.get()}) / dict([("k", ba.get())])
	if len(positionals) != 1 || len(keywords) != 0 {
		return ""
	}
	arg := positionals[0]
	switch arg.Type() {
	case "dictionary":
		return pythonHomogeneousObjectDictLiteralValue(arg, content, funcReturns, typeOf, fieldOf)
	case "list", "tuple":
		return pythonHomogeneousObjectDictPairsValue(arg, content, funcReturns, typeOf, fieldOf)
	default:
		return ""
	}
}

// pythonHomogeneousObjectChainMapValue recovers T from
// ChainMap({"k": ba.get()}) / ChainMap(dict(k=ba.get())) /
// ChainMap(OrderedDict(k=ba.get()), {"m": a}) when every positional map has
// homogeneous object-dict values of the same Class leaf. Dict literals and
// dict/OrderedDict/UserDict (and nested ChainMap) object peels accepted;
// mixed / kwargs / splat / Class()-only shapes that need
// pythonHomogeneousDictValueCtorElem fail closed here (Class() path covers
// those). Enables ChainMap({"k": ba.get()})["k"].run() /
// ChainMap({"k": ba.get()}).get("k").run() / ca = ChainMap(...); ca["k"].run()
// under foreign same-leaf.
func pythonHomogeneousObjectChainMapValue(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument", "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		case "dictionary":
			et := pythonHomogeneousObjectDictLiteralValue(ch, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
				return ""
			}
		case "call":
			// ChainMap(dict(k=ba.get())) / ChainMap(OrderedDict(k=ba.get())) /
			// ChainMap(UserDict(k=ba.get())) / nested ChainMap(...).
			et := pythonHomogeneousObjectDictValue(ch, content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
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

// pythonObjectDictValuesElemType recovers T from dict(k=ba.get()).values() /
// {"k": ba.get()}.values() / ChainMap({"k": ba.get()}).values() /
// OrderedDict(k=ba.get()).values() when the receiver peels as a homogeneous
// object-dict of Class leaf T. Zero-arg values() only. Enables
// for x in dict(k=ba.get()).values(): x.run() /
// next(iter(dict(k=ba.get()).values())).run() under foreign same-leaf.
func pythonObjectDictValuesElemType(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	for n != nil && !n.IsNull() {
		switch n.Type() {
		case "parenthesized_expression":
			n = pythonParenInner(n)
		case "named_expression":
			n = ingest.ChildByField(n, "value")
		default:
			goto peeled
		}
	}
peeled:
	if n == nil || n.IsNull() || n.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(n, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "values" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(n)
	if !ok || len(args) != 0 {
		return ""
	}
	obj := ingest.ChildByField(fn, "object")
	if et := pythonHomogeneousObjectDictValue(obj, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// dict.fromkeys(keys, ba.get()).values() — value leaf under foreign same-leaf.
	if et := pythonDictFromkeysObjectValueType(obj, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// MappingProxyType({"k": ba.get()}).values() — proxy value leaf.
	return pythonMappingProxyObjectValueType(obj, content, nil, funcReturns, typeOf, fieldOf)
}

// pythonObjectIterableElemType recovers T from object-collection / object-dict
// values iterables used under next/min/for when pythonIterableElemType only
// peels Class() / typed locals:
//
//	[ba.get()] / iter([ba.get()]) / list([ba.get()])
//	dict(k=ba.get()).values() / {"k": ba.get()}.values() /
//	ChainMap({"k": ba.get()}).values()
//	list(dict(k=ba.get()).values()) / iter(...values())
//	repeat(ba.get()) / itertools.repeat(ba.get()[, times])
//
// Enables next(iter([ba.get()])).run() / min([ba.get()], key=...).run() /
// for x in dict(k=ba.get()).values(): x.run() /
// for x in cycle([ba.get()]): x.run() / next(x for x in [ba.get()]).run() /
// next(repeat(ba.get())).run() / for x in repeat(ba.get()): x.run()
// under foreign same-leaf.
func pythonObjectIterableElemType(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	for n != nil && !n.IsNull() {
		switch n.Type() {
		case "parenthesized_expression":
			n = pythonParenInner(n)
		case "named_expression":
			n = ingest.ChildByField(n, "value")
		default:
			goto peeled
		}
	}
peeled:
	if n == nil || n.IsNull() {
		return ""
	}
	// [ba.get()] / iter([ba.get()]) / list([ba.get()] + [ba.get()]) / etc.
	if et := pythonObjectCollectionElem(n, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// dict(k=ba.get()).values() / {"k": ba.get()}.values() / ChainMap(...).values()
	if et := pythonObjectDictValuesElemType(n, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// next(x for x in [ba.get()]) / for a in [x for x in [ba.get()]] —
	// identity comprehension of object iterable.
	if et := pythonObjectComprehensionElemType(n, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	// list/iter/reversed/... of an object-dict values view (objectCollection
	// peels list of method-return elems but not list(dict(...).values())).
	if n.Type() == "call" {
		fn := ingest.ChildByField(n, "function")
		if fn != nil && fn.Type() == "identifier" {
			switch ingest.NodeText(fn, content) {
			case "list", "tuple", "set", "frozenset", "iter", "reversed", "sorted", "deque", "Counter":
				// Counter([ba.get()]) — keys are method-return elements (same as
				// Class Counter peels under foreign same-leaf).
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) == 0 {
					return ""
				}
				return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
			case "filter":
				// filter(pred, [ba.get()]) — iterable is 2nd arg.
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) < 2 {
					return ""
				}
				return pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf)
			case "map":
				// map(lambda x: x, [ba.get()]) / map(A, …) identity/class handled
				// in Class() path; object peels identity map iterable.
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) < 2 {
					return ""
				}
				if pythonIsIdentityLambda(args[0], content) {
					return pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf)
				}
				return ""
			case "nlargest", "nsmallest":
				// nlargest(n, [ba.get()], key=…) — element of 2nd arg.
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) < 2 {
					return ""
				}
				return pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf)
			case "cycle", "islice", "accumulate", "compress":
				// cycle([ba.get()]) / islice([ba.get()], n) — 1st arg element.
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) == 0 {
					return ""
				}
				return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
			case "repeat":
				// repeat(ba.get()[, times]) — yields the object (not an iterable of it).
				// times ignored. Class()/typed-local peels via pythonIterableElemType.
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) == 0 {
					return ""
				}
				return pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf)
			case "chain", "merge":
				// chain([ba.get()], [ba.get()]) — shared object element when all agree.
				return pythonObjectChainElemType(n, content, funcReturns, typeOf, fieldOf)
			case "takewhile", "dropwhile", "filterfalse":
				// takewhile(pred, [ba.get()]) — 2nd arg element.
				args, ok := pythonCallPositionalArgNodes(n)
				if !ok || len(args) < 2 {
					return ""
				}
				return pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf)
			}
		}
		// heapq.nlargest / itertools.cycle / itertools.chain / ...
		if fn != nil && fn.Type() == "attribute" {
			attr := ingest.ChildByField(fn, "attribute")
			if attr != nil {
				switch ingest.NodeText(attr, content) {
				case "nlargest", "nsmallest":
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) < 2 {
						return ""
					}
					return pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf)
				case "cycle", "islice", "accumulate", "compress":
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
				case "repeat":
					// itertools.repeat(ba.get()[, times]) — object leaf of 1st arg.
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf)
				case "chain", "merge":
					return pythonObjectChainElemType(n, content, funcReturns, typeOf, fieldOf)
				case "from_iterable":
					// chain.from_iterable([[ba.get()]]) /
					// itertools.chain.from_iterable([[ba.get()]]) — flatten one level.
					objN := ingest.ChildByField(fn, "object")
					if !pythonIsChainReceiver(objN, content) {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) != 1 {
						return ""
					}
					return pythonObjectChainFromIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
				case "takewhile", "dropwhile", "filterfalse":
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) < 2 {
						return ""
					}
					return pythonObjectIterableElemType(args[1], content, funcReturns, typeOf, fieldOf)
				case "Counter", "deque":
					// collections.Counter([ba.get()]) / collections.deque([ba.get()]) —
					// method-return elements under foreign same-leaf.
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" || ingest.NodeText(objN, content) != "collections" {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
				case "fromkeys":
					// dict.fromkeys([ba.get()]) / dict.fromkeys([ba.get()], v) —
					// iteration yields keys (1st-arg elements); value arg ignored.
					// Enables list(dict.fromkeys([ba.get()]))[0] under foreign same-leaf
					// (Class peels via pythonIterableElemType fromkeys).
					objN := ingest.ChildByField(fn, "object")
					if objN == nil || objN.Type() != "identifier" || ingest.NodeText(objN, content) != "dict" {
						return ""
					}
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) == 0 {
						return ""
					}
					return pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
				case "elements":
					// Counter([ba.get()]).elements() / collections.Counter(...).elements() —
					// zero-arg; keys of Counter (same leaf as iterating Counter).
					args, ok := pythonCallPositionalArgNodes(n)
					if !ok || len(args) != 0 {
						return ""
					}
					objN := ingest.ChildByField(fn, "object")
					return pythonObjectIterableElemType(objN, content, funcReturns, typeOf, fieldOf)
				}
			}
		}
	}
	return ""
}

// pythonObjectChainFromIterableElemType recovers T from
// chain.from_iterable([[ba.get()]]) / chain.from_iterable([(ba.get(),)]) when
// the sole arg is a list/tuple of object iterables that agree on element T.
// Mirrors pythonChainFromIterableElemType for method-return dual-class peels.
func pythonObjectChainFromIterableElemType(arg *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
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
			t := pythonObjectIterableElemType(ch, content, funcReturns, typeOf, fieldOf)
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

// pythonObjectComprehensionElemType recovers T from genexp/list/set comps over
// object leaves under foreign same-leaf:
//
//   - Identity over object iterables: x for x in [ba.get()] / [x for x in [ba.get()]]
//   - Method-return / Class() body: [ba.get() for _ in range(1)] /
//     next(ba.get() for _ in xs) — body peels via pythonObjectLeafType
//
// Same single-for rules as pythonComprehensionElemType. Nested fors and other
// transforming bodies fail closed. if_clause is ignored.
func pythonObjectComprehensionElemType(comp *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
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
	if body == nil {
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
	if forCount != 1 || forClause == nil {
		return ""
	}
	left := ingest.ChildByField(forClause, "left")
	right := ingest.ChildByField(forClause, "right")
	if left == nil || right == nil {
		return ""
	}
	// Identity: body name matches for-target over an object iterable.
	if body.Type() == "identifier" && left.Type() == "identifier" &&
		ingest.NodeText(body, content) == ingest.NodeText(left, content) {
		return pythonObjectIterableElemType(right, content, funcReturns, typeOf, fieldOf)
	}
	// Body is method-return / Class() / typed-local leaf
	// (`[ba.get() for _ in range(1)]` / next(ba.get() for _ in xs)).
	return pythonObjectLeafType(body, content, funcReturns, typeOf, fieldOf)
}

// pythonObjectChainElemType recovers shared T from chain/merge of object
// iterables: chain([ba.get()], [ba.get()]) — all args agree; any untyped fails.
func pythonObjectChainElemType(call *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) == 0 {
		return ""
	}
	found := ""
	for _, a := range args {
		et := pythonObjectIterableElemType(a, content, funcReturns, typeOf, fieldOf)
		if et == "" {
			return ""
		}
		if found == "" {
			found = et
		} else if found != et {
			return ""
		}
	}
	return found
}

// pythonObjectEnumerateZipTargetTypes recovers per-slot types for
// enumerate([ba.get()]) → ["", "A"] and zip([ba.get()], …) when args peel as
// object iterables under foreign same-leaf (Class() peels via
// pythonEnumerateZipTargetTypes).
func pythonObjectEnumerateZipTargetTypes(right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) []string {
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
		et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
		if et == "" {
			return nil
		}
		return []string{"", et}
	case "zip", "zip_longest", "product":
		if len(args) == 0 {
			return nil
		}
		out := make([]string, len(args))
		any := false
		for i, a := range args {
			et := pythonObjectIterableElemType(a, content, funcReturns, typeOf, fieldOf)
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
		if len(args) == 0 {
			return nil
		}
		et := pythonObjectIterableElemType(args[0], content, funcReturns, typeOf, fieldOf)
		if et == "" {
			return nil
		}
		return []string{et, et}
	default:
		return nil
	}
}

// pythonHomogeneousObjectDictPairsValue recovers T from [("k", ba.get())] /
// (("k", ba.get()),) when every pair value peels to the same Class leaf.
func pythonHomogeneousObjectDictPairsValue(pairs *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	if pairs == nil {
		return ""
	}
	switch pairs.Type() {
	case "list", "tuple":
		// ok
	default:
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < pairs.ChildCount(); i++ {
		ch := pairs.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		case "list", "tuple":
			var elems []*grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				el := ch.Child(j)
				switch el.Type() {
				case "[", "]", "(", ")", ",", "comment":
					continue
				default:
					elems = append(elems, el)
				}
			}
			if len(elems) != 2 {
				return ""
			}
			et := pythonObjectLeafType(elems[1], content, funcReturns, typeOf, fieldOf)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
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

// pythonObjectLeafType recovers the Class leaf of a value expression used as a
// list/tuple element or similar object peel: typed local identifier, Class()
// ctor, method/factory return (ba.get() → A), parenthesized / walrus wrappers,
// or conditional when both arms agree. Other shapes fail closed.
func pythonObjectLeafType(n *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) string {
	for n != nil && !n.IsNull() {
		switch n.Type() {
		case "parenthesized_expression":
			n = pythonParenInner(n)
		case "named_expression":
			n = ingest.ChildByField(n, "value")
		default:
			goto peeled
		}
	}
peeled:
	if n == nil || n.IsNull() {
		return ""
	}
	switch n.Type() {
	case "identifier":
		if typeOf == nil {
			return ""
		}
		return typeOf[ingest.NodeText(n, content)]
	case "call":
		if ct := pythonClassCtorName(n, content); ct != "" {
			return ct
		}
		return pythonCallFuncReturnType(n, content, funcReturns, typeOf, fieldOf)
	case "conditional_expression":
		body, alt := pythonConditionalArms(n)
		t1 := pythonObjectLeafType(body, content, funcReturns, typeOf, fieldOf)
		t2 := pythonObjectLeafType(alt, content, funcReturns, typeOf, fieldOf)
		if t1 != "" && t1 == t2 {
			return t1
		}
		return ""
	}
	return ""
}

// pythonNestedHomogeneousCtorElem recovers T from one-level nested list/tuple
// literals of homogeneous Class() rows (or dict-of-Class rows):
//
//	[[A()]] / [[A()], [A()]] / ((A(),),) / ([A()],) → "A"
//	[{"k": A()}] / [{"k": A()}, {"m": A()}] / ({"k": A()},) → "A"
//
// Stored as elemOf["@nested."+name] so aa = [[A()]]; aa[0][0].run() /
// la = [{"k": A()}]; la[0]["k"].run() / match aa: case [[xa]]: / case [row]:
// row[0].run() peel under foreign same-leaf (same leaf as annotated list[list[A]]
// / list[dict[str, A]]). Mixed row leaves / deeper nests / sets of lists /
// non-list/dict rows fail closed.
func pythonNestedHomogeneousCtorElem(collection *grammar.Node, content []byte) string {
	if collection == nil {
		return ""
	}
	switch collection.Type() {
	case "list", "tuple":
		// ok — one-level nest of rows (not set of lists; product fails closed).
	default:
		return ""
	}
	var nest string
	saw := false
	for i := uint32(0); i < collection.ChildCount(); i++ {
		ch := collection.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		case "list", "tuple":
			// Row must itself be a homogeneous Class() list/tuple of T.
			et := pythonHomogeneousCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				nest = et
				saw = true
				continue
			}
			if et != nest {
				return ""
			}
		case "dictionary":
			// Row is a mapping of Class() values: {"k": A()} (list-of-dict).
			et := pythonHomogeneousDictLiteralValueCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				nest = et
				saw = true
				continue
			}
			if et != nest {
				return ""
			}
		default:
			// Nested non-list / Class() at outer level → not a nest of rows.
			return ""
		}
	}
	if !saw {
		return ""
	}
	return nest
}

// pythonHomogeneousDictValueCtorElem recovers T from mapping constructors whose
// values are homogeneous Class() instances (scalar mapping values, not nested
// collections):
//
//	{"k": A()} / {"k": A(), "m": A()} → "A"
//	dict(k=A()) / OrderedDict(k=A()) / dict({"k": A()}) / OrderedDict([("k", A())]) → "A"
//	ChainMap({"k": A()}) / ChainMap({"k": A()}, {"m": A()}) → "A"
//	{k: A() for k in ...} → "A"
//
// Stored as elemOf[name] so da["k"].run() / for a in da.values() peel under
// foreign same-leaf (same leaf as annotated dict[str, A]). Nested
// {"k": [A()]} / dict(k=[A()]) stays on @nested paths; mixed leaves / empty /
// splat fail closed.
func pythonHomogeneousDictValueCtorElem(collection *grammar.Node, content []byte) string {
	if collection == nil {
		return ""
	}
	switch collection.Type() {
	case "dictionary":
		return pythonHomogeneousDictLiteralValueCtorElem(collection, content)
	case "dictionary_comprehension":
		return pythonHomogeneousDictCompValueCtorElem(collection, content)
	case "call":
		return pythonHomogeneousDictCallValueCtorElem(collection, content)
	default:
		return ""
	}
}

// pythonDictStarCopyValueType recovers T from a dictionary star-copy expression
// whose every arm peels to the same mapping value type T:
//
//	{**da} / {**da, **db} when da/db are typed mapping locals (elemOf)
//	{**da, "k": A()} / {**da, "k": A(), **db} when Class() values agree with stars
//
// Enables {**da}["k"].run() / db = {**da}; db["k"].run() under foreign same-leaf
// (same leaf as da.copy()["k"] / da["k"]). Nested-only mappings (@nested without
// scalar elemOf), empty dicts, mixed leaves, and non-identifier splats fail closed.
// Nested dict[str, list[A]] star-copy uses pythonDictStarCopyNestedValueType.
func pythonDictStarCopyValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil || n.Type() != "dictionary" || elemOf == nil {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "{", "}", ",", "comment":
			continue
		case "dictionary_splat":
			name := pythonDictSplatIdent(ch, content)
			if name == "" {
				return ""
			}
			// Scalar mapping values only. Nested list values use @nested and are
			// not a scalar value leaf for {**da}["k"].run().
			et := elemOf[name]
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		case "pair":
			val := ingest.ChildByField(ch, "value")
			et := pythonClassCtorName(val, content)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		default:
			// Comprehension / unknown — fail closed.
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// pythonDictSplatIdent returns the identifier name inside a dictionary_splat
// node (**da). Non-identifier splats fail closed ("").
func pythonDictSplatIdent(ch *grammar.Node, content []byte) string {
	if ch == nil || ch.Type() != "dictionary_splat" {
		return ""
	}
	var arg *grammar.Node
	for j := uint32(0); j < ch.ChildCount(); j++ {
		c := ch.Child(j)
		if c == nil || c.Type() == "**" {
			continue
		}
		arg = c
		break
	}
	if arg == nil || arg.Type() != "identifier" {
		return ""
	}
	return ingest.NodeText(arg, content)
}

// pythonDictStarCopyNestedValueType recovers T from a dictionary star-copy whose
// every arm peels to the same nested mapping-of-list/set leaf T (@nested):
//
//	{**da} / {**da, **ea} when da/ea are dict[str, list[A]] / nested literals
//	{**da, "j": [A()]} when pair values are homogeneous list/set of A
//
// Enables {**da}["k"][0].run() / ca = {**da}; ca["k"][0].run() /
// for a in {**da}["k"] / {**da}.get("k")[0].run() under foreign same-leaf
// (same leaf as da["k"][0] / annotated nested). Scalar dict[str, A] stays on
// pythonDictStarCopyValueType. Empty / mixed leaves / non-ident splats fail closed.
func pythonDictStarCopyNestedValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil || n.Type() != "dictionary" || elemOf == nil {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "{", "}", ",", "comment":
			continue
		case "dictionary_splat":
			name := pythonDictSplatIdent(ch, content)
			if name == "" {
				return ""
			}
			et := elemOf["@nested."+name]
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		case "pair":
			val := ingest.ChildByField(ch, "value")
			et := pythonNestedDictValueCollectionElem(val, content)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		default:
			return ""
		}
	}
	if !saw {
		return ""
	}
	return found
}

// pythonDictMergeValueType recovers T from a dict merge expression (PEP 584 `|`)
// whose every arm peels to the same scalar mapping value type T:
//
//	da | ea / da | {"j": A()} / {} | {"k": A()} / da | ea | {"m": A()}
//
// Empty dict literals are wildcards (same as list-concat empty arms). Enables
// (da | ea)["j"].run() / ca = da | ea; ca["j"].run() under foreign same-leaf
// (same leaf as da["k"] / {**da}["k"]). Nested dict[str, list[A]] merges use
// pythonDictMergeNestedValueType. Mixed leaves / non-mapping arms fail closed.
func pythonDictMergeValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil || n.Type() != "binary_operator" {
		return ""
	}
	op := ingest.ChildByField(n, "operator")
	if op == nil || ingest.NodeText(op, content) != "|" {
		return ""
	}
	left := ingest.ChildByField(n, "left")
	right := ingest.ChildByField(n, "right")
	if left == nil || right == nil {
		return ""
	}
	etL := pythonDictMergeArmValueType(left, content, elemOf)
	etR := pythonDictMergeArmValueType(right, content, elemOf)
	wildL := pythonDictMergeArmWildcard(left, content)
	wildR := pythonDictMergeArmWildcard(right, content)
	if etL != "" && etR != "" {
		if etL == etR {
			return etL
		}
		return ""
	}
	if etL != "" && wildR {
		return etL
	}
	if etR != "" && wildL {
		return etR
	}
	return ""
}

// pythonDictMergeArmValueType types one | arm as a scalar mapping value leaf:
// nested |, typed mapping local, homogeneous dict ctor, star-copy, or "".
func pythonDictMergeArmValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if n.Type() == "binary_operator" {
		return pythonDictMergeValueType(n, content, elemOf)
	}
	if n.Type() == "identifier" && elemOf != nil {
		// Scalar only — nested list values live under @nested and are not a
		// scalar value leaf for (da | ea)["k"].run().
		name := ingest.NodeText(n, content)
		if elemOf["@nested."+name] != "" {
			return ""
		}
		return elemOf[name]
	}
	if et := pythonHomogeneousDictValueCtorElem(n, content); et != "" {
		return et
	}
	if et := pythonDictStarCopyValueType(n, content, elemOf); et != "" {
		return et
	}
	return ""
}

// pythonDictMergeArmWildcard reports empty dictionary literals ({} is a merge
// identity for value-type peels).
func pythonDictMergeArmWildcard(n *grammar.Node, content []byte) bool {
	if n == nil {
		return false
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil || n.Type() != "dictionary" {
		return false
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "{", "}", ",", "comment":
			continue
		default:
			return false
		}
	}
	return true
}

// pythonDictMergeNestedValueType recovers T from a dict merge expression whose
// every arm peels to the same nested mapping-of-list/set leaf T (@nested):
//
//	da | ea / da | {"j": [A()]} / {} | {"k": [A()]} when da/ea are nested
//
// Enables (da | ea)["j"][0].run() / ca = da | ea; ca["j"][0].run() under foreign
// same-leaf. Scalar dict[str, A] stays on pythonDictMergeValueType. Empty dict
// arms are wildcards; mixed leaves fail closed.
func pythonDictMergeNestedValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil || n.Type() != "binary_operator" {
		return ""
	}
	op := ingest.ChildByField(n, "operator")
	if op == nil || ingest.NodeText(op, content) != "|" {
		return ""
	}
	left := ingest.ChildByField(n, "left")
	right := ingest.ChildByField(n, "right")
	if left == nil || right == nil {
		return ""
	}
	etL := pythonDictMergeArmNestedValueType(left, content, elemOf)
	etR := pythonDictMergeArmNestedValueType(right, content, elemOf)
	wildL := pythonDictMergeArmWildcard(left, content)
	wildR := pythonDictMergeArmWildcard(right, content)
	if etL != "" && etR != "" {
		if etL == etR {
			return etL
		}
		return ""
	}
	if etL != "" && wildR {
		return etL
	}
	if etR != "" && wildL {
		return etR
	}
	return ""
}

// pythonDictMergeArmNestedValueType types one | arm as a nested mapping leaf.
func pythonDictMergeArmNestedValueType(n *grammar.Node, content []byte, elemOf map[string]string) string {
	if n == nil {
		return ""
	}
	for n != nil && n.Type() == "parenthesized_expression" {
		n = pythonParenInner(n)
	}
	if n == nil {
		return ""
	}
	if n.Type() == "binary_operator" {
		return pythonDictMergeNestedValueType(n, content, elemOf)
	}
	if n.Type() == "identifier" && elemOf != nil {
		return elemOf["@nested."+ingest.NodeText(n, content)]
	}
	if et := pythonNestedDictHomogeneousListCtorElem(n, content); et != "" {
		return et
	}
	if et := pythonDictStarCopyNestedValueType(n, content, elemOf); et != "" {
		return et
	}
	return ""
}

// pythonChainMapMapsIndexValueType recovers T from ChainMap(da).maps[i] /
// ca.maps[i] / ChainMap({"k": A()}).maps[0] when the ChainMap expression has
// scalar mapping value leaf T. ChainMap.maps is list[Mapping]; indexing yields
// a mapping of T (same leaf as ChainMap(da) / ca), so maps[0]["k"].run() /
// maps[0].get("k").run() / ma = maps[0]; ma["k"].run() peel under foreign
// same-leaf. Nested list-value maps / non-.maps receivers / slices fail closed.
func pythonChainMapMapsIndexValueType(sub *grammar.Node, content []byte, elemOf map[string]string) string {
	if sub == nil || sub.Type() != "subscript" {
		return ""
	}
	for i := uint32(0); i < sub.ChildCount(); i++ {
		if sub.Child(i).Type() == "slice" {
			return ""
		}
	}
	val := ingest.ChildByField(sub, "value")
	for val != nil && val.Type() == "parenthesized_expression" {
		val = pythonParenInner(val)
	}
	if val == nil || val.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(val, "attribute")
	if attr == nil || ingest.NodeText(attr, content) != "maps" {
		return ""
	}
	obj := ingest.ChildByField(val, "object")
	return pythonChainMapExprScalarValueType(obj, content, elemOf)
}

// pythonChainMapLocalValueType recovers T from ChainMap(da[, ea...]) /
// collections.ChainMap(da) when every positional is a typed scalar mapping
// local (elemOf) of the same value leaf T. Enables ChainMap(da)["k"].run() /
// ca = ChainMap(da); ca["k"].run() under foreign same-leaf. Nested list values
// use pythonChainMapLocalNestedValueType. Literal / ctor maps stay on
// pythonHomogeneousChainMapValueCtorElem. Empty / mixed / splat fail closed.
func pythonChainMapLocalValueType(call *grammar.Node, content []byte, elemOf map[string]string) string {
	if call == nil || call.Type() != "call" || elemOf == nil {
		return ""
	}
	if pythonSimpleCalleeName(ingest.ChildByField(call, "function"), content) != "ChainMap" {
		return ""
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument", "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		case "identifier":
			name := ingest.NodeText(ch, content)
			if elemOf["@nested."+name] != "" {
				// Nested mapping — not a scalar value leaf.
				return ""
			}
			et := elemOf[name]
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		default:
			// Mix of locals + literals: peel literal arms too when they agree.
			et := pythonHomogeneousDictValueCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		}
	}
	if !saw {
		return ""
	}
	return found
}

// pythonMappingProxyValueType recovers T from MappingProxyType(da) /
// types.MappingProxyType(da) when the sole positional is a typed scalar mapping
// local (elemOf), a homogeneous dict literal of Class() values, or
// vars(SimpleNamespace(...)) / SimpleNamespace(...).__dict__ with homogeneous
// Class() fields. Enables MappingProxyType(da)["k"].run() /
// pa = MappingProxyType(da); pa["k"].run() / MappingProxyType({"k": A()})["k"].run() /
// MappingProxyType(vars(SimpleNamespace(k=A())))["k"].run() under foreign same-leaf.
// Nested list values / multi-arg / kwargs / splat fail closed.
func pythonMappingProxyValueType(call *grammar.Node, content []byte, elemOf map[string]string) string {
	return pythonMappingProxyObjectValueType(call, content, elemOf, nil, nil, nil)
}

// pythonMappingProxyObjectValueType peels MappingProxyType arg as Class() or
// method-return / typed-local object-dict value leaf (ba.get() → A) under
// foreign same-leaf. Same shapes as pythonMappingProxyValueType plus
// MappingProxyType({"k": ba.get()}) / MappingProxyType(dict(k=ba.get())).
func pythonMappingProxyObjectValueType(call *grammar.Node, content []byte, elemOf, funcReturns, typeOf, fieldOf map[string]string) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	if pythonSimpleCalleeName(ingest.ChildByField(call, "function"), content) != "MappingProxyType" {
		return ""
	}
	// types.MappingProxyType — require module ident "types" when attribute form.
	fn := ingest.ChildByField(call, "function")
	if fn != nil && fn.Type() == "attribute" {
		obj := ingest.ChildByField(fn, "object")
		if obj == nil || obj.Type() != "identifier" || ingest.NodeText(obj, content) != "types" {
			return ""
		}
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 1 {
		return ""
	}
	arg := args[0]
	if arg.Type() == "identifier" {
		if elemOf == nil {
			return ""
		}
		name := ingest.NodeText(arg, content)
		if elemOf["@nested."+name] != "" {
			return ""
		}
		return elemOf[name]
	}
	// MappingProxyType(vars(SimpleNamespace(k=A()))) / MappingProxyType(SNS(...).__dict__)
	// — homogeneous Class() fields (same leaf as vars(SNS).values()).
	if t := pythonInlineSimpleNamespaceDictViewHomogeneousType(arg, content); t != "" {
		return t
	}
	// MappingProxyType({"k": ba.get()}) / MappingProxyType({"k": A()}) —
	// homogeneous object-dict / Class() values under foreign same-leaf.
	if et := pythonHomogeneousObjectDictValue(arg, content, funcReturns, typeOf, fieldOf); et != "" {
		return et
	}
	return pythonHomogeneousDictValueCtorElem(arg, content)
}

// pythonChainMapLocalNestedValueType recovers T from ChainMap(da[, ea...]) when
// every positional is a nested mapping local (@nested) or nested dict ctor of
// the same list/set leaf T. Enables ChainMap(da)["k"][0].run() /
// ca = ChainMap(da); ca["k"][0].run() under foreign same-leaf.
func pythonChainMapLocalNestedValueType(call *grammar.Node, content []byte, elemOf map[string]string) string {
	if call == nil || call.Type() != "call" || elemOf == nil {
		return ""
	}
	if pythonSimpleCalleeName(ingest.ChildByField(call, "function"), content) != "ChainMap" {
		return ""
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument", "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		case "identifier":
			et := elemOf["@nested."+ingest.NodeText(ch, content)]
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		default:
			et := pythonNestedDictHomogeneousListCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				found = et
				saw = true
			} else if found != et {
				return ""
			}
		}
	}
	if !saw {
		return ""
	}
	return found
}

// pythonNestedMappingCopyCallType recovers T from da.copy() / da.deepcopy() /
// {**da}.copy() when the receiver is a nested mapping of list/set of T.
// Zero-arg only; other receivers / arity fail closed.
func pythonNestedMappingCopyCallType(call *grammar.Node, content []byte, elemOf map[string]string) string {
	if call == nil || call.Type() != "call" || elemOf == nil {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil {
		return ""
	}
	method := ingest.NodeText(attr, content)
	if method != "copy" && method != "deepcopy" {
		return ""
	}
	// copy.copy(x) module form is not a nested mapping copy of x's @nested leaf
	// via attribute on the mapping — handled elsewhere for scalar elemOf.
	if obj.Type() == "identifier" && ingest.NodeText(obj, content) == "copy" {
		return ""
	}
	args, ok := pythonCallPositionalArgNodes(call)
	if !ok || len(args) != 0 {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		return elemOf["@nested."+ingest.NodeText(obj, content)]
	case "dictionary":
		return pythonDictStarCopyNestedValueType(obj, content, elemOf)
	default:
		return ""
	}
}

// pythonHomogeneousDictLiteralValueCtorElem recovers T from a dictionary
// literal whose values are Class() calls of the same T.
func pythonHomogeneousDictLiteralValueCtorElem(collection *grammar.Node, content []byte) string {
	if collection == nil || collection.Type() != "dictionary" {
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < collection.ChildCount(); i++ {
		ch := collection.Child(i)
		switch ch.Type() {
		case "{", "}", ",", "comment":
			continue
		case "pair":
			val := ingest.ChildByField(ch, "value")
			et := pythonClassCtorName(val, content)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
				return ""
			}
		default:
			// Splat / comprehension / unknown — fail closed.
			return ""
		}
	}
	if !saw {
		return ""
	}
	return elem
}

// pythonHomogeneousDictCompValueCtorElem recovers T from
// `{k: A() for k in ...}` when the pair value is a Class() call.
// Nested fors / non-Class values fail closed.
func pythonHomogeneousDictCompValueCtorElem(comp *grammar.Node, content []byte) string {
	if comp == nil || comp.Type() != "dictionary_comprehension" {
		return ""
	}
	forCount := 0
	var pair *grammar.Node
	for i := uint32(0); i < comp.ChildCount(); i++ {
		ch := comp.Child(i)
		switch ch.Type() {
		case "for_in_clause":
			forCount++
		case "pair":
			if pair == nil {
				pair = ch
			}
		}
	}
	// Nested fors fail closed (value type still recoverable, but keep product-solid).
	if forCount != 1 || pair == nil {
		return ""
	}
	return pythonClassCtorName(ingest.ChildByField(pair, "value"), content)
}

// pythonHomogeneousDictCallValueCtorElem recovers T from dict/OrderedDict/
// UserDict/ChainMap constructors whose values are Class() instances (scalar
// mapping values).
func pythonHomogeneousDictCallValueCtorElem(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	name := pythonSimpleCalleeName(fn, content)
	switch name {
	case "dict", "OrderedDict", "UserDict":
		// ok — kwargs / pairs / single dict (same shapes as nested path)
	case "ChainMap":
		// ChainMap(*maps) — each positional is a dict of Class() values.
		return pythonHomogeneousChainMapValueCtorElem(call, content)
	default:
		return ""
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	var positionals []*grammar.Node
	var keywords []*grammar.Node
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument":
			keywords = append(keywords, ch)
		case "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		default:
			positionals = append(positionals, ch)
		}
	}
	// All-keyword form: dict(k=A()) / OrderedDict(k=A(), m=A())
	if len(positionals) == 0 {
		if len(keywords) == 0 {
			return ""
		}
		var elem string
		saw := false
		for _, kw := range keywords {
			et := pythonClassCtorName(ingest.ChildByField(kw, "value"), content)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
				return ""
			}
		}
		if !saw {
			return ""
		}
		return elem
	}
	// Single positional only (no kwargs): dict({"k": A()}) / dict([("k", A())])
	if len(positionals) != 1 || len(keywords) != 0 {
		return ""
	}
	arg := positionals[0]
	switch arg.Type() {
	case "dictionary":
		return pythonHomogeneousDictLiteralValueCtorElem(arg, content)
	case "list", "tuple":
		return pythonHomogeneousDictPairsValueCtorElem(arg, content)
	default:
		return ""
	}
}

// pythonHomogeneousChainMapValueCtorElem recovers T from
// ChainMap({"k": A()}) / ChainMap(OrderedDict(k=A())) /
// ChainMap({"k": A()}, {"m": A()}) when every positional map has homogeneous
// Class() values of the same T. Dict literals and dict/OrderedDict/UserDict
// calls accepted; other shapes fail closed.
func pythonHomogeneousChainMapValueCtorElem(call *grammar.Node, content []byte) string {
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument", "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		case "dictionary":
			et := pythonHomogeneousDictLiteralValueCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
				return ""
			}
		case "call":
			// ChainMap(OrderedDict(k=A())) / ChainMap(dict(k=A())) /
			// ChainMap(UserDict(k=A())) / collections.OrderedDict(...).
			et := pythonHomogeneousDictValueCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
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

// pythonHomogeneousDictPairsValueCtorElem recovers T from a list/tuple of
// key/value pairs where each value is a Class() call of the same T:
//
//	[("k", A())] / (("k", A()),) / [("k", A()), ("m", A())] → "A"
func pythonHomogeneousDictPairsValueCtorElem(pairs *grammar.Node, content []byte) string {
	if pairs == nil {
		return ""
	}
	switch pairs.Type() {
	case "list", "tuple":
		// ok
	default:
		return ""
	}
	var elem string
	saw := false
	for i := uint32(0); i < pairs.ChildCount(); i++ {
		ch := pairs.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		case "list", "tuple":
			var elems []*grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				el := ch.Child(j)
				switch el.Type() {
				case "[", "]", "(", ")", ",", "comment":
					continue
				default:
					elems = append(elems, el)
				}
			}
			if len(elems) != 2 {
				return ""
			}
			et := pythonClassCtorName(elems[1], content)
			if et == "" {
				return ""
			}
			if !saw {
				elem = et
				saw = true
				continue
			}
			if et != elem {
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

// pythonNestedDictHomogeneousListCtorElem recovers T from mapping constructors
// whose values are homogeneous Class() list/tuple/set/frozenset/deque collections
// of the same T, or homogeneous dict-of-Class (dict-of-dict):
//
//	{"k": [A()]} / {"k": [A()], "m": [A()]} / {"k": (A(),)} / {"k": {A()}} → "A"
//	{"k": frozenset([A()])} / {"k": deque([A()])} → "A"
//	{"outer": {"k": A()}} → "A"
//	dict(k=[A()]) / OrderedDict(k=[A()]) / dict(k=[A()], m=(A(),)) → "A"
//	dict([("k", [A()])]) / OrderedDict((("k", [A()]),)) → "A"
//	dict({"k": [A()]}) / OrderedDict({"k": [A()]}) → "A"
//	ChainMap({"k": [A()]}) / ChainMap({"k": [A()]}, {"m": [A()]}) → "A"
//	{k: [A()] for k in ...} → "A"
//
// Stored as elemOf["@nested."+name] so da = {"k": [A()]}; da["k"][0].run() /
// match da: case {"k": [xa]}: / ga = da["k"]; ga[0].run() / for a in da["k"] /
// for ga in da.values(); ga[0].run() / next(iter(da["k"])).run() peel under
// foreign same-leaf (same leaf as annotated dict[str, list[A]] / set[A] /
// dict[str, dict[str, A]]). Scalar {"k": A()} / dict(k=A()) stays on other
// paths; mixed value leaves / empty / splat / mixed positional+kwargs fail closed.
func pythonNestedDictHomogeneousListCtorElem(collection *grammar.Node, content []byte) string {
	if collection == nil {
		return ""
	}
	switch collection.Type() {
	case "dictionary":
		return pythonNestedDictLiteralHomogeneousListCtorElem(collection, content)
	case "dictionary_comprehension":
		return pythonNestedDictCompHomogeneousListCtorElem(collection, content)
	case "call":
		return pythonNestedDictCallHomogeneousListCtorElem(collection, content)
	default:
		return ""
	}
}

// pythonNestedDictValueCollectionElem recovers T when val is a homogeneous
// Class() list/tuple/set, a frozenset/deque/set/list/tuple(...) wrapper of those,
// a dictionary of Class() values (dict-of-dict), or a dict/OrderedDict call whose
// values are Class() (OrderedDict(outer=OrderedDict(k=A()))). Other shapes fail
// closed ("").
func pythonNestedDictValueCollectionElem(val *grammar.Node, content []byte) string {
	if val == nil {
		return ""
	}
	switch val.Type() {
	case "list", "tuple", "set":
		return pythonHomogeneousCtorElem(val, content)
	case "dictionary":
		// Nested mapping of Class() values: {"k": A()} inside outer map.
		return pythonHomogeneousDictLiteralValueCtorElem(val, content)
	case "call":
		// frozenset([A()]) / deque([A()]) / set([A()]) / list([A()]) /
		// tuple((A(),)) / collections.deque([A()]) — single positional
		// homogeneous Class() collection.
		// OrderedDict(k=A()) / dict(k=A()) / OrderedDict({"k": A()}) /
		// collections.OrderedDict(k=A()) — nested scalar mapping of Class()
		// (outer OrderedDict(outer=OrderedDict(k=A())) peels via @nested).
		fn := ingest.ChildByField(val, "function")
		name := pythonSimpleCalleeName(fn, content)
		switch name {
		case "frozenset", "deque", "set", "list", "tuple":
			args, ok := pythonCallPositionalArgNodes(val)
			if !ok || len(args) == 0 {
				return ""
			}
			// deque accepts optional maxlen kw-only; reject extra positionals.
			if name == "deque" {
				if len(args) > 1 {
					return ""
				}
			} else if len(args) != 1 {
				return ""
			}
			return pythonHomogeneousCtorElem(args[0], content)
		case "dict", "OrderedDict":
			return pythonHomogeneousDictValueCtorElem(val, content)
		default:
			return ""
		}
	default:
		return ""
	}
}

// pythonNestedDictLiteralHomogeneousListCtorElem recovers T from a dictionary
// literal whose values are homogeneous Class() list/tuple/set/frozenset/deque
// collections or dict-of-Class.
func pythonNestedDictLiteralHomogeneousListCtorElem(collection *grammar.Node, content []byte) string {
	if collection == nil || collection.Type() != "dictionary" {
		return ""
	}
	var nest string
	saw := false
	for i := uint32(0); i < collection.ChildCount(); i++ {
		ch := collection.Child(i)
		switch ch.Type() {
		case "{", "}", ",", "comment":
			continue
		case "pair":
			val := ingest.ChildByField(ch, "value")
			et := pythonNestedDictValueCollectionElem(val, content)
			if et == "" {
				return ""
			}
			if !saw {
				nest = et
				saw = true
				continue
			}
			if et != nest {
				return ""
			}
		default:
			// Splat / comprehension / unknown — fail closed.
			return ""
		}
	}
	if !saw {
		return ""
	}
	return nest
}

// pythonNestedDictCompHomogeneousListCtorElem recovers T from
// `{k: [A()] for k in ...}` / `{k: frozenset([A()]) for k in ...}` /
// `{k: {"m": A()} for k in ...}` when the pair value is a nested collection
// of Class() T. Nested fors fail closed.
func pythonNestedDictCompHomogeneousListCtorElem(comp *grammar.Node, content []byte) string {
	if comp == nil || comp.Type() != "dictionary_comprehension" {
		return ""
	}
	forCount := 0
	var pair *grammar.Node
	for i := uint32(0); i < comp.ChildCount(); i++ {
		ch := comp.Child(i)
		switch ch.Type() {
		case "for_in_clause":
			forCount++
		case "pair":
			if pair == nil {
				pair = ch
			}
		}
	}
	if forCount != 1 || pair == nil {
		return ""
	}
	return pythonNestedDictValueCollectionElem(ingest.ChildByField(pair, "value"), content)
}

// pythonNestedDictCallHomogeneousListCtorElem recovers T from dict/OrderedDict/
// UserDict/ChainMap forms:
//
//	dict(k=[A()]) / OrderedDict(k=[A()], m={A()}) / UserDict(k=[A()]) — all-keyword
//	dict([("k", [A()])]) / OrderedDict((("k", [A()]),)) — single list/tuple of pairs
//	dict({"k": [A()]}) / OrderedDict({"k": [A()]}) — single dictionary arg
//	ChainMap({"k": [A()]}) / ChainMap(OrderedDict(k=[A()])) — one+ maps
//
// Mixed positional+kwargs (dict/OrderedDict/UserDict), splat, empty, and
// non-collection values fail closed. collections.OrderedDict / UserDict /
// ChainMap accepted via attribute leaf (pythonSimpleCalleeName).
func pythonNestedDictCallHomogeneousListCtorElem(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	name := pythonSimpleCalleeName(fn, content)
	switch name {
	case "dict", "OrderedDict", "UserDict":
		// kwargs / pairs / single dict
	case "ChainMap":
		return pythonNestedChainMapHomogeneousListCtorElem(call, content)
	default:
		return ""
	}
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	var positionals []*grammar.Node
	var keywords []*grammar.Node
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument":
			keywords = append(keywords, ch)
		case "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		default:
			positionals = append(positionals, ch)
		}
	}
	// All-keyword form: dict(k=[A()], m=(A(),)) / OrderedDict(k=[A()])
	if len(positionals) == 0 {
		if len(keywords) == 0 {
			return ""
		}
		var nest string
		saw := false
		for _, kw := range keywords {
			val := ingest.ChildByField(kw, "value")
			et := pythonNestedDictValueCollectionElem(val, content)
			if et == "" {
				return ""
			}
			if !saw {
				nest = et
				saw = true
				continue
			}
			if et != nest {
				return ""
			}
		}
		if !saw {
			return ""
		}
		return nest
	}
	// Single positional only (no kwargs): dict([("k", [A()])]) / dict({"k": [A()]})
	if len(positionals) != 1 || len(keywords) != 0 {
		return ""
	}
	arg := positionals[0]
	switch arg.Type() {
	case "dictionary":
		return pythonNestedDictLiteralHomogeneousListCtorElem(arg, content)
	case "list", "tuple":
		return pythonNestedDictPairsHomogeneousListCtorElem(arg, content)
	default:
		return ""
	}
}

// pythonNestedChainMapHomogeneousListCtorElem recovers T from
// ChainMap({"k": [A()]}) / ChainMap(OrderedDict(k=[A()])) /
// ChainMap({"k": [A()]}, {"m": deque([A()])}) when every positional map has
// nested collection (or dict-of-Class) values of T. Dict literals and
// dict/OrderedDict/UserDict calls accepted.
func pythonNestedChainMapHomogeneousListCtorElem(call *grammar.Node, content []byte) string {
	argList := ingest.ChildByField(call, "arguments")
	if argList == nil {
		return ""
	}
	var nest string
	saw := false
	for i := uint32(0); i < argList.ChildCount(); i++ {
		ch := argList.Child(i)
		switch ch.Type() {
		case "(", ")", ",", "comment":
			continue
		case "keyword_argument", "list_splat", "dictionary_splat", "parenthesized_list_splat":
			return ""
		case "dictionary":
			et := pythonNestedDictLiteralHomogeneousListCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				nest = et
				saw = true
				continue
			}
			if et != nest {
				return ""
			}
		case "call":
			// ChainMap(OrderedDict(k=[A()])) / ChainMap(dict(k=[A()])) /
			// ChainMap(UserDict({"k": [A()]})) / collections.OrderedDict(...).
			et := pythonNestedDictHomogeneousListCtorElem(ch, content)
			if et == "" {
				return ""
			}
			if !saw {
				nest = et
				saw = true
				continue
			}
			if et != nest {
				return ""
			}
		default:
			return ""
		}
	}
	if !saw {
		return ""
	}
	return nest
}

// pythonNestedDictPairsHomogeneousListCtorElem recovers T from a list/tuple of
// key/value pairs where each value is a homogeneous Class() list/tuple/set/
// frozenset/deque (or dict-of-Class):
//
//	[("k", [A()])] / (("k", [A()]),) / [("k", [A()]), ("m", {A()})] → "A"
//	[("k", frozenset([A()]))] / [("outer", {"k": A()})] → "A"
//
// Pair must be a 2-element list/tuple (key ignored; value is 2nd slot). Other
// shapes / mixed leaves fail closed.
func pythonNestedDictPairsHomogeneousListCtorElem(pairs *grammar.Node, content []byte) string {
	if pairs == nil {
		return ""
	}
	switch pairs.Type() {
	case "list", "tuple":
		// ok
	default:
		return ""
	}
	var nest string
	saw := false
	for i := uint32(0); i < pairs.ChildCount(); i++ {
		ch := pairs.Child(i)
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		case "list", "tuple":
			// Pair: (key, value) — value is 2nd element.
			var elems []*grammar.Node
			for j := uint32(0); j < ch.ChildCount(); j++ {
				el := ch.Child(j)
				switch el.Type() {
				case "[", "]", "(", ")", ",", "comment":
					continue
				default:
					elems = append(elems, el)
				}
			}
			if len(elems) != 2 {
				return ""
			}
			et := pythonNestedDictValueCollectionElem(elems[1], content)
			if et == "" {
				return ""
			}
			if !saw {
				nest = et
				saw = true
				continue
			}
			if et != nest {
				return ""
			}
		default:
			return ""
		}
	}
	if !saw {
		return ""
	}
	return nest
}

// pythonCollectionNestedListElemType recovers T from collection annotations whose
// element is itself a list/set/sequence of T:
//
//	list[list[A]] / List[List[A]] / deque[list[A]] / tuple[list[A], ...] /
//	Sequence[set[A]] / Iterable[list[A]] → "A"
//
// Stored as elemOf["@nested."+name] so aa[0][0].run() / ra = aa[0]; ra[0].run() /
// for row in aa; for a in row peel under foreign same-leaf. Scalar list[A] stays
// on pythonContainerElemType. Mapping-of-list uses pythonMappingNestedListElemType.
// Unknown / multi-arg fail closed.
func pythonCollectionNestedListElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	if typeN.Type() == "type" && typeN.ChildCount() > 0 {
		typeN = typeN.Child(0)
	}
	if typeN.Type() == "string" {
		s := strings.Trim(ingest.NodeText(typeN, content), `"'`)
		return pythonParseCollectionNestedListElemString(s)
	}
	if typeN.Type() != "generic_type" {
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
			if contName == "" {
				if attr := ingest.ChildByField(ch, "attribute"); attr != nil {
					contName = ingest.NodeText(attr, content)
				}
			}
		case "type_parameter":
			typeParam = ch
		}
	}
	switch contName {
	case "list", "List", "set", "Set", "frozenset", "FrozenSet",
		"tuple", "Tuple", "Iterable", "Iterator", "Sequence", "MutableSequence",
		"Collection", "Container", "deque", "Deque",
		"AbstractSet", "MutableSet":
		// ok
	default:
		// Unknown single-arg containers (CustomList[list[A]]) fail closed.
		return ""
	}
	if typeParam == nil {
		return ""
	}
	// First (only) type arg must itself be a collection of T.
	var elemType *grammar.Node
	for i := uint32(0); i < typeParam.ChildCount(); i++ {
		ch := typeParam.Child(i)
		if ch.Type() != "type" {
			continue
		}
		// Skip ellipsis in tuple[list[A], ...]
		inner := ch
		if ch.ChildCount() > 0 {
			inner = ch.Child(0)
		}
		if inner != nil && inner.Type() == "ellipsis" {
			continue
		}
		if ch.ChildCount() == 1 && ch.Child(0).Type() == "ellipsis" {
			continue
		}
		elemType = ch
		break
	}
	if elemType == nil {
		return ""
	}
	return pythonContainerElemType(elemType, content)
}

// pythonParseCollectionNestedListElemString handles quoted annotations like
// "list[list[A]]" / "deque[list[A]]".
func pythonParseCollectionNestedListElemString(s string) string {
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
	switch contName {
	case "list", "List", "set", "Set", "frozenset", "FrozenSet",
		"tuple", "Tuple", "Iterable", "Iterator", "Sequence", "MutableSequence",
		"Collection", "Container", "deque", "Deque",
		"AbstractSet", "MutableSet":
		// ok
	default:
		return ""
	}
	inner := strings.TrimSpace(s[lb+1 : rb])
	// Strip trailing ", ..." for tuple[list[A], ...]
	// Take first type arg at top-level comma.
	depth := 0
	end := len(inner)
	for i, c := range inner {
		switch c {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case ',':
			if depth == 0 {
				end = i
				break
			}
		}
		if end != len(inner) {
			break
		}
	}
	elemAnn := strings.TrimSpace(inner[:end])
	if elemAnn == "..." {
		return ""
	}
	return pythonParseContainerElemString(elemAnn)
}

// pythonNestedCollectionIdentElemType recovers T from a bare identifier that is a
// collection-of-list local (elemOf["@nested."+name]). Used for for row in aa when
// aa: list[list[A]] — rows are list-of-T, not T.
func pythonNestedCollectionIdentElemType(right *grammar.Node, content []byte, elemOf map[string]string) string {
	if right == nil || right.Type() != "identifier" || elemOf == nil {
		return ""
	}
	return elemOf["@nested."+ingest.NodeText(right, content)]
}

// pythonNestedMappingItemsElemType recovers T from da.items() when da is a
// mapping of list/set of T. Item values are list-of-T (not T); callers bind
// elemOf[ga] = T for for k, ga in da.items(); ga[0].run().
// Also {**da}.items() / da.copy().items() nested star-copy / copy receivers,
// (da | ea).items() nested merge, and ChainMap(da).items() of nested locals.
func pythonNestedMappingItemsElemType(right *grammar.Node, content []byte, elemOf map[string]string) string {
	if right == nil || right.Type() != "call" || elemOf == nil {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil || ingest.NodeText(attr, content) != "items" {
		return ""
	}
	for obj != nil && obj.Type() == "parenthesized_expression" {
		obj = pythonParenInner(obj)
	}
	if obj == nil {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		return elemOf["@nested."+ingest.NodeText(obj, content)]
	case "dictionary":
		return pythonDictStarCopyNestedValueType(obj, content, elemOf)
	case "binary_operator":
		return pythonDictMergeNestedValueType(obj, content, elemOf)
	case "call":
		if nest := pythonNestedMappingCopyCallType(obj, content, elemOf); nest != "" {
			return nest
		}
		if nest := pythonChainMapLocalNestedValueType(obj, content, elemOf); nest != "" {
			return nest
		}
		return pythonNestedDictHomogeneousListCtorElem(obj, content)
	default:
		return ""
	}
}

// pythonMappingNestedListElemType recovers T from mapping annotations whose value
// is a list/set/sequence of T:
//
//	defaultdict[str, list[A]] / dict[K, List[A]] / Mapping[K, set[A]] → "A"
//
// Stored as elemOf["@nested."+name] so da["k"][0].run() / for a in da["k"] /
// ga = da["k"]; ga[0].run() peel under foreign same-leaf. Scalar mapping values
// (dict[str, A]) stay on pythonContainerElemType. Unknown / multi-arg fail closed.
func pythonMappingNestedListElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil {
		return ""
	}
	if typeN.Type() == "type" && typeN.ChildCount() > 0 {
		typeN = typeN.Child(0)
	}
	if typeN.Type() == "string" {
		s := strings.Trim(ingest.NodeText(typeN, content), `"'`)
		return pythonParseMappingNestedListElemString(s)
	}
	if typeN.Type() != "generic_type" {
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
			if contName == "" {
				if attr := ingest.ChildByField(ch, "attribute"); attr != nil {
					contName = ingest.NodeText(attr, content)
				}
			}
		case "type_parameter":
			typeParam = ch
		}
	}
	switch contName {
	case "dict", "Dict", "Mapping", "MutableMapping", "OrderedDict", "defaultdict", "DefaultDict",
		"ChainMap", "UserDict":
		// ok
	default:
		return ""
	}
	if typeParam == nil {
		return ""
	}
	// Value type is the second type arg.
	var valueType *grammar.Node
	argIdx := 0
	for i := uint32(0); i < typeParam.ChildCount(); i++ {
		ch := typeParam.Child(i)
		if ch.Type() != "type" {
			continue
		}
		argIdx++
		if argIdx == 2 {
			valueType = ch
			break
		}
	}
	if valueType == nil {
		return ""
	}
	// value must itself be a collection of T (list[A] / List[A] / …).
	return pythonContainerElemType(valueType, content)
}

// pythonParseMappingNestedListElemString handles quoted annotations like
// "defaultdict[str, list[A]]".
func pythonParseMappingNestedListElemString(s string) string {
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
	switch contName {
	case "dict", "Dict", "Mapping", "MutableMapping", "OrderedDict", "defaultdict", "DefaultDict",
		"ChainMap", "UserDict":
		// ok
	default:
		return ""
	}
	inner := strings.TrimSpace(s[lb+1 : rb])
	// Split on top-level comma: K, list[A]
	depth := 0
	comma := -1
	for i, c := range inner {
		switch c {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case ',':
			if depth == 0 {
				comma = i
				break
			}
		}
		if comma >= 0 {
			break
		}
	}
	if comma < 0 {
		return ""
	}
	valueAnn := strings.TrimSpace(inner[comma+1:])
	return pythonParseContainerElemString(valueAnn)
}

// pythonDataAttrObjectIdent returns the object identifier for xs.data /
// da.data attribute access (UserList/UserDict underlying container). Other
// attributes and non-ident objects return "".
func pythonDataAttrObjectIdent(n *grammar.Node, content []byte) string {
	if n == nil || n.Type() != "attribute" {
		return ""
	}
	obj := ingest.ChildByField(n, "object")
	attr := ingest.ChildByField(n, "attribute")
	if obj == nil || attr == nil || obj.Type() != "identifier" {
		return ""
	}
	if ingest.NodeText(attr, content) != "data" {
		return ""
	}
	return ingest.NodeText(obj, content)
}

// pythonNestedMappingSubscriptElemType recovers T from da["k"] when da is a
// mapping of list/set of T (elemOf["@nested."+da]). The subscript expression
// is a collection of T (not T itself). Also da.data["k"] when da is UserDict
// of nested list values (underlying .data shares @nested leaf),
// {**da}["k"] star-copy, da.copy()["k"] zero-arg copy,
// (da | ea)["k"] nested merge, and ChainMap(da)["k"] of nested locals.
func pythonNestedMappingSubscriptElemType(sub *grammar.Node, content []byte, elemOf map[string]string) string {
	if sub == nil || sub.Type() != "subscript" || elemOf == nil {
		return ""
	}
	// Non-slice only (slice of nested list values is not product-solid).
	for i := uint32(0); i < sub.ChildCount(); i++ {
		if sub.Child(i).Type() == "slice" {
			return ""
		}
	}
	val := ingest.ChildByField(sub, "value")
	if val == nil {
		return ""
	}
	for val != nil && val.Type() == "parenthesized_expression" {
		val = pythonParenInner(val)
	}
	if val == nil {
		return ""
	}
	switch val.Type() {
	case "identifier":
		return elemOf["@nested."+ingest.NodeText(val, content)]
	case "attribute":
		// da.data["k"] — UserDict underlying mapping shares @nested leaf.
		obj := ingest.ChildByField(val, "object")
		attr := ingest.ChildByField(val, "attribute")
		if obj == nil || attr == nil || obj.Type() != "identifier" {
			return ""
		}
		if ingest.NodeText(attr, content) != "data" {
			return ""
		}
		return elemOf["@nested."+ingest.NodeText(obj, content)]
	case "dictionary":
		// {**da}["k"] / {**da, "j": [A()]}["j"] — nested star-copy.
		return pythonDictStarCopyNestedValueType(val, content, elemOf)
	case "binary_operator":
		// (da | ea)["k"] / (da | {"j": [A()]})["j"] — nested dict merge.
		return pythonDictMergeNestedValueType(val, content, elemOf)
	case "call":
		// da.copy()["k"] / {**da}.copy()["k"] — zero-arg nested mapping copy.
		if nest := pythonNestedMappingCopyCallType(val, content, elemOf); nest != "" {
			return nest
		}
		// ChainMap(da).new_child(...)["k"] — nested new_child merge.
		if nest := pythonChainMapNewChildNestedValueType(val, content, elemOf); nest != "" {
			return nest
		}
		// ChainMap(da)["k"] / ChainMap({"k": [A()]})["k"] — nested ChainMap.
		if nest := pythonChainMapLocalNestedValueType(val, content, elemOf); nest != "" {
			return nest
		}
		return pythonNestedDictHomogeneousListCtorElem(val, content)
	case "list", "tuple":
		// [[A()]][0] / ((A(),),)[0] — outer nested ctor; row is list/tuple of T.
		// Enables aa = [[A()]][0]; aa[0].run() under foreign same-leaf.
		return pythonNestedHomogeneousCtorElem(val, content)
	default:
		return ""
	}
}

// pythonNestedMappingValuesElemType recovers T from da.values() when da is a
// mapping of list/set of T. Values yield list-of-T (not T); callers bind
// elemOf[ga] = T for for ga in da.values(); ga[0].run().
// Also {**da}.values() / da.copy().values() nested star-copy / copy receivers,
// (da | ea).values() nested merge, and ChainMap(da).values() of nested locals.
func pythonNestedMappingValuesElemType(right *grammar.Node, content []byte, elemOf map[string]string) string {
	if right == nil || right.Type() != "call" || elemOf == nil {
		return ""
	}
	fn := ingest.ChildByField(right, "function")
	if fn == nil || fn.Type() != "attribute" {
		return ""
	}
	attr := ingest.ChildByField(fn, "attribute")
	obj := ingest.ChildByField(fn, "object")
	if attr == nil || obj == nil || ingest.NodeText(attr, content) != "values" {
		return ""
	}
	for obj != nil && obj.Type() == "parenthesized_expression" {
		obj = pythonParenInner(obj)
	}
	if obj == nil {
		return ""
	}
	switch obj.Type() {
	case "identifier":
		return elemOf["@nested."+ingest.NodeText(obj, content)]
	case "dictionary":
		return pythonDictStarCopyNestedValueType(obj, content, elemOf)
	case "binary_operator":
		return pythonDictMergeNestedValueType(obj, content, elemOf)
	case "call":
		if nest := pythonNestedMappingCopyCallType(obj, content, elemOf); nest != "" {
			return nest
		}
		if nest := pythonChainMapLocalNestedValueType(obj, content, elemOf); nest != "" {
			return nest
		}
		return pythonNestedDictHomogeneousListCtorElem(obj, content)
	default:
		return ""
	}
}

// pythonBindMatchSubjectTypeCaptures binds match patterns that capture the whole
// subject value when the subject is a typed local of type tn:
//
//	match a: case x: / case x as xa: / case _ as xa:
//
// Class patterns (case A() as a) stay on pythonAsPatternBinding. Nested
// sequence/mapping/class patterns fail closed here.
func pythonBindMatchSubjectTypeCaptures(n *grammar.Node, content []byte, tn string, ourReceivers, out map[string]bool) {
	if n == nil || n.IsNull() || tn == "" || !ourReceivers[tn] {
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
		if name != "" && name != "_" {
			out[name] = true
		}
	case "as_pattern":
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
		if alias != "" && alias != "_" {
			out[alias] = true
		}
		// Left capture (x as xa) also binds; wildcard `_` does not.
		if left != nil {
			// Unwrap left pattern wrappers.
			for left != nil && (left.Type() == "case_pattern" || left.Type() == "pattern") {
				inner := pythonMatchPatternInner(left)
				if inner == nil {
					break
				}
				left = inner
			}
			if left != nil && (left.Type() == "identifier" || left.Type() == "dotted_name") {
				name := pythonMatchCaptureName(left, content)
				if name != "" && name != "_" {
					out[name] = true
				}
			}
		}
	default:
		// class_pattern / list_pattern / value patterns — fail closed here.
	}
}

// pythonSubscriptContainerElemType recovers T from asyncio.Queue[A] /
// collections.deque[A] written as a subscript node (attribute/ident + [T]).
// Single type-arg containers only; mapping forms with two args return the value
// arg. Empty / unknown fail closed.
func pythonSubscriptContainerElemType(typeN *grammar.Node, content []byte) string {
	if typeN == nil || typeN.Type() != "subscript" {
		return ""
	}
	var contName string
	var args []string
	for i := uint32(0); i < typeN.ChildCount(); i++ {
		ch := typeN.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "[", "]", ",", "comment":
			continue
		case "identifier":
			if contName == "" {
				contName = ingest.NodeText(ch, content)
			} else {
				// Type arg as bare identifier (asyncio.Queue[A]).
				args = append(args, ingest.NodeText(ch, content))
			}
		case "attribute":
			if contName == "" {
				if attr := ingest.ChildByField(ch, "attribute"); attr != nil {
					contName = ingest.NodeText(attr, content)
				} else {
					for j := uint32(0); j < ch.ChildCount(); j++ {
						c := ch.Child(j)
						if c != nil && c.Type() == "identifier" {
							contName = ingest.NodeText(c, content)
						}
					}
				}
			}
		case "type":
			if tn := pythonTypeName(ch, content); tn != "" {
				args = append(args, tn)
			}
		default:
			if contName != "" {
				if tn := pythonTypeName(ch, content); tn != "" {
					args = append(args, tn)
				} else {
					return ""
				}
			}
		}
	}
	if contName == "" || len(args) == 0 {
		return ""
	}
	switch contName {
	case "dict", "Dict", "Mapping", "MutableMapping", "OrderedDict", "defaultdict", "DefaultDict",
		"ChainMap":
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
	// asyncio.Queue[A] / collections.deque[A] — tree-sitter uses subscript
	// (attribute/ident + [T]) rather than generic_type for dotted containers.
	if typeN.Type() == "subscript" {
		return pythonSubscriptContainerElemType(typeN, content)
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
	case "dict", "Dict", "Mapping", "MutableMapping", "OrderedDict", "defaultdict", "DefaultDict",
		"ChainMap":
		// value type is last type arg when there are two.
		// ChainMap[K, V] is a mapping of V (same value leaf as dict[K, V]).
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
	case "dict", "Dict", "Mapping", "MutableMapping", "OrderedDict", "defaultdict", "DefaultDict",
		"ChainMap":
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

// pythonUnwrapOptionalTypeNode peels Optional[T] / Union[T, None] / T | None to the
// non-None arm type node so container peels see list[A] under Optional[list[A]] and
// list[A] | None (tree-sitter union_type or binary_operator). Multi non-None arms
// fail closed (returns nil). Non-optional annotations return typeN unchanged
// (after a single type wrapper peel). Used for elemOf / @nested recording;
// pythonTypeName still unwraps scalar leaves.
func pythonUnwrapOptionalTypeNode(typeN *grammar.Node, content []byte) *grammar.Node {
	if typeN == nil {
		return nil
	}
	if typeN.Type() == "type" && typeN.ChildCount() > 0 {
		typeN = typeN.Child(0)
	}
	if typeN == nil {
		return nil
	}
	switch typeN.Type() {
	case "generic_type":
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
			return typeN
		}
		switch contName {
		case "Optional":
			// Optional[T] — single type arg (None is implicit).
			var only *grammar.Node
			count := 0
			for i := uint32(0); i < typeParam.ChildCount(); i++ {
				ch := typeParam.Child(i)
				if ch.Type() != "type" {
					continue
				}
				if pythonIsNoneTypeNode(ch, content) {
					continue
				}
				count++
				only = ch
			}
			if count == 1 && only != nil {
				return pythonUnwrapOptionalTypeNode(only, content)
			}
			return nil
		case "Union":
			// Union[T, None] / Union[None, T] → T; multi non-None fails closed.
			var only *grammar.Node
			count := 0
			for i := uint32(0); i < typeParam.ChildCount(); i++ {
				ch := typeParam.Child(i)
				if ch.Type() != "type" {
					continue
				}
				if pythonIsNoneTypeNode(ch, content) {
					continue
				}
				count++
				only = ch
			}
			if count == 1 && only != nil {
				return pythonUnwrapOptionalTypeNode(only, content)
			}
			return nil
		default:
			return typeN
		}
	case "union_type":
		// list[A] | None — tree-sitter-python uses union_type (not binary_operator).
		// Exactly one non-None arm → that type; multi non-None fails closed.
		var only *grammar.Node
		count := 0
		for i := uint32(0); i < typeN.ChildCount(); i++ {
			ch := typeN.Child(i)
			if ch == nil || ch.Type() == "|" || ch.Type() == "comment" {
				continue
			}
			if pythonIsNoneTypeNode(ch, content) {
				continue
			}
			count++
			only = ch
		}
		if count == 1 && only != nil {
			return pythonUnwrapOptionalTypeNode(only, content)
		}
		return nil
	case "binary_operator":
		// T | None / None | T — exactly one non-None arm.
		arms := pythonPipeUnionArmNodes(typeN, content)
		if arms == nil {
			return nil
		}
		var only *grammar.Node
		count := 0
		for _, a := range arms {
			if a == nil || pythonIsNoneTypeNode(a, content) {
				continue
			}
			count++
			only = a
		}
		if count == 1 && only != nil {
			return pythonUnwrapOptionalTypeNode(only, content)
		}
		return nil
	default:
		return typeN
	}
}

// pythonPipeUnionArmNodes flattens a | chain into type nodes (including None arms).
// Returns nil if any arm is not a resolvable type position (fail closed).
func pythonPipeUnionArmNodes(n *grammar.Node, content []byte) []*grammar.Node {
	if n == nil {
		return nil
	}
	if n.Type() != "binary_operator" {
		return []*grammar.Node{n}
	}
	// Only | unions (PEP 604); other binary ops fail closed.
	op := ingest.ChildByField(n, "operator")
	if op == nil {
		// Some grammars put "|" as a bare child.
		sawPipe := false
		for i := uint32(0); i < n.ChildCount(); i++ {
			if ingest.NodeText(n.Child(i), content) == "|" {
				sawPipe = true
				break
			}
		}
		if !sawPipe {
			return nil
		}
	} else if ingest.NodeText(op, content) != "|" {
		return nil
	}
	left := ingest.ChildByField(n, "left")
	right := ingest.ChildByField(n, "right")
	if left == nil || right == nil {
		return nil
	}
	la := pythonPipeUnionArmNodes(left, content)
	ra := pythonPipeUnionArmNodes(right, content)
	if la == nil || ra == nil {
		return nil
	}
	return append(la, ra...)
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

// pythonCastCallType recovers T from cast(T, x) / typing.cast(T, x) when the
// first type arg peels to a class leaf. Enables cast(A, x).run() under foreign
// same-leaf (assignment form already binds typed locals). Non-cast callees fail closed.
func pythonCastCallType(call *grammar.Node, content []byte) string {
	if call == nil || call.Type() != "call" {
		return ""
	}
	fn := ingest.ChildByField(call, "function")
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier":
		if ingest.NodeText(fn, content) != "cast" {
			return ""
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		if attr == nil || ingest.NodeText(attr, content) != "cast" {
			return ""
		}
		// typing.cast / module.cast — module name unchecked (product: typing.cast).
	default:
		return ""
	}
	return pythonCastTypeArg(call, content)
}

// pythonAsPatternIdentityCMBinding recovers (alias, T) from
// `with nullcontext(x) as a` / `with closing(x) as a` /
// `with contextlib.nullcontext(x) as a` when x peels to T (typed local / Class() /
// method-return ba.get() under foreign same-leaf).
// Same-file @contextmanager factories stay on pythonAsPatternCMYieldBinding.
// Other CM names / arity fail closed.
func pythonAsPatternIdentityCMBinding(n *grammar.Node, content []byte, typeOf, funcReturns, fieldOf map[string]string) (name, typ string) {
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
			if id := ingest.ChildByType(ch, "identifier"); id != nil {
				alias = id
			}
		case "identifier":
			alias = ch
		}
	}
	if left == nil || alias == nil || left.Type() != "call" {
		return "", ""
	}
	fn := ingest.ChildByField(left, "function")
	if fn == nil {
		return "", ""
	}
	ok := false
	switch fn.Type() {
	case "identifier":
		switch ingest.NodeText(fn, content) {
		case "nullcontext", "closing":
			ok = true
		}
	case "attribute":
		attr := ingest.ChildByField(fn, "attribute")
		obj := ingest.ChildByField(fn, "object")
		if attr != nil && obj != nil && obj.Type() == "identifier" &&
			ingest.NodeText(obj, content) == "contextlib" {
			switch ingest.NodeText(attr, content) {
			case "nullcontext", "closing":
				ok = true
			}
		}
	}
	if !ok {
		return "", ""
	}
	args, okArgs := pythonCallPositionalArgNodes(left)
	if !okArgs || len(args) != 1 {
		return "", ""
	}
	tn := pythonObjectExprType(args[0], content, typeOf)
	if tn == "" {
		tn = pythonClassCtorName(args[0], content)
	}
	// nullcontext(ba.get()) / closing(ba.get()) — method-return peels under
	// foreign same-leaf (Class / typed-local peels above).
	if tn == "" {
		tn = pythonObjectLeafType(args[0], content, funcReturns, typeOf, fieldOf)
	}
	if tn == "" {
		return "", ""
	}
	return ingest.NodeText(alias, content), tn
}

// pythonListSliceAssignElemType recovers (local, T) from xs[:] = [A()] /
// xs[i:j] = [A()] / xs[:] = (A(),) / xs[:] = [ba.get()] when the RHS is a
// list/tuple of uniform Class leaves (ctor or method-return) and the left is a
// slice subscript of an identifier. Enables xs[0].run() after slice assign
// under foreign same-leaf. Non-slice keys / mixed RHS / non-object elems fail
// closed.
func pythonListSliceAssignElemType(left, right *grammar.Node, content []byte, funcReturns, typeOf, fieldOf map[string]string) (local, classType string) {
	if left == nil || left.Type() != "subscript" || right == nil {
		return "", ""
	}
	hasSlice := false
	for i := uint32(0); i < left.ChildCount(); i++ {
		if left.Child(i).Type() == "slice" {
			hasSlice = true
			break
		}
	}
	if !hasSlice {
		return "", ""
	}
	val := ingest.ChildByField(left, "value")
	if val == nil || val.Type() != "identifier" {
		return "", ""
	}
	// RHS: list/tuple of Class() / method-return with uniform type.
	if right.Type() != "list" && right.Type() != "tuple" {
		return "", ""
	}
	found := ""
	saw := false
	for i := uint32(0); i < right.ChildCount(); i++ {
		ch := right.Child(i)
		if ch == nil {
			continue
		}
		switch ch.Type() {
		case "[", "]", "(", ")", ",", "comment":
			continue
		}
		// Class() / typed local / zero-arg method return (ba.get()).
		et := pythonObjectLeafType(ch, content, funcReturns, typeOf, fieldOf)
		if et == "" {
			return "", ""
		}
		if !saw {
			found = et
			saw = true
		} else if found != et {
			return "", ""
		}
	}
	if !saw {
		return "", ""
	}
	return ingest.NodeText(val, content), found
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
