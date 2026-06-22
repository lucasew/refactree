package ingest

import (
	"os"
	"path/filepath"
	"strings"
)

// CanonicalizeReference turns a reference into the preferred form for navigation
// and inspection. It is provider-agnostic at the logic layer: after a small
// filesystem prelude for path dirs, it ingests a Result and walks only
// Entities / Aliases (the ingestor graph), which any provider/language shares.
//
// Prelude (path only, to obtain a concrete seed file):
//   - normalize ./ prefix
//   - directory → module entry via DirectoryModuleResolver
//
// Graph walk (any provider, via Result — see CanonicalizeInResult):
//   - exact Entity hit
//   - module/file ref with no symbol → sole entity in that path (default/primary export)
//   - symbol not defined at this path → Alias forwards from this scope (re-exports/barrels)
//   - else sole entity with that symbol anywhere in the Result
//
// When the input uses a non-path provider (go:, python:, …), the final reference
// keeps that provider/path and only updates Symbol (entities in Result are path-shaped).
//
// Missing ingest / unresolvable hops return the best ref found so far (no error).
func CanonicalizeReference(rootDir string, ref Reference) Reference {
	ref = normalizePathReference(ref)
	if ref.Provider == "" {
		ref.Provider = "path"
	}
	origProvider := ref.Provider
	origPath := ref.Path

	rootAbs, err := filepath.Abs(rootDir)
	if err != nil || rootAbs == "" {
		rootAbs = rootDir
		if rootAbs == "" {
			rootAbs = "."
		}
	}

	if ref.Provider == "path" {
		ref = canonicalizeDirectoryModule(rootAbs, ref)
	}

	result, ok := ingestForCanonicalize(rootAbs, ref)
	if !ok || result == nil {
		return ref
	}
	out := CanonicalizeInResult(result, ref)
	return projectToInputProvider(origProvider, origPath, out)
}

// CanonicalizeInResult walks only the provider-agnostic ingest graph (Entities,
// Aliases). No filesystem or language driver calls — callers supply an already
// built Result (e.g. from Ingest / IngestForFile / provider scope ingest).
//
// Intermediate alias/entity targets may be path: refs even when the input was
// go:/python:; use CanonicalizeReference (or projectToInputProvider) to restore
// the caller's provider wrapper.
func CanonicalizeInResult(result *Result, ref Reference) Reference {
	if result == nil {
		return ref
	}
	ref = normalizePathReference(ref)
	if ref.Provider == "" {
		ref.Provider = "path"
	}

	const maxHops = 16
	seen := map[string]bool{}
	for hop := 0; hop < maxHops; hop++ {
		key := ref.String()
		if seen[key] {
			return ref
		}
		seen[key] = true

		if ent, ok := entityExact(result, key); ok {
			return ParseReference(ent.Reference)
		}

		if ref.Symbol == "" {
			if next, ok := soleEntityInScope(result, ref); ok {
				ref = next
				continue
			}
			return ref
		}

		if ent, ok := entityAtPathSymbol(result, ref); ok {
			return ParseReference(ent.Reference)
		}

		if next, ok := followAliasForward(result, ref); ok {
			ref = next
			continue
		}

		if ent, ok := soleEntityNamed(result, ref.Symbol); ok {
			return ParseReference(ent.Reference)
		}

		return ref
	}
	return ref
}

// projectToInputProvider keeps non-path providers (go:fmt) instead of leaking
// path:./print.go entities from the ingest Result.
func projectToInputProvider(origProvider, origPath string, out Reference) Reference {
	if origProvider == "" || origProvider == "path" {
		return out
	}
	return Reference{
		Provider: origProvider,
		Path:     origPath,
		Symbol:   out.Symbol,
	}
}

// CanonicalizePathReference is the directory→module-entry step only (path provider).
// Prefer CanonicalizeReference for full graph canonicalization.
func CanonicalizePathReference(baseDir string, ref Reference) Reference {
	ref = normalizePathReference(ref)
	if ref.Provider != "path" {
		return ref
	}
	if baseDir == "" {
		baseDir = "."
	}
	rootAbs, err := filepath.Abs(baseDir)
	if err != nil {
		rootAbs = baseDir
	}
	return canonicalizeDirectoryModule(rootAbs, ref)
}

