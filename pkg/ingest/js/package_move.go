package js

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// ExpandPackageDirs implements ingest.PackageMovePlanner.
// ECMA has no multi-root package lattice; keep the primary pair only.
func (moveDriver) ExpandPackageDirs(_ *ingest.Result, srcDir, dstDir string) [][2]string {
	return [][2]string{{ingest.CleanRelDir(srcDir), ingest.CleanRelDir(dstDir)}}
}

// RewriteSupportFiles rewrites package.json path strings when a file module is
// relocated. Catalog seed astro moved packages/astro/astro-jsx.d.ts while
// package.json still exported "./astro-jsx": "./astro-jsx.d.ts", breaking the
// package surface even when unit tests would otherwise pass.
func (moveDriver) RewriteSupportFiles(rootDir string, result *ingest.Result, movedFiles map[string]bool, srcDir, dstDir string) ([]ingest.Edit, error) {
	srcDir = ingest.CleanRelDir(srcDir)
	dstDir = ingest.CleanRelDir(dstDir)
	if srcDir == "" || dstDir == "" || srcDir == dstDir {
		return nil, nil
	}

	// Map of old path form → new path form for every relocated file.
	// For a whole-tree package move, apply the same dir rewrite per file.
	replacements := map[string]string{}
	if len(movedFiles) > 0 {
		for oldRel := range movedFiles {
			oldRel = strings.TrimPrefix(oldRel, "./")
			if !isUnderOrEqual(oldRel, srcDir) {
				continue
			}
			under := strings.TrimPrefix(oldRel, srcDir)
			under = strings.TrimPrefix(under, "/")
			newRel := dstDir
			if under != "" {
				newRel = path.Join(dstDir, under)
			}
			addPathReplacement(replacements, oldRel, newRel)
		}
	} else {
		// Fallback: treat srcDir/dstDir as a single file or dir path pair.
		addPathReplacement(replacements, srcDir, dstDir)
	}
	if len(replacements) == 0 {
		return nil, nil
	}

	ingested := map[string]bool{}
	if result != nil {
		for _, f := range result.Files {
			ingested[strings.TrimPrefix(f.Path, "./")] = true
		}
	}

	var edits []ingest.Edit
	err := filepath.WalkDir(rootDir, func(abs string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "build", ".pnpm-store", "coverage":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "package.json" {
			return nil
		}
		rel, err := filepath.Rel(rootDir, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if ingested[rel] || movedFiles[rel] {
			return nil
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		// Apply longer old paths first so prefix overlaps rewrite correctly.
		keys := make([]string, 0, len(replacements))
		for k := range replacements {
			keys = append(keys, k)
		}
		// simple length-desc sort without importing sort for tiny N
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if len(keys[j]) > len(keys[i]) {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for _, oldPath := range keys {
			newPath := replacements[oldPath]
			edits = append(edits, replaceQuotedPath(rel, content, oldPath, newPath)...)
		}
		return nil
	})
	return edits, err
}

func isUnderOrEqual(p, dir string) bool {
	d := strings.TrimSuffix(dir, "/")
	if d == "" {
		return false
	}
	return p == d || strings.HasPrefix(p, d+"/")
}

func addPathReplacement(m map[string]string, oldRel, newRel string) {
	oldRel = strings.TrimPrefix(oldRel, "./")
	newRel = strings.TrimPrefix(newRel, "./")
	if oldRel == "" || newRel == "" || oldRel == newRel {
		return
	}
	m[oldRel] = newRel
	m["./"+oldRel] = "./" + newRel
}

// replaceQuotedPath rewrites "old" / 'old' string literals that equal oldPath.
func replaceQuotedPath(file string, content []byte, oldPath, newPath string) []ingest.Edit {
	if oldPath == "" || oldPath == newPath {
		return nil
	}
	text := string(content)
	var edits []ingest.Edit
	for _, quote := range []byte{'"', '\''} {
		needle := string(quote) + oldPath + string(quote)
		off := 0
		for {
			idx := strings.Index(text[off:], needle)
			if idx < 0 {
				break
			}
			pos := off + idx
			// Span is the path inside the quotes.
			edits = append(edits, ingest.Edit{
				File:    file,
				Span:    ingest.Span{StartByte: uint32(pos + 1), EndByte: uint32(pos + 1 + len(oldPath))},
				NewText: newPath,
			})
			off = pos + len(needle)
		}
	}
	return edits
}

var _ ingest.PackageMovePlanner = moveDriver{}
