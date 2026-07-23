package pattern

import (
	"regexp"
	"sort"
	"strings"
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

// CaptureValues returns capture display texts for a match in the order of names
// (missing binds are empty strings). Multi-site captures join with ", ".
// source is the file bytes used for matching.
func CaptureValues(names []string, m Match, source []byte) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = formatCaptureSites(m.Captures[name], source)
	}
	return out
}

// PublicCaptures filters match captures for display (no ROOT/empty).
// Multi-site captures join with ", ".
func PublicCaptures(m Match, source []byte) map[string]string {
	out := make(map[string]string)
	for name, sites := range m.Captures {
		if name == "" || name == "ROOT" {
			continue
		}
		out[name] = formatCaptureSites(sites, source)
	}
	return out
}

func formatCaptureSites(sites []Span, source []byte) string {
	if len(sites) == 0 {
		return ""
	}
	if len(sites) == 1 {
		return sites[0].Text(source)
	}
	parts := make([]string, len(sites))
	for i, sp := range sites {
		parts[i] = sp.Text(source)
	}
	return strings.Join(parts, ", ")
}
