package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/pattern"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// Options control a lint run.
type Options struct {
	// Paths optional file/dir filters under Root (same as grep/rewrite).
	Paths []string
	// LangFilter, when set, is a language id or family id; only matching files run.
	LangFilter string
}

// Finding is one rule match at a source location.
type Finding struct {
	RuleID  string
	Level   string
	Message string
	File    string
	Line    int // 1-based
	Column  int // 1-based
	EndLine int
	EndCol  int
	Snippet string
	// SiteEdits are replacement edits for this match (no import hygiene for pattern rules).
	SiteEdits []ingest.Edit
	// Fixable is true when site edits were produced (pattern replacement or builtin fix).
	Fixable bool
	// FixSkipped is true when Fixable but edits were dropped due to earlier-rule conflict.
	FixSkipped bool
	// Source is the file bytes at match time (for SARIF/display).
	source []byte
}

// Result is the outcome of a catalog run.
type Result struct {
	Findings []Finding
	// ApplyEdits is the conflict-filtered site edits plus per-file import hygiene
	// for pattern rewrite rules, ready for ApplyEdits / --fix.
	ApplyEdits []ingest.Edit
	Rules      []CompiledRule
}

// Run walks sources under root and evaluates all compiled rules (YAML order).
func Run(root string, rules []CompiledRule, opts Options) (Result, error) {
	var out Result
	out.Rules = rules
	if len(rules) == 0 {
		return out, nil
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return out, err
	}

	// claimed[file] = non-overlapping site spans already taken by earlier rules.
	claimed := map[string][]ingest.Span{}
	// patternPending: file → pattern rewrite site edits (before import hygiene).
	patternPending := map[string][]ingest.Edit{}
	// builtinApply: file → builtin site edits (already final; no second hygiene pass).
	builtinApply := map[string][]ingest.Edit{}
	type fileRuleNeed struct {
		lang  string
		needs []ingest.ImportNeed
	}
	hygieneMeta := map[string]fileRuleNeed{}

	needLinksAny := false
	for _, r := range rules {
		if r.Builtin != "" {
			continue
		}
		if pattern.PatternNeedsLinks(r.Pattern) {
			needLinksAny = true
			break
		}
	}

	err = pattern.WalkSourceFiles(rootAbs, opts.Paths, func(fe *ingest.FileExtract) error {
		if fe == nil {
			return nil
		}
		if !fileMatchesFilter(fe.Language, opts.LangFilter) {
			return nil
		}

		rel := strings.TrimPrefix(filepath.ToSlash(fe.Path), "./")
		abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
		source, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		pf, err := ingest.ParseSourceFile(abs, fe.Language)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		defer pf.Close()

		var fileResult *ingest.Result
		if needLinksAny {
			fileResult = ingest.Materialize(rootAbs, []*ingest.FileExtract{fe}, ingest.MaterializeOptions{
				ExpandImports: false,
			})
		}

		for _, cr := range rules {
			if !cr.AppliesToFile(fe.Language) {
				continue
			}
			if cr.Builtin != "" {
				if err := runBuiltin(cr, rel, fe.Language, source, claimed, builtinApply, &out); err != nil {
					return err
				}
				continue
			}

			var res *ingest.Result
			if pattern.PatternNeedsLinks(cr.Pattern) {
				res = fileResult
			}
			ms, err := pattern.MatchFile(rootAbs, rel, source, pf.Root, cr.Pattern, res)
			if err != nil {
				return fmt.Errorf("rule %q match %s: %w", cr.Spec.ID, rel, err)
			}
			if len(ms) == 0 {
				continue
			}

			var allSite []ingest.Edit
			perMatchEdits := make([][]ingest.Edit, len(ms))
			if cr.Rule != nil {
				for i, m := range ms {
					edits, err := cr.Rule.Edits([]pattern.Match{m}, source)
					if err != nil {
						return fmt.Errorf("rule %q edits %s: %w", cr.Spec.ID, rel, err)
					}
					perMatchEdits[i] = edits
					allSite = append(allSite, edits...)
				}
			}

			skipFix := false
			if len(allSite) > 0 {
				for _, e := range allSite {
					if spansOverlapAny(e.Span, claimed[rel]) {
						skipFix = true
						break
					}
				}
			}

			for i, m := range ms {
				line, col, endLine, endCol, snippet, err := spanLoc(source, m.Span)
				if err != nil {
					return err
				}
				f := Finding{
					RuleID:    cr.Spec.ID,
					Level:     cr.Level,
					Message:   cr.Spec.Message,
					File:      rel,
					Line:      line,
					Column:    col,
					EndLine:   endLine,
					EndCol:    endCol,
					Snippet:   snippet,
					SiteEdits: perMatchEdits[i],
					Fixable:   cr.Rule != nil && len(perMatchEdits[i]) > 0,
					source:    source,
				}
				if f.Fixable && skipFix {
					f.FixSkipped = true
				}
				out.Findings = append(out.Findings, f)
			}

			if len(allSite) > 0 && !skipFix {
				claimed[rel] = append(claimed[rel], siteSpans(allSite)...)
				patternPending[rel] = append(patternPending[rel], allSite...)
				if cr.Rule != nil {
					needs := pattern.ImportNeedsForRule(fe.Language, *cr.Rule)
					meta := hygieneMeta[rel]
					meta.lang = fe.Language
					meta.needs = append(meta.needs, needs...)
					hygieneMeta[rel] = meta
				}
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	for _, edits := range builtinApply {
		out.ApplyEdits = append(out.ApplyEdits, edits...)
	}

	// Import hygiene per file on accepted *pattern* site edits only.
	for rel, siteEdits := range patternPending {
		if len(siteEdits) == 0 {
			continue
		}
		abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
		source, err := os.ReadFile(abs)
		if err != nil {
			return out, err
		}
		meta := hygieneMeta[rel]
		prune := ingest.PruneImportOpts{MaskSpans: siteEditMaskSpans(siteEdits)}
		edits := pattern.WithImportHygiene(rel, meta.lang, source, siteEdits, dedupeNeeds(meta.needs), prune)
		out.ApplyEdits = append(out.ApplyEdits, edits...)
	}
	return out, nil
}

func runBuiltin(cr CompiledRule, rel, lang string, source []byte, claimed map[string][]ingest.Span, builtinApply map[string][]ingest.Edit, out *Result) error {
	switch cr.Builtin {
	case BuiltinDeadImports:
		return runDeadImports(cr, rel, lang, source, claimed, builtinApply, out)
	default:
		return fmt.Errorf("rule %q: unknown builtin %q", cr.Spec.ID, cr.Builtin)
	}
}

func runDeadImports(cr CompiledRule, rel, lang string, source []byte, claimed map[string][]ingest.Span, builtinApply map[string][]ingest.Edit, out *Result) error {
	h, ok := ingest.ImportHygieneForLanguage(lang)
	if !ok {
		return nil
	}
	// Full-file prune: no mask, no candidate filter — drop unused named imports only.
	edits := h.PruneNamedUnusedEdits(rel, source, ingest.PruneImportOpts{})
	if len(edits) == 0 {
		return nil
	}

	skipFix := false
	for _, e := range edits {
		if spansOverlapAny(e.Span, claimed[rel]) {
			skipFix = true
			break
		}
	}

	for _, e := range edits {
		line, col, endLine, endCol, snippet, err := spanLoc(source, e.Span)
		if err != nil {
			return err
		}
		snippet = strings.TrimSpace(snippet)
		msg := cr.Spec.Message
		if snippet != "" {
			msg = cr.Spec.Message + ": " + oneLine(snippet)
		}
		site := []ingest.Edit{e}
		f := Finding{
			RuleID:    cr.Spec.ID,
			Level:     cr.Level,
			Message:   msg,
			File:      rel,
			Line:      line,
			Column:    col,
			EndLine:   endLine,
			EndCol:    endCol,
			Snippet:   oneLine(snippet),
			SiteEdits: site,
			Fixable:   true,
			source:    source,
		}
		if skipFix {
			f.FixSkipped = true
		}
		out.Findings = append(out.Findings, f)
	}

	if !skipFix {
		claimed[rel] = append(claimed[rel], siteSpans(edits)...)
		builtinApply[rel] = append(builtinApply[rel], edits...)
	}
	return nil
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i]) + "…"
	}
	return s
}

func fileMatchesFilter(fileLang, filter string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	if fileLang == filter {
		return true
	}
	return ingest.LanguageInFamily(fileLang, filter)
}

func siteSpans(edits []ingest.Edit) []ingest.Span {
	out := make([]ingest.Span, 0, len(edits))
	for _, e := range edits {
		if e.EndByte > e.StartByte {
			out = append(out, e.Span)
		}
	}
	return out
}

func siteEditMaskSpans(edits []ingest.Edit) []ingest.Span {
	return siteSpans(edits)
}

func spansOverlapAny(sp ingest.Span, claimed []ingest.Span) bool {
	for _, c := range claimed {
		if spansOverlap(sp, c) {
			return true
		}
	}
	return false
}

// spansOverlap reports half-open range overlap [a,b) ∩ [c,d) ≠ ∅.
func spansOverlap(a, b ingest.Span) bool {
	return a.StartByte < b.EndByte && b.StartByte < a.EndByte
}

func dedupeNeeds(needs []ingest.ImportNeed) []ingest.ImportNeed {
	if len(needs) <= 1 {
		return needs
	}
	seen := map[string]bool{}
	var out []ingest.ImportNeed
	for _, n := range needs {
		if n.ImportPath == "" || seen[n.ImportPath] {
			continue
		}
		seen[n.ImportPath] = true
		out = append(out, n)
	}
	return out
}

func spanLoc(src []byte, sp ingest.Span) (line, col, endLine, endCol int, snippet string, err error) {
	if int(sp.EndByte) > len(src) || sp.StartByte > sp.EndByte {
		return 0, 0, 0, 0, "", fmt.Errorf("span out of range")
	}
	li := grammar.NewLineIndexBytes(src)
	l, c0 := li.LineColumnAtU32(sp.StartByte)
	el, ec0 := li.LineColumnAtU32(sp.EndByte)
	text := string(src[sp.StartByte:sp.EndByte])
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i] + "…"
	}
	return l, c0 + 1, el, ec0 + 1, text, nil
}
