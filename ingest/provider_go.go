package ingest

type goReferenceProvider struct{}

func (goReferenceProvider) Name() string { return "go" }

func (goReferenceProvider) Resolve(spec string, ctx ImportResolveContext) (string, bool) {
	last := lastPathComponent(spec)
	if ctx.KnownDirs[last] {
		return FileRef("./" + last), true
	}
	return "go:" + last, true
}
