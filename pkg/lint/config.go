package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/pattern"
	"gopkg.in/yaml.v3"
)

// ConfigFileName is the default project rulebook filename.
const ConfigFileName = "refactree.yaml"

// BuiltinDeadImports is the catalog id for full-file unused named import prune
// via ingest.ImportHygiene.PruneNamedUnusedEdits (never barrels).
const BuiltinDeadImports = "dead-imports"

// Config is the decoded refactree.yaml root.
type Config struct {
	Rules []RuleSpec `yaml:"rules"`
}

// RuleSpec is one catalog entry (YAML).
//
// Pattern rules: require pattern + (language|family); optional replacement.
// Builtin rules: require builtin (e.g. dead-imports); pattern/replacement must
// be empty; language|family optional (dead-imports defaults to every language
// with ImportHygiene).
type RuleSpec struct {
	ID          string `yaml:"id"`
	Language    string `yaml:"language,omitempty"`
	Family      string `yaml:"family,omitempty"`
	Pattern     string `yaml:"pattern,omitempty"`
	Message     string `yaml:"message"`
	Level       string `yaml:"level,omitempty"`
	Replacement string `yaml:"replacement,omitempty"`
	Builtin     string `yaml:"builtin,omitempty"`
}

// CompiledRule is a validated RuleSpec with parsed pattern IR (and optional rewrite Rule).
type CompiledRule struct {
	Spec    RuleSpec
	Level   string // normalized: error | warning | note
	Pattern pattern.Node
	// Rule is non-nil when Replacement is set (pattern rules only).
	Rule *pattern.Rule
	// Builtin is non-empty for engine builtins (e.g. BuiltinDeadImports).
	Builtin string
}

// DefaultConfig is used when no refactree.yaml is found (and --config is not set).
func DefaultConfig() Config {
	return Config{
		Rules: []RuleSpec{{
			ID:      "imports/unused-named",
			Builtin: BuiltinDeadImports,
			Message: "Unused named import",
			Level:   "warning",
		}},
	}
}

// FindConfig walks up from startDir looking for refactree.yaml.
// Returns the absolute path of the first hit, or "" if none.
func FindConfig(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, ConfigFileName)
		st, err := os.Stat(cand)
		if err == nil && !st.IsDir() {
			return cand, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// ResolveConfigPath returns the config file to load.
// If override is non-empty, it is used (absolute) and missing file is an error.
// If override is empty, walks up from startDir; missing file returns ("", nil)
// so the caller can use DefaultConfig.
func ResolveConfigPath(startDir, override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		p, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		st, err := os.Stat(p)
		if err != nil {
			return "", fmt.Errorf("config %s: %w", p, err)
		}
		if st.IsDir() {
			return "", fmt.Errorf("config %s: is a directory", p)
		}
		return p, nil
	}
	found, err := FindConfig(startDir)
	if err != nil {
		return "", err
	}
	return found, nil
}

// LoadCatalog resolves config for a lint run: --config path, walk-up
// refactree.yaml, or DefaultConfig when none is found.
// path is empty when using defaults; fromDefault is true in that case.
func LoadCatalog(startDir, override string) (path string, rules []CompiledRule, fromDefault bool, err error) {
	path, err = ResolveConfigPath(startDir, override)
	if err != nil {
		return "", nil, false, err
	}
	if path == "" {
		_, rules, err = LoadDefault()
		return "", rules, true, err
	}
	_, rules, err = LoadFile(path)
	return path, rules, false, err
}

// LoadDefault compiles DefaultConfig().
func LoadDefault() (Config, []CompiledRule, error) {
	cfg := DefaultConfig()
	rules, err := CompileRules(cfg.Rules)
	return cfg, rules, err
}

// LoadFile reads and validates a refactree.yaml path.
func LoadFile(path string) (Config, []CompiledRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, nil, err
	}
	return Load(data)
}

// Load decodes and validates YAML bytes.
func Load(data []byte) (Config, []CompiledRule, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Rules == nil {
		cfg.Rules = []RuleSpec{}
	}
	compiled, err := CompileRules(cfg.Rules)
	if err != nil {
		return Config{}, nil, err
	}
	return cfg, compiled, nil
}

