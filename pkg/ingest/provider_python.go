package ingest

type pythonReferenceProvider struct{}

func (pythonReferenceProvider) Name() string { return "python" }

func (pythonReferenceProvider) Resolve(spec string, ctx ImportResolveContext) (string, bool) {
	if ctx.KnownFiles[spec+".py"] {
		return FileRef("./" + spec + ".py"), true
	}
	if ctx.KnownFiles[spec+"/__init__.py"] {
		return FileRef("./" + spec), true
	}
	return "python:" + spec, true
}
