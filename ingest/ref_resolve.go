package ingest

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type entityCandidate struct {
	ref      Reference
	language string
}

func normalizePathReference(ref Reference) Reference {
	if ref.Provider != "path" {
		return ref
	}
	if ref.Path == "" {
		ref.Path = "./"
		return ref
	}
	if ref.Path == "." {
		ref.Path = "./"
		return ref
	}
	if strings.HasPrefix(ref.Path, "./") || strings.HasPrefix(ref.Path, "../") || strings.HasPrefix(ref.Path, "/") {
		return ref
	}
	ref.Path = "./" + ref.Path
	return ref
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
			ref:      entRef,
			language: langByPath[entPath],
		})
	}

	if len(candidates) == 0 {
		return ref, fmt.Errorf("no entity %q found under directory %q", ref.Symbol, ref.Path)
	}
	if len(candidates) == 1 {
		return candidates[0].ref, nil
	}

	if picked, ok := pickPreferredDirectoryEntity(candidates, dirRel); ok {
		return picked, nil
	}

	refs := make([]string, 0, len(candidates))
	for _, c := range candidates {
		refs = append(refs, c.ref.String())
	}
	return ref, fmt.Errorf("ambiguous directory reference %q, matches: %s", ref.String(), strings.Join(refs, ", "))
}

func pickPreferredDirectoryEntity(candidates []entityCandidate, dirRel string) (Reference, bool) {
	pyInit := path.Join(dirRel, "__init__.py")
	jsIndex := path.Join(dirRel, "index.js")

	for _, c := range candidates {
		candPath := strings.TrimPrefix(c.ref.Path, "./")
		if c.language == "python" && candPath == pyInit {
			return c.ref, true
		}
	}
	for _, c := range candidates {
		candPath := strings.TrimPrefix(c.ref.Path, "./")
		if c.language == "javascript" && candPath == jsIndex {
			return c.ref, true
		}
	}

	goDirect := []Reference{}
	for _, c := range candidates {
		if c.language != "go" {
			continue
		}
		candPath := strings.TrimPrefix(c.ref.Path, "./")
		if path.Dir(candPath) == dirRel {
			goDirect = append(goDirect, c.ref)
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

	dstDirRel := strings.TrimSuffix(strings.TrimPrefix(dstRef.Path, "./"), "/")
	switch srcLang {
	case "python":
		dstRef.Path = "./" + path.Join(dstDirRel, "__init__.py")
	case "javascript":
		dstRef.Path = "./" + path.Join(dstDirRel, "index.js")
	case "go":
		srcBase := path.Base(strings.TrimPrefix(srcRef.Path, "./"))
		dstRef.Path = "./" + path.Join(dstDirRel, srcBase)
	default:
		return dstRef, fmt.Errorf("unsupported source language %q for directory destination", srcLang)
	}

	return dstRef, nil
}
