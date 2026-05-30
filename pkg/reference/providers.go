package reference

import (
	"strings"
	"sync"
)

// ImportResolveContext carries filesystem and index metadata needed by
// provider-backed import resolution.
type ImportResolveContext struct {
	RootDir      string
	ImporterPath string
	KnownFiles   map[string]bool
	KnownDirs    map[string]bool
}

// Provider resolves import specs into canonical references.
type Provider interface {
	Name() string
	Resolve(spec string, ctx ImportResolveContext) (string, bool)
}

// SymbolTarget points to a directory that can be ingested for provider-backed
// symbol lookup.
type SymbolTarget struct {
	Dir    string
	Symbol string
}

// ScopeTarget points to a directory that can be ingested for provider-backed
// scope operations such as listing.
type ScopeTarget struct {
	Dir string
}

// SymbolTargetProvider is an optional provider capability used by doc lookup.
type SymbolTargetProvider interface {
	ResolveSymbolTarget(ref Reference) (SymbolTarget, bool, error)
}

// ScopeTargetProvider is an optional provider capability used by listing.
type ScopeTargetProvider interface {
	ResolveScopeTarget(ref Reference) (ScopeTarget, bool, error)
}

var (
	providersMu sync.RWMutex
	providers   = map[string]Provider{}
)

// RegisterProvider registers or overrides a provider name.
func RegisterProvider(name string, provider Provider) {
	name = strings.ToLower(name)
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[name] = provider
}

// ProviderForName looks up a provider by name.
func ProviderForName(name string) (Provider, bool) {
	name = strings.ToLower(name)
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}

// ResolveSymbolTarget tries to resolve provider-backed symbol lookup target.
func ResolveSymbolTarget(ref Reference) (SymbolTarget, bool, error) {
	provider, ok := ProviderForName(ref.Provider)
	if !ok {
		return SymbolTarget{}, false, nil
	}

	symbolProvider, ok := provider.(SymbolTargetProvider)
	if !ok {
		return SymbolTarget{}, false, nil
	}

	return symbolProvider.ResolveSymbolTarget(ref)
}

// ResolveScopeTarget tries to resolve provider-backed listing scope.
func ResolveScopeTarget(ref Reference) (ScopeTarget, bool, error) {
	provider, ok := ProviderForName(ref.Provider)
	if !ok {
		return ScopeTarget{}, false, nil
	}

	scopeProvider, ok := provider.(ScopeTargetProvider)
	if !ok {
		return ScopeTarget{}, false, nil
	}

	return scopeProvider.ResolveScopeTarget(ref)
}
