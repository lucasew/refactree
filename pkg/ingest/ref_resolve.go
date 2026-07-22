package ingest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	refpkg "github.com/lucasew/refactree/pkg/reference"
)

type atomCandidate struct {
	Ref      Reference
	Language string
}

func allowDirectoryAtomRef(language string) bool {
	return LanguageUsesDirectoryModule(language)
}

func normalizePathReference(ref Reference) Reference {
	return refpkg.NormalizePathReference(ref)
}

func languageForRefPath(result *Result, pathRef string) string {
	needle := strings.TrimPrefix(pathRef, "./")
	for _, f := range result.Files {
		if f.Path == needle {
			return f.Language
		}
	}
	return ""
}

func canonicalSourceReference(dir string, result *Result, ref Reference) (Reference, error) {
	ref = normalizePathReference(ref)
	if ref.Provider != "path" || ref.Name == "" {
		return ref, nil
	}

	absPath := ref.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(dir, strings.TrimPrefix(ref.Path, "./"))
	}

	st, err := os.Stat(absPath)
	if err != nil || !st.IsDir() {
		return ref, nil
	}

	dirRel := CleanRelDir(ref.Path)
	prefix := dirRel
	if prefix != "" {
		prefix += "/"
	}

	langByPath := map[string]string{}
	for _, f := range result.Files {
		langByPath[f.Path] = f.Language
	}

	candidates := []atomCandidate{}
	for _, ent := range result.Atoms {
		entRef := ParseReference(ent.Reference)
		entPath := strings.TrimPrefix(entRef.Path, "./")
		if entRef.Name != ref.Name {
			continue
		}
		if prefix != "" {
			if !strings.HasPrefix(entPath, prefix) {
				continue
			}
		}
		candidates = append(candidates, atomCandidate{
			Ref:      entRef,
			Language: langByPath[entPath],
		})
	}

	if len(candidates) == 0 {
		return ref, fmt.Errorf("no entity %q found under directory %q", ref.Name, ref.Path)
	}
	if !hasDirectoryAtomCandidates(candidates) {
		return ref, fmt.Errorf("directory symbol reference %q is not supported for this language; use a file reference", ref.String())
	}
	if len(candidates) == 1 {
		return candidates[0].Ref, nil
	}

	if picked, ok := pickPreferredDirectoryEntity(candidates, dirRel); ok {
		return picked, nil
	}

	refs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		refs = append(refs, c.Ref.String())
	}
	return ref, fmt.Errorf("ambiguous directory reference %q, matches: %s", ref.String(), strings.Join(refs, ", "))
}

func pickPreferredDirectoryEntity(candidates []atomCandidate, dirRel string) (Reference, bool) {
	direct := []Reference{}
	for _, c := range candidates {
		if !LanguageUsesDirectoryModule(c.Language) {
			continue
		}
		candPath := strings.TrimPrefix(c.Ref.Path, "./")
		if path.Dir(candPath) == dirRel {
			direct = append(direct, c.Ref)
		}
	}
	if len(direct) == 1 {
		return direct[0], true
	}

	return Reference{}, false
}

func hasDirectoryAtomCandidates(candidates []atomCandidate) bool {
	for _, c := range candidates {
		if allowDirectoryAtomRef(c.Language) {
			return true
		}
	}
	return false
}

func canonicalDestinationReference(dir string, result *Result, srcRef, dstRef Reference) (Reference, error) {
	dstRef = normalizePathReference(dstRef)
	if dstRef.Provider != "path" || dstRef.Name == "" {
		return dstRef, nil
	}

	absPath := dstRef.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(dir, strings.TrimPrefix(dstRef.Path, "./"))
	}

	st, err := os.Stat(absPath)
	if err != nil || !st.IsDir() {
		return dstRef, nil
	}

	srcLang := languageForRefPath(result, srcRef.Path)
	if srcLang == "" {
		return dstRef, fmt.Errorf("could not determine language for source %s", srcRef.String())
	}
	if !allowDirectoryAtomRef(srcLang) {
		return dstRef, fmt.Errorf("directory destination %q is not supported for language %q; use a file path", dstRef.Path, srcLang)
	}
	driver, ok := languageDriverForName(srcLang)
	if !ok {
		return dstRef, fmt.Errorf("unsupported source language %q for directory destination", srcLang)
	}

	dstDirRel := CleanRelDir(dstRef.Path)
	dstRelFile := driver.DestinationFileInDirectory(dstDirRel, srcRef)
	if dstRelFile == "" {
		return dstRef, fmt.Errorf("driver %q does not provide directory destination mapping", srcLang)
	}
	dstRef.Path = "./" + path.Clean(dstRelFile)

	return dstRef, nil
}
