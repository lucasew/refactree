package ingest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	refpkg "github.com/lucasew/refactree/pkg/reference"
)

type entityCandidate struct {
	Ref      Reference
	Language string
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
	if ref.Provider != "path" || ref.Symbol == "" {
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

	dirRel := strings.TrimSuffix(strings.TrimPrefix(ref.Path, "./"), "/")
	prefix := dirRel
	if prefix != "" {
		prefix += "/"
	}

	langByPath := map[string]string{}
	for _, f := range result.Files {
		langByPath[f.Path] = f.Language
	}

	candidates := []entityCandidate{}
	for _, ent := range result.Entities {
		entRef := ParseReference(ent.Reference)
		entPath := strings.TrimPrefix(entRef.Path, "./")
		if entRef.Symbol != ref.Symbol {
			continue
		}
		if prefix != "" {
			if !strings.HasPrefix(entPath, prefix) {
				continue
			}
		}
		candidates = append(candidates, entityCandidate{
			Ref:      entRef,
			Language: langByPath[entPath],
		})
	}

	if len(candidates) == 0 {
		return ref, fmt.Errorf("no entity %q found under directory %q", ref.Symbol, ref.Path)
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

func pickPreferredDirectoryEntity(candidates []entityCandidate, dirRel string) (Reference, bool) {
	// Keep deterministic preference for languages that define a canonical
	// directory entry file.
	for _, lang := range []string{"python", "javascript"} {
		driver, ok := languageDriverForName(lang)
		if !ok {
			continue
		}
		preferred := driver.DirectoryEntryFile(dirRel)
		if preferred == "" {
			continue
		}
		preferred = path.Clean(preferred)
		for _, c := range candidates {
			if c.Language != lang {
				continue
			}
			candPath := path.Clean(strings.TrimPrefix(c.Ref.Path, "./"))
			if candPath == preferred {
				return c.Ref, true
			}
		}
	}

	goDirect := []Reference{}
	for _, c := range candidates {
		if c.Language != "go" {
			continue
		}
		candPath := strings.TrimPrefix(c.Ref.Path, "./")
		if path.Dir(candPath) == dirRel {
			goDirect = append(goDirect, c.Ref)
		}
	}
	if len(goDirect) == 1 {
		return goDirect[0], true
	}

	return Reference{}, false
}

func canonicalDestinationReference(dir string, result *Result, srcRef, dstRef Reference) (Reference, error) {
	dstRef = normalizePathReference(dstRef)
	if dstRef.Provider != "path" || dstRef.Symbol == "" {
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
	driver, ok := languageDriverForName(srcLang)
	if !ok {
		return dstRef, fmt.Errorf("unsupported source language %q for directory destination", srcLang)
	}

	dstDirRel := strings.TrimSuffix(strings.TrimPrefix(dstRef.Path, "./"), "/")
	dstRelFile := driver.DestinationFileInDirectory(dstDirRel, srcRef)
	if dstRelFile == "" {
		return dstRef, fmt.Errorf("driver %q does not provide directory destination mapping", srcLang)
	}
	dstRef.Path = "./" + path.Clean(dstRelFile)

	return dstRef, nil
}
