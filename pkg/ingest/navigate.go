package ingest

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lucasew/refactree/pkg/projectfs"
)

// SeedResultFS is Seed BFS through fsys (nil = disk). ExpandImports off by default
// (same as SeedResult).
func SeedResultFS(root, seedPath string, fsys projectfs.FS) (*Result, error) {
	return MaterializeSource(ExtractSource{
		Kind:  ExtractSeed,
		Root:  root,
		Paths: []string{seedPath},
		FS:    fsys,
	}, MaterializeOptions{ExpandImports: false, FS: fsys})
}

// HopResultFS parses only the given paths (no BFS), optional ExpandImports one-hop.
func HopResultFS(root string, paths []string, fsys projectfs.FS, expandImports bool) (*Result, error) {
	return MaterializeSource(ExtractSource{
		Kind:  ExtractHop,
		Root:  root,
		Paths: paths,
		FS:    fsys,
	}, MaterializeOptions{ExpandImports: expandImports, FS: fsys})
}

// NavigateAround materializes a neighborhood for goto/hover/refs without a full
// project Dir walk. Seeds from seedAbs (or Hop if not found), then ExpandImports
// one hop so package entry → reexport targets under node_modules are included.
//
// Refactor paths (Rename / ProjectResult) must not use this as a substitute for
// the closed project graph.
func NavigateAround(root string, fsys projectfs.FS, seedAbs string) (*Result, error) {
	if fsys == nil {
		fsys = projectfs.OS{}
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		rootAbs = root
	}
	if !filepath.IsAbs(seedAbs) {
		seedAbs = filepath.Join(rootAbs, filepath.FromSlash(strings.TrimPrefix(seedAbs, "./")))
	}

	// Hop the seed file first (works under node_modules where Seed BFS won't grow).
	hop, err := HopResultFS(rootAbs, []string{seedAbs}, fsys, true)
	if err != nil {
		return nil, err
	}
	// Also Seed from project root perspective — neighbors for non-vendored seeds.
	seed, err := MaterializeSource(ExtractSource{
		Kind:  ExtractSeed,
		Root:  rootAbs,
		Paths: []string{seedAbs},
		FS:    fsys,
	}, MaterializeOptions{ExpandImports: true, FS: fsys})
	if err != nil {
		return hop, nil
	}
	return MergeResults(hop, seed), nil
}

// NavigateReference resolves ref for navigation: hop/seed on demand around its
// path (and follow alias hops up to maxHops), merging into base when non-nil.
func NavigateReference(root string, fsys projectfs.FS, base *Result, ref Reference) (*Result, Reference) {
	if fsys == nil {
		fsys = projectfs.OS{}
	}
	ref = normalizePathReference(ref)
	if ref.Provider == "" {
		ref.Provider = "path"
	}
	out := base
	if out == nil {
		out = &Result{}
	}

	const maxHops = 12
	seen := map[string]bool{}
	for hop := 0; hop < maxHops; hop++ {
		key := ref.String()
		if seen[key] {
			break
		}
		seen[key] = true

		// Prefer graph walk on what we already have.
		next := CanonicalizeInResult(out, ref)
		if entityExactOK(out, next.String()) || (next.Name != "" && entityAtPathSymbolOK(out, next)) {
			return out, next
		}

		// On-demand neighborhood for path-shaped targets (incl. node_modules).
		if next.Provider == "path" || next.Provider == "" {
			abs := absPathForRef(root, next)
			if abs != "" {
				nav, err := NavigateAround(root, fsys, abs)
				if err == nil && nav != nil {
					out = MergeResults(out, nav)
					next2 := CanonicalizeInResult(out, next)
					if next2.String() == next.String() && !entityExactOK(out, next2.String()) {
						// try seed of hop target from aliases
						if aliasT, ok := firstAliasTarget(out, next); ok {
							ref = aliasT
							continue
						}
						return out, next2
					}
					ref = next2
					continue
				}
			}
		}

		// Non-path provider (node:pkg still symbolic): resolve import-like path via hop
		// of current file neighborhood already in base.
		if next.String() == ref.String() {
			return out, next
		}
		ref = next
	}
	return out, CanonicalizeInResult(out, ref)
}

func entityExactOK(result *Result, refStr string) bool {
	_, ok := entityExact(result, refStr)
	return ok
}

func entityAtPathSymbolOK(result *Result, ref Reference) bool {
	_, ok := entityAtPathSymbol(result, ref)
	return ok
}

func absPathForRef(root string, ref Reference) string {
	if ref.Provider != "" && ref.Provider != "path" {
		return ""
	}
	if ref.Path == "" {
		return ""
	}
	if filepath.IsAbs(ref.Path) {
		return ref.Path
	}
	rel := strings.TrimPrefix(ref.Path, "./")
	abs, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return filepath.Join(root, filepath.FromSlash(rel))
	}
	return abs
}

func firstAliasTarget(result *Result, ref Reference) (Reference, bool) {
	if result == nil {
		return Reference{}, false
	}
	key := ref.String()
	scope := FileRef("./" + strings.TrimPrefix(ref.Path, "./"))
	if ref.Name != "" {
		scope = ref.String()
	}
	for _, a := range result.Aliases {
		if a.Reference == key || a.Reference == scope || (ref.Name == "" && a.Reference == FileRef("./"+strings.TrimPrefix(ref.Path, "./"))) {
			if a.Target != "" {
				return ParseReference(a.Target), true
			}
		}
	}
	// also match path-only aliases when looking for symbol
	for _, a := range result.Aliases {
		ar := ParseReference(a.Reference)
		if sameScopePath(ref, ar) && a.Target != "" {
			t := ParseReference(a.Target)
			if ref.Name != "" && t.Name == "" {
				t.Name = ref.Name
			}
			return t, true
		}
	}
	return Reference{}, false
}

// MergeResults concatenates files/entities/aliases/relations (dedupe by identity).
func MergeResults(parts ...*Result) *Result {
	out := &Result{}
	seenFile := map[string]bool{}
	seenEnt := map[string]bool{}
	seenAlias := map[string]bool{}
	seenRel := map[string]bool{}
	for _, p := range parts {
		if p == nil {
			continue
		}
		for _, f := range p.Files {
			k := f.Path + "\x00" + f.Language
			if seenFile[k] {
				continue
			}
			seenFile[k] = true
			out.Files = append(out.Files, f)
		}
		for _, e := range p.Atoms {
			k := e.Reference + "\x00" + strconv.FormatUint(uint64(e.StartByte), 10) + "\x00" + strconv.FormatUint(uint64(e.EndByte), 10)
			if seenEnt[k] {
				continue
			}
			seenEnt[k] = true
			out.Atoms = append(out.Atoms, e)
		}
		for _, a := range p.Aliases {
			k := a.Reference + "\x00" + a.Target + "\x00" + strconv.FormatUint(uint64(a.StartByte), 10)
			if seenAlias[k] {
				continue
			}
			seenAlias[k] = true
			out.Aliases = append(out.Aliases, a)
		}
		for _, r := range p.Uses {
			k := r.Reference + "\x00" + r.Target + "\x00" + strconv.FormatUint(uint64(r.StartByte), 10)
			if seenRel[k] {
				continue
			}
			seenRel[k] = true
			out.Uses = append(out.Uses, r)
		}
	}
	return out
}
