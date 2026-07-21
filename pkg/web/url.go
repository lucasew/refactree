package web

import (
	"net/url"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/web/annotate"
)

// CodePathPrefix is the route prefix for code views: /code/<reference>
const CodePathPrefix = "/code/"

// EncodeCodeURL builds /code/<url-encoded-full-reference>[#anchor].
// The reference itself uniquely identifies the code unit via the path system.
// Symbol refs keep the ::symbol in the path; the fragment scrolls to the definition.
func EncodeCodeURL(ref string) string {
	return encodeCodeURLRef(ref)
}

// EncodeCodeURLInRoot builds a code URL for a reference.
// The rootDir parameter is kept for call-site stability and possible future use;
// it is currently ignored. Encoding matches EncodeCodeURL(ref).
//
// CanonicalizeReference is deliberately not used here: it runs Seed Materialize,
// and chasing import aliases into node_modules can take tens of seconds per
// hyperlink during annotate. The page request already canonicalizes the primary
// ref; link targets use the references produced by the current ingest result.
func EncodeCodeURLInRoot(rootDir, ref string) string {
	_ = rootDir
	return EncodeCodeURL(ref)
}

func encodeCodeURLRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return CodePathPrefix
	}
	path := CodePathPrefix + url.PathEscape(ref)
	parsed := ingest.ParseReference(ref)
	if parsed.Symbol != "" {
		path += "#" + annotate.AnchorID(ref)
	}
	return path
}

// DecodeCodePath extracts the reference from a /code/... request path.
// Accepts both encoded and plain path segments. Path results always include
// the path:./ prefix (never bare "cmd" / "cmd/rft").
func DecodeCodePath(requestPath string) (string, bool) {
	if !strings.HasPrefix(requestPath, CodePathPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(requestPath, CodePathPrefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", false
	}
	// Single segment is URL-encoded reference (may include ::symbol).
	if !strings.Contains(rest, "/") {
		decoded, err := url.PathUnescape(rest)
		if err != nil {
			decoded = rest
		}
		return canonicalizeDecodedRef(decoded), true
	}
	// Multi-segment fallback: treat as path:./<joined>
	decoded, err := url.PathUnescape(rest)
	if err != nil {
		decoded = rest
	}
	if !strings.Contains(decoded, ":") {
		return "path:./" + strings.TrimPrefix(decoded, "./"), true
	}
	return canonicalizeDecodedRef(decoded), true
}

// canonicalizeDecodedRef ensures path refs keep provider + ./ prefix.
func canonicalizeDecodedRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "path:./"
	}
	r := ingest.ParseReference(ref)
	if r.Provider == "" {
		// Bare cmd or cmd/rft from a non-encoded /code/cmd URL.
		r.Provider = "path"
		if r.Path == "" {
			r.Path = "./"
		} else if !strings.HasPrefix(r.Path, "./") && !strings.HasPrefix(r.Path, "../") && !strings.HasPrefix(r.Path, "/") {
			r.Path = "./" + r.Path
		}
	}
	if strings.EqualFold(r.Provider, "path") {
		if r.Path == "" || r.Path == "." {
			r.Path = "./"
		} else if !strings.HasPrefix(r.Path, "./") && !strings.HasPrefix(r.Path, "../") && !strings.HasPrefix(r.Path, "/") {
			r.Path = "./" + r.Path
		}
		r.Provider = "path"
	}
	return r.String()
}

// ScopeReferenceForView returns the scope-level reference (symbol stripped).
func ScopeReferenceForView(ref string) ingest.Reference {
	r := ingest.ParseReference(ref)
	r.Symbol = ""
	if r.Provider == "" {
		r.Provider = "path"
	}
	if r.Path == "" {
		r.Path = "./"
	}
	return r
}
