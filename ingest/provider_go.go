package ingest

import goref "github.com/lucasew/refactree/pkg/reference/go"

type goReferenceProvider struct{}

func (goReferenceProvider) Name() string { return "go" }

func (goReferenceProvider) Resolve(spec string, ctx ImportResolveContext) (string, bool) {
	return goref.ResolveImport(spec, ctx.KnownDirs), true
}

func (goReferenceProvider) ResolveScopeTarget(ref Reference) (ProviderScopeTarget, bool, error) {
	if ref.Path == "" {
		return ProviderScopeTarget{}, false, nil
	}
	dir, err := goref.ResolvePackageDir(ref.Path)
	if err != nil {
		return ProviderScopeTarget{}, true, err
	}
	return ProviderScopeTarget{Dir: dir}, true, nil
}

func (goReferenceProvider) ResolveSymbolTarget(ref Reference) (ProviderSymbolTarget, bool, error) {
	target, ok, err := goref.ResolveSymbolTarget(ref.Path, ref.Symbol)
	if !ok || err != nil {
		return ProviderSymbolTarget{}, ok, err
	}
	return ProviderSymbolTarget{
		Dir:    target.Dir,
		Symbol: target.Symbol,
	}, true, nil
}
