package ingest_test

import (
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/java"
	_ "github.com/lucasew/refactree/pkg/ingest/js"
	_ "github.com/lucasew/refactree/pkg/ingest/svelte"
)

func TestJVMFamilyJavaOnly(t *testing.T) {
	if got := ingest.FamilyForLanguage("java"); got != ingest.FamilyJVM {
		t.Fatalf("java family=%q want %q", got, ingest.FamilyJVM)
	}
	langs := ingest.LanguagesInFamily(ingest.FamilyJVM)
	if len(langs) != 1 || langs[0] != "java" {
		t.Fatalf("jvm surfaces=%v want [java]", langs)
	}
	if _, ok := ingest.LanguageForFile("Main.kt"); ok {
		t.Fatal("kotlin must not be registered yet")
	}
	if ingest.FamilyForLanguage("kotlin") != "" {
		t.Fatal("kotlin must not be in a family until registered")
	}
}

func TestECMAFamilyJavascriptAndSvelte(t *testing.T) {
	if got := ingest.FamilyForLanguage("javascript"); got != ingest.FamilyECMA {
		t.Fatalf("javascript family=%q want %q", got, ingest.FamilyECMA)
	}
	if got := ingest.FamilyForLanguage("svelte"); got != ingest.FamilyECMA {
		t.Fatalf("svelte family=%q want %q", got, ingest.FamilyECMA)
	}
	langs := ingest.LanguagesInFamily(ingest.FamilyECMA)
	hasJS, hasSvelte := false, false
	for _, l := range langs {
		if l == "javascript" {
			hasJS = true
		}
		if l == "svelte" {
			hasSvelte = true
		}
	}
	if !hasJS || !hasSvelte {
		t.Fatalf("ecma surfaces=%v want javascript and svelte", langs)
	}
	if !ingest.SameFamily("javascript", "svelte") {
		t.Fatal("javascript and svelte must share FamilyECMA")
	}
}

func TestLanguageInFamilyCatalogFamily(t *testing.T) {
	// Catalog projects use family ids (jvm, ecma), not language ids.
	if !ingest.LanguageInFamily("java", ingest.FamilyJVM) {
		t.Fatal("java ∈ jvm")
	}
	if ingest.LanguageInFamily("java", "java") {
		t.Fatal("java is not its own family id; catalog uses family=jvm")
	}
	if !ingest.LanguageInFamily("javascript", ingest.FamilyECMA) {
		t.Fatal("javascript ∈ ecma")
	}
	if !ingest.LanguageInFamily("svelte", ingest.FamilyECMA) {
		t.Fatal("svelte ∈ ecma")
	}
	// When kotlin joins jvm, LanguageInFamily("kotlin", FamilyJVM) becomes true.
	if ingest.LanguageInFamily("kotlin", ingest.FamilyJVM) {
		t.Fatal("kotlin not registered; must not match jvm yet")
	}
	if !ingest.SameFamily("java", "java") {
		t.Fatal("same language is same family")
	}
}

func TestFamilyForFileJava(t *testing.T) {
	f, ok := ingest.FamilyForFile("Foo.java")
	if !ok || f != ingest.FamilyJVM {
		t.Fatalf("FamilyForFile java: %q ok=%v", f, ok)
	}
	if _, ok := ingest.FamilyForFile("Foo.kt"); ok {
		t.Fatal("kt has no family until registered")
	}
	f, ok = ingest.FamilyForFile("App.svelte")
	if !ok || f != ingest.FamilyECMA {
		t.Fatalf("FamilyForFile svelte: %q ok=%v", f, ok)
	}
}
