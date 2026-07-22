package pattern

import (
	"regexp"
	"sort"
)

// PatternNeedsLinks reports whether matching requires ingest hyperlink targets
// (any @ref / kind "ref" in the pattern). Pure token patterns skip materialize.
func PatternNeedsLinks(pat Node) bool {
	var walk func(Node) bool
	walk = func(n Node) bool {
		if n.Kind == "ref" || n.Ref != "" {
			return true
		}
		if n.Callee != nil && walk(*n.Callee) {
			return true
		}
		for _, a := range n.Args {
			if walk(a) {
				return true
			}
		}
		return false
	}
	return walk(pat)
}

// CaptureNames returns the statically known capture variable names declared by
// the pattern IR, in stable sorted order. Includes:
//   - $name / $name:@ref / $name:/re/ / $name:{…}  (Node.As)
//   - named regex groups (?P<rest>…) inside /regex/
//
// Excludes empty names and the internal "ROOT" binder.
func CaptureNames(pat Node) []string {
	seen := map[string]struct{}{}
	var walk func(Node)
	walk = func(n Node) {
		if n.As != "" && n.As != "ROOT" && n.As != "_" {
			seen[n.As] = struct{}{}
		}
		if n.Regex != "" {
			if re, err := regexp.Compile(n.Regex); err == nil {
				for i, name := range re.SubexpNames() {
					if i > 0 && name != "" {
						seen[name] = struct{}{}
					}
				}
			}
		}
		if n.Callee != nil {
			walk(*n.Callee)
		}
		for _, a := range n.Args {
			walk(a)
		}
	}
	walk(pat)
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CaptureValues returns capture values for a match in the order of names
// (missing binds are empty strings). Skips ROOT/empty keys in m.Captures when
// names is built from CaptureNames.
func CaptureValues(names []string, m Match) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = m.Captures[name]
	}
	return out
}

// PublicCaptures filters match captures for display (no ROOT/empty).
func PublicCaptures(m Match) map[string]string {
	out := make(map[string]string)
	for name, val := range m.Captures {
		if name == "" || name == "ROOT" {
			continue
		}
		out[name] = val
	}
	return out
}
