package ingest_test

import (
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	_ "github.com/lucasew/refactree/pkg/ingest/java"
	_ "github.com/lucasew/refactree/pkg/ingest/js"
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

func TestECMAFamilyJavascript(t *testing.T) {
	if got := ingest.FamilyForLanguage("javascript"); got != ingest.FamilyECMA {
		t.Fatalf("javascript family=%q want %q", got, ingest.FamilyECMA)
	}
}

func TestLanguageMatchesProjectSameFamily(t *testing.T) {
	if !ingest.LanguageMatchesProject("java", "java") {
		t.Fatal("exact match")
	}
	// When kotlin joins jvm, LanguageMatchesProject("kotlin", "java") should become true.
	if ingest.LanguageMatchesProject("kotlin", "java") {
		t.Fatal("kotlin not registered; must not match java project yet")
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
}
