package java

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// ExpandPackageDirs implements ingest.PackageMovePlanner.
// Java packages often exist under multiple source roots (src/main/java and
// src/test/java); moving only one root leaves PackageLocation errors.
func (moveDriver) ExpandPackageDirs(result *ingest.Result, srcDir, dstDir string) [][2]string {
	srcDir = strings.TrimSuffix(strings.TrimPrefix(srcDir, "./"), "/")
	dstDir = strings.TrimSuffix(strings.TrimPrefix(dstDir, "./"), "/")
	pairs := [][2]string{{srcDir, dstDir}}

	srcSuffix, ok := sourceRootSuffix(srcDir)
	if !ok {
		return pairs
	}
	dstSuffix, ok := sourceRootSuffix(dstDir)
	if !ok {
		return pairs
	}

	seen := map[string]bool{srcDir: true}
	for _, f := range result.Files {
		if f.Language != "java" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		pkgDir := path.Dir(rel)
		if pkgDir == "." {
			continue
		}
		suffix, ok := sourceRootSuffix(pkgDir)
		if !ok || suffix != srcSuffix || seen[pkgDir] {
			continue
		}
		root := strings.TrimSuffix(pkgDir, suffix)
		root = strings.TrimSuffix(root, "/")
		dstPkgDir := dstSuffix
		if root != "" {
			dstPkgDir = path.Join(root, dstSuffix)
		}
		seen[pkgDir] = true
		pairs = append(pairs, [2]string{pkgDir, dstPkgDir})
	}
	return pairs
}

// RewriteSupportFiles implements ingest.PackageMovePlanner.
// Rewrites dotted and slash-separated package tokens in build/config files
// (pom.xml, proguard .conf, etc.) that are outside the ingest graph.
func (moveDriver) RewriteSupportFiles(rootDir string, result *ingest.Result, movedFiles map[string]bool, srcDir, dstDir string) ([]ingest.Edit, error) {
	oldPkg, ok := packageNameFromSourceDir(srcDir)
	if !ok {
		return nil, nil
	}
	newPkg, ok := packageNameFromSourceDir(dstDir)
	if !ok || oldPkg == newPkg {
		return nil, nil
	}

	ingested := map[string]bool{}
	for _, f := range result.Files {
		ingested[strings.TrimPrefix(f.Path, "./")] = true
	}

	oldSlash := strings.ReplaceAll(oldPkg, ".", "/")
	newSlash := strings.ReplaceAll(newPkg, ".", "/")

	var edits []ingest.Edit
	err := filepath.Walk(rootDir, func(abs string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "target", "node_modules", ".gradle", "build":
				return filepath.SkipDir
			}
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
		if !isJavaSupportFile(rel) {
			return nil
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		text := string(content)
		if !strings.Contains(text, oldPkg) && !strings.Contains(text, oldSlash) {
			return nil
		}
		edits = append(edits, rewriteJavaNameToken(rel, content, oldPkg, newPkg)...)
		if oldSlash != oldPkg {
			edits = append(edits, rewriteJavaNameToken(rel, content, oldSlash, newSlash)...)
		}
		return nil
	})
	return edits, err
}

func sourceRootSuffix(dir string) (string, bool) {
	dir = strings.Trim(strings.TrimPrefix(dir, "./"), "/")
	for _, root := range []string{"src/main/java/", "src/test/java/", "src/"} {
		if strings.HasPrefix(dir, root) {
			return strings.TrimPrefix(dir, root), true
		}
	}
	return "", false
}

func packageNameFromSourceDir(dir string) (string, bool) {
	suffix, ok := sourceRootSuffix(dir)
	if !ok || suffix == "" {
		return "", false
	}
	return strings.ReplaceAll(suffix, "/", "."), true
}

func isJavaSupportFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".pro", ".conf", ".xml", ".properties", ".mf", ".gradle", ".kts", ".md", ".txt", ".json", ".cfg", ".ini":
		return true
	}
	base := path.Base(rel)
	return base == "module-info.java" || strings.HasPrefix(base, "Module.")
}

var _ ingest.PackageMovePlanner = moveDriver{}
var _ ingest.MoveDriver = moveDriver{}
