package box

type A struct{}
type B struct{}

func (A) Execute() int { return 1 }
func (B) Run() int { return 2 }

func At[T any](xs []T, i int) T { return xs[i] }

func Use() int {
	return At([]A{{}}, 0).Execute() + At([]B{{}}, 0).Run()
}

func UseParam(as []A, bs []B) int {
	return At(as, 0).Execute() + At(bs, 0).Run()
}

func UseAssign() int {
	a := At([]A{{}}, 0)
	b := At([]B{{}}, 0)
	return a.Execute() + b.Run()
}

func UsePreservesB() int {
	return At([]B{{}}, 0).Run()
}
