package ingest

// Test helpers — only visible to tests in package ingest_test.

func TargetMatchesPackageSymbolForTest(rootDir string, ref Reference, pkgDir string) bool {
	return targetMatchesPackageSymbol(rootDir, ref, pkgDir)
}

func ExpandRenameSourceSetForTest(rootDir string, result *Result, sourceRefs []string) map[string]bool {
	return expandRenameSourceSet(rootDir, result, sourceRefs)
}
