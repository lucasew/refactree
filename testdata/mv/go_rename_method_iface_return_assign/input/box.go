package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type GA interface{ Get() *A }
type GB interface{ Get() *B }

func Use(ga GA, gb GB) int {
	a := ga.Get()
	b := gb.Get()
	return a.Run() + b.Run()
}

func UseVar(ga GA, gb GB) int {
	var a = ga.Get()
	var b = gb.Get()
	return a.Run() + b.Run()
}

func UsePreservesB(gb GB) int {
	b := gb.Get()
	return b.Run()
}
