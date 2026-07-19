package box

type A struct{}
type B struct{}

func (A) Run() int { return 1 }
func (B) Run() int { return 2 }

func First[T any](xs []T) T { return xs[0] }

func Use() int {
	return First([]A{{}}).Run() + First([]B{{}}).Run()
}

func UseParam(as []A, bs []B) int {
	return First(as).Run() + First(bs).Run()
}

func UseAssign() int {
	a := First([]A{{}})
	b := First([]B{{}})
	return a.Run() + b.Run()
}

func UseAssignParam(as []A, bs []B) int {
	a := First(as)
	b := First(bs)
	return a.Run() + b.Run()
}

func UsePreservesB() int {
	return First([]B{{}}).Run()
}
