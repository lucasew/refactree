package lint

import (
	"fmt"
	"io"
)

// WriteText writes human diagnostics: file:line:col: level: ruleId: message
func WriteText(w io.Writer, res Result) error {
	for _, f := range res.Findings {
		fixNote := ""
		if f.Fixable && f.FixSkipped {
			fixNote = " [fix skipped: overlap]"
		} else if f.Fixable {
			fixNote = " [fixable]"
		}
		if _, err := fmt.Fprintf(w, "%s:%d:%d: %s: %s: %s%s\n",
			f.File, f.Line, f.Column, f.Level, f.RuleID, f.Message, fixNote); err != nil {
			return err
		}
	}
	return nil
}
