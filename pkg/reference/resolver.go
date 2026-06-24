package reference

import "path/filepath"

// Resolver routes reference lookups to the correct language-specific provider,
// carrying the operational context (such as the project's root directory).
// Passing the root directory allows backend tools (like `go list` or python venv)
// to accurately resolve references relative to a specific project.
type Resolver struct {
	// RootDir is the project/module working directory passed to providers.
	// If empty, providers fall back to process cwd or global module cache resolution.
	RootDir string
}

// NewResolver instantiates a Resolver bound to the specified root directory.
// It normalizes non-absolute paths, ensuring consistent path operations downstream.
func NewResolver(rootDir string) *Resolver {
	if rootDir != "" {
		if abs, err := filepath.Abs(rootDir); err == nil {
			rootDir = abs
		}
	}
	return &Resolver{RootDir: rootDir}
}

// ResolveSymbolTarget routes the reference to the corresponding SymbolTargetProvider.
// It returns the physical directory containing the symbol, and a boolean indicating
// whether the provider supported the lookup.
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

// ResolveScopeTarget routes the reference to the corresponding ScopeTargetProvider.
// It returns the physical directory matching the scope, enabling package/module browsing.
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

// ResolveScopeChildren routes the reference to the corresponding ScopeChildrenProvider.
// It enables the provider to explicitly list sub-modules or files for the UI to display.
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

// ResolveSymbolTarget is a package-level helper that invokes the symbol resolver
// using an empty project root (relying purely on process cwd or global lookups).
// When exploring an active project, NewResolver(root).ResolveSymbolTarget is preferred.
func ResolveSymbolTarget(ref Reference) (SymbolTarget, bool, error) {
	return NewResolver("").ResolveSymbolTarget(ref)
}

// ResolveScopeTarget is a package-level helper that invokes the scope resolver
// using an empty project root (relying purely on process cwd or global lookups).
// When exploring an active project, NewResolver(root).ResolveScopeTarget is preferred.
func ResolveScopeTarget(ref Reference) (ScopeTarget, bool, error) {
	return NewResolver("").ResolveScopeTarget(ref)
}

// ResolveScopeChildren is a package-level helper that invokes child discovery
// using an empty project root (relying purely on process cwd or global lookups).
// When exploring an active project, NewResolver(root).ResolveScopeChildren is preferred.
func ResolveScopeChildren(ref Reference, includeHidden bool) ([]ScopeChild, bool, error) {
	return NewResolver("").ResolveScopeChildren(ref, includeHidden)
}
