package ingest

import (
	"fmt"
	"strings"
	"sync"
)

// Well-known language families. Surfaces (java, kotlin, …) register under a family
// so move/module models and multi-file projects can share lattice behavior while
// keeping honest per-file language ids.
const (
	FamilyJVM  = "jvm"  // Java today; Kotlin later
	FamilyECMA = "ecma" // JavaScript / TypeScript / TSX / JSX
)

var (
	familyMu        sync.RWMutex
	familyByLang    = map[string]string{} // language id → family
	langsByFamily   = map[string][]string{}
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

// LanguageMatchesProject reports whether a file's language belongs to a catalog
// project language (exact match or same family).
func LanguageMatchesProject(fileLanguage, projectLanguage string) bool {
	if fileLanguage == "" || projectLanguage == "" {
		return false
	}
	if fileLanguage == projectLanguage {
		return true
	}
	return SameFamily(fileLanguage, projectLanguage)
}

func appendUnique(slice []string, v string) []string {
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}

