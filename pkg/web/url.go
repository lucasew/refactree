package web

import (
	"net/url"
	"strings"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/web/annotate"
)

// CodePathPrefix is the route prefix for code views: /code/<reference>
const CodePathPrefix = "/code/"

// EncodeCodeURL builds /code/<url-encoded-reference>[#anchor].
// The reference itself uniquely identifies the code unit via the path system.
func EncodeCodeURL(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return CodePathPrefix
	}
	parsed := ingest.ParseReference(ref)
	base := parsed
	base.Symbol = ""
	fileRef := base.String()
	path := CodePathPrefix + url.PathEscape(fileRef)
	if parsed.Symbol != "" {
		path += "#" + annotate.AnchorID(ref)
	}
	return path
}

// DecodeCodePath extracts the reference from a /code/... request path.
// Accepts both encoded and plain path segments.
func DecodeCodePath(requestPath string) (string, bool) {
	if !strings.HasPrefix(requestPath, CodePathPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(requestPath, CodePathPrefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", false
	}
	// Single segment is URL-encoded reference.
	if !strings.Contains(rest, "/") {
		decoded, err := url.PathUnescape(rest)
		if err != nil {
			return rest, true
		}
		return decoded, true
	}
	// Multi-segment fallback: treat as path:./<joined>
	decoded, err := url.PathUnescape(rest)
	if err != nil {
		decoded = rest
	}
	if !strings.Contains(decoded, ":") {
		return "path:./" + strings.TrimPrefix(decoded, "./"), true
	}
	return decoded, true
}

// FileReferenceForView returns the file-level reference (symbol stripped) for loading source.
func FileReferenceForView(ref string) ingest.Reference {
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

// EncodeProviderFileURL links to a source file inside a provider package scope.
// Uses ?file= because provider refs address packages/modules, not individual files.
func EncodeProviderFileURL(scopeRef ingest.Reference, fileName string) string {
	scopeRef.Symbol = ""
	base := EncodeCodeURL(scopeRef.String())
	// Strip any accidental fragment from EncodeCodeURL (none without symbol).
	if i := strings.Index(base, "#"); i >= 0 {
		base = base[:i]
	}
	q := url.Values{}
	q.Set("file", fileName)
	return base + "?" + q.Encode()
}
