package ingest

import (
	"fmt"
	"strings"
	"sync"
)

// Well-known language families. Surfaces register under a family so move/module
// models and catalog projects select by family (not a single language id).
// Singleton families (go, python, …) equal the sole surface language id.
const (
	FamilyJVM    = "jvm"    // Java today; Kotlin later
	FamilyECMA   = "ecma"   // JS / TS / TSX / JSX / Svelte (import-export module model)
	FamilyGo     = "go"     // Go only
	FamilyPython = "python" // Python only
	FamilyNix    = "nix"    // Nix only
)

var (
	familyMu      sync.RWMutex
	familyByLang  = map[string]string{} // language id → family
	langsByFamily = map[string][]string{}
)

// RegisterLanguageFamily records that language belongs to family.
// Empty family is allowed (language is standalone). Panics on empty language
// or conflicting family for the same language.
func RegisterLanguageFamily(language, family string) {
	if language == "" {
		panic("ingest: RegisterLanguageFamily with empty language")
	}
	family = strings.TrimSpace(family)

	familyMu.Lock()
	defer familyMu.Unlock()

	if prev, ok := familyByLang[language]; ok && prev != family {
		panic(fmt.Sprintf("ingest: language %q already in family %q, cannot set %q", language, prev, family))
	}
	if family == "" {
		return
	}
	if familyByLang[language] == family {
		return
	}
	familyByLang[language] = family
	langsByFamily[family] = appendUnique(langsByFamily[family], language)
}

// FamilyForLanguage returns the family id for a language, or "" if standalone.
func FamilyForLanguage(language string) string {
	familyMu.RLock()
	defer familyMu.RUnlock()
	return familyByLang[language]
}

// FamilyForFile returns the family for the language owning filename's extension.
func FamilyForFile(filename string) (family string, ok bool) {
	lang, ok := LanguageForFile(filename)
	if !ok {
		return "", false
	}
	f := FamilyForLanguage(lang)
	return f, f != ""
}

// LanguagesInFamily returns language ids registered in family (stable order of registration).
func LanguagesInFamily(family string) []string {
	familyMu.RLock()
	defer familyMu.RUnlock()
	src := langsByFamily[family]
	if len(src) == 0 {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// SameFamily reports whether two language ids share a non-empty family.
func SameFamily(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	fa, fb := FamilyForLanguage(a), FamilyForLanguage(b)
	return fa != "" && fa == fb
}

// LanguageInFamily reports whether fileLanguage is a surface in projectFamily.
func LanguageInFamily(fileLanguage, projectFamily string) bool {
	if fileLanguage == "" || projectFamily == "" {
		return false
	}
	if f := FamilyForLanguage(fileLanguage); f != "" {
		return f == projectFamily
	}
	// Unfamilied language: treat language id as a singleton family.
	return fileLanguage == projectFamily
}

// IsKnownFamily reports whether any language has registered under family.
func IsKnownFamily(family string) bool {
	familyMu.RLock()
	defer familyMu.RUnlock()
	_, ok := langsByFamily[family]
	return ok
}

func appendUnique(slice []string, v string) []string {
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}
