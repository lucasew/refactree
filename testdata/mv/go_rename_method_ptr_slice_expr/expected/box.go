package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// (*pas)[1:][0] peels slice-of-pointer-to-collection under foreign same-leaf.
func UsePtrSliceExpr(pas *[]*A, pbs *[]*B) int {
	return (*pas)[1:][0].Execute() + (*pbs)[1:][0].Run()
}

func UsePtrSliceExprOnlyA(pas *[]*A) int {
	return (*pas)[1:][0].Execute()
}

func UsePreservesB(pbs *[]*B) int {
	return (*pbs)[1:][0].Run()
}
