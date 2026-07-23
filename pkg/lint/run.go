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
	// SiteEdits are replacement edits for this match (no import hygiene).
	SiteEdits []ingest.Edit
	// Fixable is true when the rule has a replacement and site edits were produced.
	Fixable bool
	// FixSkipped is true when Fixable but edits were dropped due to earlier-rule conflict.
	FixSkipped bool
	// Source is the file bytes at match time (for SARIF/display); not written to disk formats as-is.
	source []byte
}

// Result is the outcome of a catalog run.
type Result struct {
	Findings []Finding
	// ApplyEdits is the conflict-filtered site edits plus per-file import hygiene,
	// ready for ApplyEdits / --fix.
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
	// pendingHygiene: file → site edits accepted for apply (before hygiene).
	pending := map[string][]ingest.Edit{}
	// needsByFile accumulates import needs for accepted rewrite rules.
	type fileRuleNeed struct {
		lang  string
		needs []ingest.ImportNeed
	}
	hygieneMeta := map[string]fileRuleNeed{}

	needLinksAny := false
	for _, r := range rules {
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
			// Materialize once if any rule needs links; cheap hop reuse of extract.
			fileResult = ingest.Materialize(rootAbs, []*ingest.FileExtract{fe}, ingest.MaterializeOptions{
				ExpandImports: false,
			})
		}

		for _, cr := range rules {
			if !cr.AppliesToFile(fe.Language) {
				continue
			}
			// Skip link materialize cost per rule when only some need links:
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

			// Conflict: if any site edit of this rule overlaps claimed spans, skip all
			// fixes for this rule on this file (findings still emitted).
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
				line, col, endLine, endCol, snippet, err := matchLoc(source, m)
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
				pending[rel] = append(pending[rel], allSite...)
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

	// Import hygiene per file on accepted site edits.
	for rel, siteEdits := range pending {
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

func matchLoc(src []byte, m pattern.Match) (line, col, endLine, endCol int, snippet string, err error) {
	if int(m.EndByte) > len(src) || m.StartByte > m.EndByte {
		return 0, 0, 0, 0, "", fmt.Errorf("match span out of range in %s", m.File)
	}
	li := grammar.NewLineIndexBytes(src)
	l, c0 := li.LineColumnAtU32(m.StartByte)
	el, ec0 := li.LineColumnAtU32(m.EndByte)
	text := string(src[m.StartByte:m.EndByte])
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i] + "…"
	}
	// LineIndex: line 1-based, column 0-based byte col → 1-based for tools.
	return l, c0 + 1, el, ec0 + 1, text, nil
}
