package apps

type cmakeTool struct{}

func (t *cmakeTool) fetchManifest() string { return "m" }

func (t *cmakeTool) Run() string {
	// Self-call on foreign type — must NOT rename.
	_ = t.fetchManifest()
	// Cross-type call on the renamed type — MUST rename.
	c := &claudeTool{}
	return c.renamed()
}
