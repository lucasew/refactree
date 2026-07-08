package ingest_test

import (
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestLanguageForFile(t *testing.T) {
	cases := []struct {
		file string
		lang string
		ok   bool
	}{
		{file: "main.go", lang: "go", ok: true},
		{file: "app.py", lang: "python", ok: true},
		{file: "main.js", lang: "javascript", ok: true},
		{file: "main.mjs", lang: "javascript", ok: true},
		{file: "main.cjs", lang: "javascript", ok: true},
		{file: "main.ts", lang: "javascript", ok: true},
		{file: "Button.tsx", lang: "javascript", ok: true},
		{file: "icon.jsx", lang: "javascript", ok: true},
		{file: "Main.java", lang: "java", ok: true},
		{file: "README.md", lang: "", ok: false},
		{file: "App.vue", lang: "", ok: false},
		{file: "Page.astro", lang: "", ok: false},
	}

	for _, tc := range cases {
		got, ok := ingest.LanguageForFile(tc.file)
		if ok != tc.ok {
			t.Fatalf("LanguageForFile(%q) ok=%v want %v", tc.file, ok, tc.ok)
		}
		if got != tc.lang {
			t.Fatalf("LanguageForFile(%q)=%q want %q", tc.file, got, tc.lang)
		}
	}
}

func TestLanguageUsesDirectoryModule(t *testing.T) {
	if !ingest.LanguageUsesDirectoryModule("go") {
		t.Fatal("expected go to use directory module semantics")
	}
	if !ingest.LanguageUsesDirectoryModule("java") {
		t.Fatal("expected java to use directory module semantics")
	}
	if ingest.LanguageUsesDirectoryModule("python") {
		t.Fatal("expected python to use file module semantics")
	}
	if ingest.LanguageUsesDirectoryModule("javascript") {
		t.Fatal("expected javascript to use file module semantics")
	}
}
