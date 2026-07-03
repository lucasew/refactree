package fuzzy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// InvariantOptions tunes strictness.
type InvariantOptions struct {
	StrictRefs bool
}

// InvariantFailure is one graph consistency problem.
type InvariantFailure struct {
	Check   string `json:"check"`
	Message string `json:"message"`
}

func (f InvariantFailure) String() string {
	return f.Check + ": " + f.Message
}

// CheckInvariants validates an ingest Result against on-disk sources.
func CheckInvariants(dir string, result *ingest.Result, opts InvariantOptions) []InvariantFailure {
	var fails []InvariantFailure
	if result == nil {
		return []InvariantFailure{{Check: "result", Message: "nil result"}}
	}

	fileSizes := map[string]int{}
	entityRefs := map[string]bool{}
	fileRefs := map[string]bool{}

	for _, f := range result.Files {
		rel := strings.TrimPrefix(f.Path, "./")
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			fails = append(fails, InvariantFailure{Check: "file_exists", Message: fmt.Sprintf("%s: %v", f.Path, err)})
		} else {
			fileSizes[rel] = len(data)
		}
		if f.Language == "" {
			fails = append(fails, InvariantFailure{Check: "language", Message: fmt.Sprintf("%s: empty language", f.Path)})
		}
		addFileRef(fileRefs, rel)
	}

	entityKeys := map[string]bool{}
	for _, e := range result.Entities {
		entityRefs[e.Reference] = true
		key := fmt.Sprintf("%s\x00%d\x00%d", e.Reference, e.StartByte, e.EndByte)
		if entityKeys[key] {
			fails = append(fails, InvariantFailure{Check: "duplicate_entity", Message: e.Reference})
		}
		entityKeys[key] = true

		ref := ingest.ParseReference(e.Reference)
		rel := strings.TrimPrefix(ref.Path, "./")
		fails = append(fails, checkSpan("entity_span", e.Reference, e.StartByte, e.EndByte, fileSizes[rel])...)
		if ref.Symbol != "" {
			if text, ok := readSpan(dir, rel, e.StartByte, e.EndByte); ok && text != symbolLeaf(ref.Symbol) {
				fails = append(fails, InvariantFailure{
					Check:   "entity_text",
					Message: fmt.Sprintf("%s span %q != symbol leaf %q (from %q)", e.Reference, text, symbolLeaf(ref.Symbol), ref.Symbol),
				})
			}
		}
	}

	resolveTarget := func(target string) bool {
		if entityRefs[target] || fileRefs[target] {
			return true
		}
		tref := ingest.ParseReference(target)
		if tref.Provider != "" && tref.Provider != "path" {
			return true
		}
		trel := strings.TrimPrefix(tref.Path, "./")
		abs := filepath.Join(dir, filepath.FromSlash(trel))
		if st, err := os.Stat(abs); err == nil {
			return st.IsDir() || st.Mode().IsRegular()
		}
		return false
	}

	for _, a := range result.Aliases {
		ref := ingest.ParseReference(a.Reference)
		rel := strings.TrimPrefix(ref.Path, "./")
		fails = append(fails, checkSpan("alias_span", a.Reference, a.StartByte, a.EndByte, fileSizes[rel])...)
		if opts.StrictRefs && a.Target != "" && !resolveTarget(a.Target) {
			fails = append(fails, InvariantFailure{
				Check:   "dangling_alias_target",
				Message: fmt.Sprintf("%s -> %s", a.Reference, a.Target),
			})
		}
	}
	for _, r := range result.Relations {
		ref := ingest.ParseReference(r.Reference)
		rel := strings.TrimPrefix(ref.Path, "./")
		fails = append(fails, checkSpan("relation_span", r.Reference, r.StartByte, r.EndByte, fileSizes[rel])...)
		if opts.StrictRefs && r.Target != "" && !resolveTarget(r.Target) {
			fails = append(fails, InvariantFailure{
				Check:   "dangling_relation_target",
				Message: fmt.Sprintf("%s -> %s", r.Reference, r.Target),
			})
		}
	}

	return uniqueFailures(fails)
}

