package apps

type claudeTool struct{}

func (t *claudeTool) fetchManifest() string { return "c" }

func (t *claudeTool) Run() string {
	return t.fetchManifest()
}