// CompileRules validates rule specs and parses pattern/replacement IR or builtins.
func CompileRules(specs []RuleSpec) ([]CompiledRule, error) {
	seen := make(map[string]int, len(specs))
	out := make([]CompiledRule, 0, len(specs))
	for i, spec := range specs {
		id := strings.TrimSpace(spec.ID)
		if id == "" {
			return nil, fmt.Errorf("rules[%d]: id is required", i)
		}
		if prev, ok := seen[id]; ok {
			return nil, fmt.Errorf("rules[%d]: duplicate id %q (also rules[%d])", i, id, prev)
		}
		seen[id] = i

		lang := strings.TrimSpace(spec.Language)
		fam := strings.TrimSpace(spec.Family)
		builtin := strings.TrimSpace(spec.Builtin)
		patStr := strings.TrimSpace(spec.Pattern)
		repl := strings.TrimSpace(spec.Replacement)
		msg := strings.TrimSpace(spec.Message)
		if msg == "" {
			return nil, fmt.Errorf("rule %q: message is required", id)
		}

		level, err := normalizeLevel(spec.Level)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", id, err)
		}

		if builtin != "" {
			if patStr != "" || repl != "" {
				return nil, fmt.Errorf("rule %q: builtin rules cannot set pattern or replacement", id)
			}
			if lang != "" && fam != "" {
				return nil, fmt.Errorf("rule %q: set only one of language or family", id)
			}
			if err := validateBuiltin(builtin); err != nil {
				return nil, fmt.Errorf("rule %q: %w", id, err)
			}
			out = append(out, CompiledRule{
				Spec: RuleSpec{
					ID:       id,
					Language: lang,
					Family:   fam,
					Message:  msg,
					Level:    level,
					Builtin:  builtin,
				},
				Level:   level,
				Builtin: builtin,
			})
			continue
		}

		switch {
		case lang != "" && fam != "":
			return nil, fmt.Errorf("rule %q: set only one of language or family", id)
		case lang == "" && fam == "":
			return nil, fmt.Errorf("rule %q: language or family is required", id)
		}
		if patStr == "" {
			return nil, fmt.Errorf("rule %q: pattern is required", id)
		}

		cr := CompiledRule{
			Spec: RuleSpec{
				ID:          id,
				Language:    lang,
				Family:      fam,
				Pattern:     patStr,
				Message:     msg,
				Level:       level,
				Replacement: repl,
			},
			Level: level,
		}

		if repl != "" {
			rule, err := pattern.RuleFromStrings(patStr, repl)
			if err != nil {
				return nil, fmt.Errorf("rule %q: %w", id, err)
			}
			cr.Rule = &rule
			cr.Pattern = rule.Pattern
		} else {
			pat, err := pattern.ParsePattern(patStr)
			if err != nil {
				return nil, fmt.Errorf("rule %q: pattern: %w", id, err)
			}
			cr.Pattern = pat
		}
		out = append(out, cr)
	}
	return out, nil
}

func validateBuiltin(name string) error {
	switch name {
	case BuiltinDeadImports:
		return nil
	default:
		return fmt.Errorf("unknown builtin %q", name)
	}
}

func normalizeLevel(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "warning", nil
	}
	switch s {
	case "error", "warning", "note":
		return s, nil
	default:
		return "", fmt.Errorf("level %q (want error, warning, or note)", s)
	}
}

// AppliesToFile reports whether the rule targets this file language.
func (r CompiledRule) AppliesToFile(fileLang string) bool {
	if fileLang == "" {
		return false
	}
	if r.Builtin == BuiltinDeadImports {
		if r.Spec.Language == "" && r.Spec.Family == "" {
			_, ok := ingest.ImportHygieneForLanguage(fileLang)
			return ok
		}
	}
	if r.Spec.Language != "" {
		return fileLang == r.Spec.Language
	}
	if r.Spec.Family != "" {
		return ingest.LanguageInFamily(fileLang, r.Spec.Family)
	}
	return false
}
