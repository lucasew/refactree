package apps

type claudeTool struct{}

func (t *claudeTool) renamed() string { return "c" }

func (t *claudeTool) Run() string {
	return t.renamed()
}
