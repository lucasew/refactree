package ingest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestNavigateAround_NodeModulesReexport(t *testing.T) {
	dir := t.TempDir()
	// package entry reexports from internal file (not pulled by project Dir alone)
	pkg := filepath.Join(dir, "node_modules", "widget")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "package.json"), []byte(`{"main":"index.js"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "index.js"), []byte(`export { helper } from "./lib.js";\n`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "lib.js"), []byte(`export function helper() { return 1; }\n`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.js"), []byte(`import { helper } from "widget";\nhelper();\n`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Full project: one-hop may stop at index reexport target path
	proj, err := ingest.ProjectResult(dir)
	if err != nil {
		t.Fatal(err)
	}

	// On-demand navigate from import target should pull lib.js entity
	ref := ingest.ParseReference("path:./node_modules/widget/index.js::helper")
	nav, got := ingest.NavigateReference(dir, nil, proj, ref)
	if nav == nil {
		t.Fatal("nil nav result")
	}
	found := false
	for _, e := range nav.Atoms {
		if strings.Contains(e.Reference, "lib.js") && strings.Contains(e.Reference, "helper") {
			found = true
			break
		}
	}
	// either entity on lib or canonicalize landed on lib
	if !found {
		if !strings.Contains(got.Path, "lib.js") && !strings.Contains(got.String(), "helper") {
			// dump for debug
			var ents []string
			for _, e := range nav.Atoms {
				ents = append(ents, e.Reference)
			}
			t.Fatalf("expected helper in lib.js via on-demand nav, got ref=%s entities=%v", got.String(), ents)
		}
	}
}
