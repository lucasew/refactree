package java

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
)

// ExpandPackageDirs implements ingest.PackageMovePlanner.
// Java packages often exist under multiple source roots (src/main/java,
// src/test/java, src/jmh/java, src/testFixtures/java, src/main/java-templates,
// ...); moving only one root leaves PackageLocation errors when package clauses
// are rewritten elsewhere.
func (moveDriver) ExpandPackageDirs(result *ingest.Result, srcDir, dstDir string) [][2]string {
	srcDir = ingest.CleanRelDir(srcDir)
	dstDir = ingest.CleanRelDir(dstDir)
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
	addPair := func(pkgDir string) {
		pkgDir = ingest.CleanRelDir(pkgDir)
		if pkgDir == "" || seen[pkgDir] {
			return
		}
		suffix, ok := sourceRootSuffix(pkgDir)
		if !ok || suffix != srcSuffix {
			return
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
	for _, f := range result.Files {
		if f.Language != "java" {
			continue
		}
		rel := strings.TrimPrefix(f.Path, "./")
		pkgDir := path.Dir(rel)
		if pkgDir == "." {
			continue
		}
		addPair(pkgDir)
		// Pair the package root when this file lives in a subpackage directory
		// (e.g. .../com/google/gson/internal under .../com/google/gson), including
		// alternate roots such as src/main/java-templates.
		if idx := strings.Index(pkgDir, "/"+srcSuffix+"/"); idx >= 0 {
			addPair(pkgDir[:idx+1+len(srcSuffix)])
		} else if strings.HasSuffix(pkgDir, "/"+srcSuffix) || pkgDir == srcSuffix {
			addPair(pkgDir)
		}
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
	err := filepath.WalkDir(rootDir, func(abs string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
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
	// Longer / more specific roots first. The bare "src/" fallback must stay
	// last so Gradle source sets like jmh/testFixtures are not swallowed as
	// part of the package path (suffix would become "jmh/java/com/foo").
	//
	// Also match when a monorepo/module prefix sits in front of the source
	// root (e.g. gson/src/test/java/com/foo when fuzzy applies on the repo
	// workDir but ingest_roots is gson/). Without this, package renames under
	// a module directory leave package clauses unchanged → PackageLocation.
	for _, root := range []string{
		"src/main/java-templates/",
		"src/test/java-templates/",
		"src/main/java/",
		"src/test/java/",
		"src/jmh/java/",
		"src/testFixtures/java/",
		"src/integrationTest/java/",
		"src/androidTest/java/",
		"src/main/resources/",
		"src/test/resources/",
		"src/",
	} {
		if strings.HasPrefix(dir, root) {
			return strings.TrimPrefix(dir, root), true
		}
		if idx := strings.Index(dir, "/"+root); idx >= 0 {
			return strings.TrimPrefix(dir[idx+1:], root), true
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
