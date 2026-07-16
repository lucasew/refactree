package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func Use(x any) int {
	switch v := x.(type) {
	case *A:
		// Target type — MUST rename.
		return v.Run()
	case *B:
		// Foreign same-leaf method — must NOT rename.
		return v.Run()
	default:
		return 0
	}
}
