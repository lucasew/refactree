package apps

type cmakeTool struct{}

func (t *cmakeTool) fetchManifest() string { return "m" }

func (t *cmakeTool) Run() string {
	return t.fetchManifest()
}
