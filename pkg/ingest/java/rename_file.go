package java

import (
	"path"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// RenameFileMoves relocates a Java compilation unit when renaming a top-level
// type whose file stem matches the old leaf. Public top-level types must live
// in a file named after the type (JLS 7.6); without this, renames leave
// `public class NewName` in OldName.java and javac fails.
//
// Only same-directory renames of the defining file are emitted. Nested types
// (Outer.Inner) never match a file stem of "Inner", so they are left alone.
func (moveDriver) RenameFileMoves(result *ingest.Result, sourceRefs []string, oldLeaf, newLeaf string) map[string]string {
	if result == nil || oldLeaf == "" || newLeaf == "" || oldLeaf == newLeaf {
		return nil
	}
	// Nested / qualified members are not top-level type renames.
	if strings.Contains(oldLeaf, ".") || strings.Contains(newLeaf, ".") {
		return nil
	}
	moves := map[string]string{}
	seen := map[string]bool{}
	for _, refStr := range sourceRefs {
		ref := ingest.ParseReference(refStr)
		if ingest.AtomName(ref.Name) != oldLeaf {
			continue
		}
		// Skip nested type entities (Outer.Inner).
		if strings.Contains(ref.Name, ".") {
			continue
		}
		rel := strings.TrimPrefix(ref.Path, "./")
		if rel == "" || seen[rel] {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(rel), ".java") {
			continue
		}
		base := path.Base(rel)
		stem := strings.TrimSuffix(base, path.Ext(base))
		if stem != oldLeaf {
			// File already does not follow type-name convention; do not invent a move.
			continue
		}
		dir := path.Dir(rel)
		newRel := newLeaf + ".java"
		if dir != "." && dir != "" {
			newRel = path.Join(dir, newLeaf+".java")
		}
		if newRel == rel {
			continue
		}
		// Avoid colliding with an existing different file in the ingest graph.
		if fileExistsInResult(result, newRel) {
			continue
		}
		seen[rel] = true
		moves[rel] = newRel
	}
	if len(moves) == 0 {
		return nil
	}
	return moves
}

func fileExistsInResult(result *ingest.Result, rel string) bool {
	rel = strings.TrimPrefix(rel, "./")
	for _, f := range result.Files {
		if strings.TrimPrefix(f.Path, "./") == rel {
			return true
		}
	}
	return false
}

var _ ingest.RenameFileMover = moveDriver{}
