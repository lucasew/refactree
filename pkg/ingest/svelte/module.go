package svelte

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lucasew/ccgo-tree-sitter/grammar"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/javascript"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/svelte"
	_ "github.com/lucasew/ccgo-tree-sitter/grammar/typescript"
	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/ingest/js"
)

func init() {
	// Honest language id "svelte". Component file is the module.
	// Script bodies and markup expressions are re-parsed as ECMA (JS/TS).
	ingest.RegisterLanguageDriver("svelte", languageDriver{})
	ingest.RegisterLanguageRules("svelte", ingest.LanguageRules{
		Extensions:      []string{".svelte"},
		DirectoryModule: false,
	})
}

type languageDriver struct{}

func (languageDriver) Language() string { return "svelte" }

func (languageDriver) TreeSitterGrammar(string) (grammar.Language, bool) {
	return grammar.Get("svelte")
}

func (languageDriver) Extract(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	return extractSvelte(root, source, relPath)
}

func (languageDriver) ResolveImport(sourcePath string, ctx ingest.ImportResolveContext) string {
	return js.ResolveECMAImport(sourcePath, ctx)
}

func (languageDriver) AllowListSymbol(string, ingest.SymbolListOptions) bool { return true }

func (languageDriver) DestinationFileInDirectory(dstDirRel string, srcRef ingest.Reference) string {
	srcPath := strings.TrimPrefix(srcRef.Path, "./")
	base := path.Base(srcPath)
	if base == "." || base == "/" || base == "" {
		return ""
	}
	return path.Join(dstDirRel, base)
}

func extractSvelte(root *grammar.Node, source []byte, relPath string) *ingest.FileExtract {
	fe := &ingest.FileExtract{Language: "svelte", Path: relPath}
	if root == nil {
		return fe
	}
	// Default script grammar for the file; first <script lang=…> wins for markup expr parsing.
	scriptGrammar := "javascript"

	var walk func(n *grammar.Node, inScript bool)
	walk = func(n *grammar.Node, inScript bool) {
		if n == nil {
			return
		}
		switch n.Type() {
		case "script_element":
			lang := scriptLangAttr(n, source)
			if g := js.GrammarNameForScriptLang(lang); g != "" {
				scriptGrammar = g
			}
			mergeScript(fe, n, source, relPath)
			// Do not walk into script raw_text again as markup.
			return
		case "style_element":
			// CSS: ignore for symbol graph.
			return
		case "svelte_raw_text":
			if inScript {
				return
			}
			mergeMarkupExpression(fe, n, source, scriptGrammar)
			return
		case "tag_name":
			if !inScript {
				mergeComponentTag(fe, n, source)
			}
			return
		default:
			for i := uint32(0); i < n.ChildCount(); i++ {
				walk(n.Child(i), inScript)
			}
		}
	}
	walk(root, false)
	return fe
}

func mergeScript(fe *ingest.FileExtract, scriptEl *grammar.Node, source []byte, relPath string) {
	langAttr := scriptLangAttr(scriptEl, source)
	raw := childByType(scriptEl, "raw_text")
	if raw == nil {
		return
	}
	start := raw.StartByte()
	end := raw.EndByte()
	if end > uint32(len(source)) {
		end = uint32(len(source))
	}
	if start >= end {
		return
	}
	script := source[start:end]
	grammarName := js.GrammarNameForScriptLang(langAttr)
	sub, err := js.ExtractECMAScript(script, grammarName, relPath)
	if err != nil || sub == nil {
		return
	}
	for _, e := range sub.Entities {
		e.StartByte += start
		e.EndByte += start
		fe.Entities = append(fe.Entities, e)
	}
	for _, im := range sub.Imports {
		im.StartByte += start
		im.EndByte += start
		if im.TargetStartByte != 0 || im.TargetEndByte != 0 {
			im.TargetStartByte += start
			im.TargetEndByte += start
		}
		fe.Imports = append(fe.Imports, im)
	}
	for _, u := range sub.Usages {
		u.StartByte += start
		u.EndByte += start
		if u.QualStartByte != 0 || u.QualEndByte != 0 {
			u.QualStartByte += start
			u.QualEndByte += start
		}
		fe.Usages = append(fe.Usages, u)
	}
	fe.Reexports = append(fe.Reexports, sub.Reexports...)
	if sub.DefaultExport != "" && fe.DefaultExport == "" {
		fe.DefaultExport = sub.DefaultExport
	}
}

// mergeMarkupExpression parses {…} / #each / #if expression text and records usages.
func mergeMarkupExpression(fe *ingest.FileExtract, raw *grammar.Node, source []byte, grammarName string) {
	start := raw.StartByte()
	end := raw.EndByte()
	if end > uint32(len(source)) {
		end = uint32(len(source))
	}
	if start >= end {
		return
	}
	expr := source[start:end]
	// Skip pure whitespace / numeric literals (e.g. size={18}).
	trim := strings.TrimSpace(string(expr))
	if trim == "" || isAllDigits(trim) {
		return
	}
	usages, err := js.ExtractECMAExpressionUsages(expr, grammarName)
	if err != nil {
		return
	}
	for _, u := range usages {
		u.StartByte += start
		u.EndByte += start
		if u.QualStartByte != 0 || u.QualEndByte != 0 {
			u.QualStartByte += start
			u.QualEndByte += start
		}
		fe.Usages = append(fe.Usages, u)
	}
}

// mergeComponentTag records PascalCase tags as component references (e.g. <Search />).
func mergeComponentTag(fe *ingest.FileExtract, tag *grammar.Node, source []byte) {
	name := ingest.NodeText(tag, source)
	if name == "" || !isComponentTagName(name) {
		return
	}
	fe.Usages = append(fe.Usages, ingest.UsageDef{
		Name:      name,
		StartByte: tag.StartByte(),
		EndByte:   tag.EndByte(),
	})
}

func isComponentTagName(name string) bool {
	// Svelte/custom components are conventionally PascalCase (or dotted Foo.Bar).
	r, _ := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError || !unicode.IsUpper(r) {
		return false
	}
	switch strings.ToLower(name) {
	case "svelte:head", "svelte:window", "svelte:body", "svelte:element", "svelte:component",
		"svelte:self", "svelte:fragment", "svelte:options", "slot":
		return false
	default:
		return true
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func scriptLangAttr(scriptEl *grammar.Node, source []byte) string {
	startTag := childByType(scriptEl, "start_tag")
	if startTag == nil {
		return ""
	}
	for i := uint32(0); i < startTag.ChildCount(); i++ {
		attr := startTag.Child(i)
		if attr.Type() != "attribute" {
			continue
		}
		nameN := childByType(attr, "attribute_name")
		if nameN == nil || !strings.EqualFold(ingest.NodeText(nameN, source), "lang") {
			continue
		}
		if q := childByType(attr, "quoted_attribute_value"); q != nil {
			if v := childByType(q, "attribute_value"); v != nil {
				return ingest.NodeText(v, source)
			}
		}
		if v := childByType(attr, "attribute_value"); v != nil {
			return ingest.NodeText(v, source)
		}
	}
	return ""
}

func childByType(n *grammar.Node, typ string) *grammar.Node {
	if n == nil {
		return nil
	}
	for i := uint32(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c != nil && c.Type() == typ {
			return c
		}
	}
	return nil
}
