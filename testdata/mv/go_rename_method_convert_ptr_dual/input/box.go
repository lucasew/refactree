package box

type A int
type B int

func (*A) Run() int { return 1 }
func (*B) Run() int { return 2 }

func Use(p *int, q *int) int {
	return (*A)(p).Run() + (*B)(q).Run()
}

func UsePreservesB(q *int) int {
	return (*B)(q).Run()
}
