package apps

// Package-level helper calling foreign type — must NOT rename.
func useCmake() string {
	t := &cmakeTool{}
	return t.fetchManifest()
}

// Package-level helper calling renamed type — MUST rename.
func useClaude() string {
	t := &claudeTool{}
	return t.fetchManifest()
}

type otherType struct{}

func (o *otherType) Run() string {
	// Call on foreign type inside unrelated method — must NOT rename.
	t := &cmakeTool{}
	return t.fetchManifest()
}
