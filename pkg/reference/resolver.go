package reference

import "path/filepath"

// Resolver performs provider-backed scope/symbol resolution in the context of a
// project root (e.g. serve --dir, browse root). Language/provider specifics are
// dispatched through registered providers; only this type carries RootDir.
type Resolver struct {
	// RootDir is the project/module working directory used by providers that
	// run tools like `go list` (empty means process cwd / global lookup only).
	RootDir string
}

// NewResolver builds a resolver for the given project root. Non-absolute paths
// are cleaned; errors are not returned here so callers can always construct one.
func NewResolver(rootDir string) *Resolver {
	if rootDir != "" {
		if abs, err := filepath.Abs(rootDir); err == nil {
			rootDir = abs
		}
	}
	return &Resolver{RootDir: rootDir}
}

// ResolveSymbolTarget dispatches to the provider named by ref.Provider.
func (r *Resolver) ResolveSymbolTarget(ref Reference) (SymbolTarget, bool, error) {
	if r == nil {
		r = &Resolver{}
	}
	provider, ok := ProviderForName(ref.Provider)
	if !ok {
		return SymbolTarget{}, false, nil
	}
	symbolProvider, ok := provider.(SymbolTargetProvider)
	if !ok {
		return SymbolTarget{}, false, nil
	}
	return symbolProvider.ResolveSymbolTarget(ref, r.RootDir)
}

// ResolveScopeTarget dispatches to the provider named by ref.Provider.
func (r *Resolver) ResolveScopeTarget(ref Reference) (ScopeTarget, bool, error) {
	if r == nil {
		r = &Resolver{}
	}
	provider, ok := ProviderForName(ref.Provider)
	if !ok {
		return ScopeTarget{}, false, nil
	}
	scopeProvider, ok := provider.(ScopeTargetProvider)
	if !ok {
		return ScopeTarget{}, false, nil
	}
	return scopeProvider.ResolveScopeTarget(ref, r.RootDir)
}

// ResolveScopeChildren dispatches to the provider named by ref.Provider.
func (r *Resolver) ResolveScopeChildren(ref Reference, includeHidden bool) ([]ScopeChild, bool, error) {
	if r == nil {
		r = &Resolver{}
	}
	provider, ok := ProviderForName(ref.Provider)
	if !ok {
		return nil, false, nil
	}
	childrenProvider, ok := provider.(ScopeChildrenProvider)
	if !ok {
		return nil, false, nil
	}
	return childrenProvider.ListScopeChildren(ref, r.RootDir, includeHidden)
}

// ResolveSymbolTarget resolves with an empty project root (process cwd only).
// Prefer NewResolver(root).ResolveSymbolTarget when a project dir is known.
func ResolveSymbolTarget(ref Reference) (SymbolTarget, bool, error) {
	return NewResolver("").ResolveSymbolTarget(ref)
}

// ResolveScopeTarget resolves with an empty project root (process cwd only).
// Prefer NewResolver(root).ResolveScopeTarget when a project dir is known.
func ResolveScopeTarget(ref Reference) (ScopeTarget, bool, error) {
	return NewResolver("").ResolveScopeTarget(ref)
}

// ResolveScopeChildren lists children with an empty project root.
// Prefer NewResolver(root).ResolveScopeChildren when a project dir is known.
func ResolveScopeChildren(ref Reference, includeHidden bool) ([]ScopeChild, bool, error) {
	return NewResolver("").ResolveScopeChildren(ref, includeHidden)
}