func addFileRef(fileRefs map[string]bool, rel string) {
	variants := []string{
		"path:./" + rel,
		"path:" + rel,
		"path:./" + strings.TrimPrefix(rel, "./"),
	}
	for _, v := range variants {
		fileRefs[v] = true
	}
	dirPath := filepath.ToSlash(filepath.Dir(rel))
	if dirPath != "." && dirPath != "" {
		fileRefs["path:./"+dirPath] = true
		fileRefs["path:./"+dirPath+"/"] = true
	}
}

// CheckIdempotentIngest re-ingests and compares sorted JSON.
func CheckIdempotentIngest(dir string, first *ingest.Result) []InvariantFailure {
	second, err := ingest.Ingest(dir)
	if err != nil {
		return []InvariantFailure{{Check: "reingest", Message: err.Error()}}
	}
	a := cloneSorted(first)
	b := cloneSorted(second)
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) {
		return []InvariantFailure{{Check: "idempotent", Message: "second ingest diverged"}}
	}
	return nil
}

// CheckWalkSymbolsSubset ensures listed symbols are present as entities.
func CheckWalkSymbolsSubset(dir string, result *ingest.Result) []InvariantFailure {
	entityRefs := map[string]bool{}
	for _, e := range result.Entities {
		entityRefs[e.Reference] = true
	}
	var fails []InvariantFailure
	err := ingest.WalkSymbols(dir, "path:./", ingest.ListOptions{IncludeHidden: true, Recursive: true}, func(info ingest.SymbolInfo) bool {
		ref := info.Entity.Reference
		if !entityRefs[ref] {
			fails = append(fails, InvariantFailure{
				Check:   "walk_symbols_subset",
				Message: ref + " listed but missing from ingest entities",
			})
		}
		return true
	})
	if err != nil {
		fails = append(fails, InvariantFailure{Check: "walk_symbols", Message: err.Error()})
	}
	return fails
}

func checkSpan(check, ref string, start, end uint32, size int) []InvariantFailure {
	if start > end || (start == end && end != 0) {
		// zero-span aliases are allowed (start == end == 0)
		if start == end {
			return nil
		}
		return []InvariantFailure{{Check: check, Message: fmt.Sprintf("%s: start %d > end %d", ref, start, end)}}
	}
	if start == end {
		return nil
	}
	if size > 0 && int(end) > size {
		return []InvariantFailure{{Check: check, Message: fmt.Sprintf("%s: end %d > file size %d", ref, end, size)}}
	}
	return nil
}

// symbolLeaf returns the identifier text expected at an entity span.
// Qualified symbols use "." separators; Go pointer receivers may prefix "*".
func symbolLeaf(symbol string) string {
	leaf := symbol
	if i := strings.LastIndex(leaf, "."); i >= 0 {
		leaf = leaf[i+1:]
	}
	return strings.TrimPrefix(leaf, "*")
}

func readSpan(dir, rel string, start, end uint32) (string, bool) {
	if start >= end {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil || int(end) > len(data) {
		return "", false
	}
	return string(data[start:end]), true
}

func cloneSorted(r *ingest.Result) *ingest.Result {
	if r == nil {
		return nil
	}
	cp := *r
	cp.Files = append([]ingest.File(nil), r.Files...)
	cp.Entities = append([]ingest.Entity(nil), r.Entities...)
	cp.Aliases = append([]ingest.Alias(nil), r.Aliases...)
	cp.Relations = append([]ingest.Relation(nil), r.Relations...)
	ingest.SortResult(&cp)
	return &cp
}

func uniqueFailures(in []InvariantFailure) []InvariantFailure {
	seen := map[string]bool{}
	var out []InvariantFailure
	for _, f := range in {
		k := f.Check + "\x00" + f.Message
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}
