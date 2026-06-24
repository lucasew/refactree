package reference

import (
	"strings"
	"sync"
)

// ImportResolveContext carries filesystem and index metadata needed by
// language-specific provider backends to resolve cross-file import paths.
type ImportResolveContext struct {
	RootDir      string
	ImporterPath string
	KnownFiles   map[string]bool
	KnownDirs    map[string]bool
}

// Provider is the base interface for language-specific reference backends.
// It translates language-native import strings into canonical Reference strings.
type Provider interface {
	Name() string
	Resolve(spec string, ctx ImportResolveContext) (string, bool)
}

// SymbolTarget indicates the concrete disk directory mapped to a reference,
// allowing the ingest engine to parse the files and extract target symbols.
type SymbolTarget struct {
	Dir    string
	Symbol string
}

// ScopeTarget indicates the concrete disk directory mapped to a scope reference,
// primarily used to power directory listing or module browsing UIs.
type ScopeTarget struct {
	Dir string
	// CanDescend controls whether provider browse UIs should offer child scope
	// navigation from this target. Nil means unknown/default behavior.
	CanDescend *bool
}

// ScopeChildKind distinguishes whether a child item is a file or a subdirectory.
type ScopeChildKind int

const (
	ScopeChildDir ScopeChildKind = iota
	ScopeChildFile
)

// ScopeChild represents an individual navigable item inside a ScopeTarget,
// exposed as a Reference to the browse UI for progressive exploration.
type ScopeChild struct {
	Ref  Reference
	Kind ScopeChildKind
}

// SymbolTargetProvider is an optional interface capability for providers.
// When implemented, it enables the resolver to map a logical symbol reference
// to a concrete filesystem directory for code ingestion and doc generation.
// The rootDir context is injected by the Resolver.
type SymbolTargetProvider interface {
	ResolveSymbolTarget(ref Reference, rootDir string) (SymbolTarget, bool, error)
}

// ScopeTargetProvider is an optional interface capability for providers.
// When implemented, it enables the resolver to map a module or package reference
// to a concrete filesystem directory, allowing the UI to browse its contents.
type ScopeTargetProvider interface {
	ResolveScopeTarget(ref Reference, rootDir string) (ScopeTarget, bool, error)
}

// ScopeChildrenProvider is an optional interface capability for providers.
// When implemented, it allows a provider to take over child discovery, explicitly
// dictating the valid sub-modules or files exposed in the browse UI.
type ScopeChildrenProvider interface {
	ListScopeChildren(ref Reference, rootDir string, includeHidden bool) ([]ScopeChild, bool, error)
}

var (
	providersMu sync.RWMutex
	providers   = map[string]Provider{}
)

// RegisterProvider adds or replaces a provider instance in the global registry.
// This function is thread-safe and is typically called during package init().
func RegisterProvider(name string, provider Provider) {
	name = strings.ToLower(name)
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[name] = provider
}

// ProviderForName safely retrieves a registered provider backend by its lowercase name.
func ProviderForName(name string) (Provider, bool) {
	name = strings.ToLower(name)
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}
