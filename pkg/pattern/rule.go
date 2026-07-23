package pattern

import (
	"fmt"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/modernc-tree-sitter/ccgo-tree-sitter/grammar"
)

// Rule is one site transform: match Pattern, emit Replacement text as ingest.Edit
// values. This is the common unit for rft rewrite and for planner-generated site
// rewrites (for example @oldRef leaf → new identifier text for mv use sites).
//
// Apply stays on ingest (Edit / StageEdits / ApplyEdits). Planners differ; the
// site engine is Rule → Match → []Edit.
type Rule struct {
	Pattern     Node
	Replacement Node
	// SetCapture, when non-empty, emits one edit per span of that capture
	// instead of the whole match root (CLI form: "name=template").
	SetCapture string
}

// Valid reports whether r can be expanded (non-empty pattern and replacement).
func (r Rule) Valid() error {
	if r.Pattern.Kind == "" {
		return fmt.Errorf("rule: empty pattern")
	}
	if r.Replacement.Kind == "" {
		return fmt.Errorf("rule: empty replacement")
	}
	return nil
}

// NeedsLinks reports whether matching this rule needs ingest hyperlink targets.
func (r Rule) NeedsLinks() bool {
	return PatternNeedsLinks(r.Pattern)
}

// Edits turns matches + this rule's replacement into ingest.Edit values.
// source is the file content for all matches (one file per call).
func (r Rule) Edits(matches []Match, source []byte) ([]ingest.Edit, error) {
	if err := r.Valid(); err != nil {
		return nil, err
	}
	return EditsForMatches(matches, r.Replacement, source, r.SetCapture)
}

// MatchFile runs this rule's pattern on one parsed file (same as MatchFile + Pattern).
func (r Rule) MatchFile(root, fileRel string, source []byte, rootNode *grammar.Node, result *ingest.Result) ([]Match, error) {
	if r.Pattern.Kind == "" {
		return nil, fmt.Errorf("rule: empty pattern")
	}
	return MatchFile(root, fileRel, source, rootNode, r.Pattern, result)
}

// ExpandFile matches and builds edits for one file under root.
func (r Rule) ExpandFile(root, fileRel string, source []byte, rootNode *grammar.Node, result *ingest.Result) (matches []Match, edits []ingest.Edit, err error) {
	matches, err = r.MatchFile(root, fileRel, source, rootNode, result)
	if err != nil {
		return nil, nil, err
	}
	if len(matches) == 0 {
		return matches, nil, nil
	}
	edits, err = r.Edits(matches, source)
	return matches, edits, err
}

// RuleFromOp builds a site Rule from a rewrite Op (fixtures / CLI).
func RuleFromOp(op Op) (Rule, error) {
	if op.ReplacementIR == nil {
		return Rule{}, fmt.Errorf("rule: op has no replacement_ir")
	}
	r := Rule{
		Pattern:     op.PatternIR,
		Replacement: *op.ReplacementIR,
		SetCapture:  op.SetCapture,
	}
	if err := r.Valid(); err != nil {
		return Rule{}, err
	}
	return r, nil
}

// RuleFromStrings parses pattern and replacement templates (rewrite CLI form).
// replacement may be "name=template" to set only capture $name when name is
// declared by the pattern.
func RuleFromStrings(patternStr, replacementStr string) (Rule, error) {
	pat, err := ParsePattern(patternStr)
	if err != nil {
		return Rule{}, fmt.Errorf("pattern: %w", err)
	}
	setName, tmpl := splitCaptureSet(pat, replacementStr)
	repl, err := ParseReplacement(tmpl)
	if err != nil {
		return Rule{}, fmt.Errorf("replacement: %w", err)
	}
	r := Rule{Pattern: pat, Replacement: repl, SetCapture: setName}
	if err := r.Valid(); err != nil {
		return Rule{}, err
	}
	return r, nil
}

// RefLeafRule matches tokens whose hyperlink target is targetRef and replaces
// that leaf token with newText. This is the site-engine shape of a use-site
// rename: @oldRef → new identifier text (not a full symbol-identity mv plan).
//
// targetRef accepts with or without a leading @; product form is provider:path::Name.
func RefLeafRule(targetRef, newText string) (Rule, error) {
	ref := strings.TrimSpace(targetRef)
	ref = strings.TrimPrefix(ref, "@")
	if ref == "" {
		return Rule{}, fmt.Errorf("RefLeafRule: empty target ref")
	}
	if newText == "" {
		return Rule{}, fmt.Errorf("RefLeafRule: empty new text")
	}
	// Bare @ref matches the link leaf only (dialect: hyperlink on the leaf token).
	return RuleFromStrings("@"+ref, newText)
}
