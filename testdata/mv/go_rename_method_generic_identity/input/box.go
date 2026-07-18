package box

type A struct{}
type B struct{}

func (A) Run() int { return 1 }
func (B) Run() int { return 2 }

func Id[T any](v T) T { return v }

func Use() int {
	return Id(A{}).Run() + Id(B{}).Run()
}

func UsePtr() int {
	return Id(&A{}).Run() + Id(&B{}).Run()
}

func UseAssign() int {
	a := Id(A{})
	b := Id(B{})
	return a.Run() + b.Run()
}

func UsePreservesB() int {
	return Id(B{}).Run()
}