func canonicalizeDirectoryModule(rootAbs string, ref Reference) Reference {
	if ref.Provider != "path" || ref.Path == "" || ref.Path == "./" {
		return ref
	}
	rel := strings.TrimPrefix(ref.Path, "./")
	abs := ref.Path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(rootAbs, filepath.FromSlash(rel))
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return ref
	}
	entry, ok := ResolveDirectoryModuleFile(abs)
	if !ok {
		return ref
	}
	entry = filepath.ToSlash(entry)
	ref.Path = "./" + pathJoinSlash(rel, entry)
	return ref
}

// ingestForCanonicalize builds a Result seeded from ref so Entities/Aliases include
// the target and its re-export/import neighborhood.
func ingestForCanonicalize(rootAbs string, ref Reference) (*Result, bool) {
	if ref.Provider == "" || ref.Provider == "path" {
		rel := strings.TrimPrefix(ref.Path, "./")
		if rel == "" || rel == "." {
			return nil, false
		}
		abs := ref.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(rootAbs, filepath.FromSlash(rel))
		}
		st, err := os.Stat(abs)
		if err != nil {
			return nil, false
		}
		if st.IsDir() {
			result, err := IngestWithRecursion(abs, false)
			return result, err == nil
		}
		result, err := IngestForFile(rootAbs, abs)
		return result, err == nil
	}

	scope, ok, err := NewResolver(rootAbs).ResolveScopeTarget(ref)
	if err != nil || !ok {
		return nil, false
	}
	result, err := IngestWithRecursion(scope.Dir, false)
	return result, err == nil
}

func entityExact(result *Result, refStr string) (Entity, bool) {
	for _, ent := range result.Entities {
		if ent.Reference == refStr {
			return ent, true
		}
	}
	return Entity{}, false
}

func entityAtPathSymbol(result *Result, ref Reference) (Entity, bool) {
	for _, ent := range result.Entities {
		er := ParseReference(ent.Reference)
		if er.Symbol != ref.Symbol {
			continue
		}
		if sameScopePath(ref, er) {
			return ent, true
		}
	}
	return Entity{}, false
}

func soleEntityInScope(result *Result, ref Reference) (Reference, bool) {
	var matches []Entity
	for _, ent := range result.Entities {
		er := ParseReference(ent.Reference)
		if sameScopePath(ref, er) {
			matches = append(matches, ent)
		}
	}
	if len(matches) == 1 {
		return ParseReference(matches[0].Reference), true
	}
	return Reference{}, false
}

func soleEntityNamed(result *Result, symbol string) (Entity, bool) {
	var matches []Entity
	for _, ent := range result.Entities {
		er := ParseReference(ent.Reference)
		if er.Symbol == symbol {
			matches = append(matches, ent)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return Entity{}, false
}

// followAliasForward uses Alias targets as provider-agnostic hops (imports and
// re-exports are both recorded as aliases in the Result).
func followAliasForward(result *Result, ref Reference) (Reference, bool) {
	var starHop Reference
	var hasStar bool

	for _, a := range result.Aliases {
		ar := ParseReference(a.Reference)
		if !sameScopePath(ref, ar) {
			continue
		}
		tr := ParseReference(a.Target)
		if tr.Symbol == ref.Symbol {
			return tr, true
		}
		// Star/module forward: alias target is a module/file without symbol.
		if ref.Symbol != "" && tr.Symbol == "" {
			tr.Symbol = ref.Symbol
			if !hasStar {
				starHop = tr
				hasStar = true
			}
		}
	}
	if hasStar {
		return starHop, true
	}
	return Reference{}, false
}

// sameScopePath compares provider+path identity for scope (ignores symbol).
// Non-path providers (go:fmt) do not match path:./print.go entity scopes; alias
// hops use path refs in the Result, so followAliasForward only applies when ref
// itself is path-scoped during intermediate hops.
func sameScopePath(a, b Reference) bool {
	ap := a.Provider
	if ap == "" {
		ap = "path"
	}
	bp := b.Provider
	if bp == "" {
		bp = "path"
	}
	if ap != bp {
		return false
	}
	return normalizeRelPath(a.Path) == normalizeRelPath(b.Path)
}

func normalizeRelPath(p string) string {
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimSuffix(p, "/")
	if p == "." {
		return ""
	}
	return p
}
