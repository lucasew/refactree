package ingest

import goref "github.com/lucasew/refactree/pkg/reference/go"

type goReferenceProvider struct{}

func (goReferenceProvider) Name() string { return "go" }

func (goReferenceProvider) Resolve(spec string, ctx ImportResolveContext) (string, bool) {
	return goref.ResolveImport(spec, ctx.KnownDirs), true
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
