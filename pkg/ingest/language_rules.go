package ingest

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// LanguageRules captures language-specific behavior knobs shared by ingest
// callers (CLI/UI) and core resolution logic.
type LanguageRules struct {
	// Extensions maps file suffixes (for example .go, .py, .js) to the language.
	Extensions []string
	// DirectoryModule reports whether a directory can be treated as a symbol
	// container (for example path:./dir::Symbol).
	DirectoryModule bool
	// Family groups related surfaces that share a module lattice (for example
	// FamilyJVM for java; FamilyECMA for javascript). Empty means standalone.
	// Prefer honest language ids per surface; use Family for shared behavior.
	Family string
}

var (
	languageRulesMu    sync.RWMutex
	languageRulesByKey = map[string]LanguageRules{}
	languageByExt      = map[string]string{}
)

// RegisterLanguageRules registers per-language rules and file extension
// ownership. It panics on empty names, duplicate languages, or conflicting
// extension ownership.
func RegisterLanguageRules(name string, rules LanguageRules) {
	if name == "" {
		panic("ingest: RegisterLanguageRules with empty name")
	}

	normalized := LanguageRules{
		DirectoryModule: rules.DirectoryModule,
		Extensions:      normalizeExtensions(rules.Extensions),
		Family:          strings.TrimSpace(rules.Family),
	}

	languageRulesMu.Lock()
	if _, exists := languageRulesByKey[name]; exists {
		languageRulesMu.Unlock()
		panic(fmt.Sprintf("ingest: language rules %q already registered", name))
	}
	for _, ext := range normalized.Extensions {
		if owner, exists := languageByExt[ext]; exists && owner != name {
			languageRulesMu.Unlock()
			panic(fmt.Sprintf("ingest: extension %q already registered by %q", ext, owner))
		}
	}

	languageRulesByKey[name] = normalized
	for _, ext := range normalized.Extensions {
		languageByExt[ext] = name
	}
	family := normalized.Family
	languageRulesMu.Unlock()

	// Family map uses its own lock; register after rules are visible.
	if family != "" {
		RegisterLanguageFamily(name, family)
	}
}

// LanguageForFile returns the registered language for filename extension.
func LanguageForFile(filename string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return "", false
	}

	languageRulesMu.RLock()
	defer languageRulesMu.RUnlock()
	lang, ok := languageByExt[ext]
	return lang, ok
}

// LanguageUsesDirectoryModule reports whether the language treats directories
// as symbol containers.
func LanguageUsesDirectoryModule(language string) bool {
	languageRulesMu.RLock()
	defer languageRulesMu.RUnlock()
	rules, ok := languageRulesByKey[language]
	return ok && rules.DirectoryModule
}

func normalizeExtensions(exts []string) []string {
	if len(exts) == 0 {
		return nil
	}

	out := make([]string, 0, len(exts))
	seen := map[string]bool{}
	for _, raw := range exts {
		ext := strings.ToLower(strings.TrimSpace(raw))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if seen[ext] {
			continue
		}
		seen[ext] = true
		out = append(out, ext)
	}
	return out
}
