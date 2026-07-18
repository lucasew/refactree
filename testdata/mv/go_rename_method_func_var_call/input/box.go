package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Local func var call under foreign same-leaf methods.
func UseFuncVar() int {
	f := func() *A { return &A{} }
	g := func() *B { return &B{} }
	return f().Run() + g().Run()
}

func UseVarFunc() int {
	var f = func() *A { return &A{} }
	var g = func() *B { return &B{} }
	return f().Run() + g().Run()
}

func UseValueRecvFunc() int {
	f := func() A { return A{} }
	g := func() B { return B{} }
	return f().Run() + g().Run()
}

func UsePreservesB() int {
	g := func() *B { return &B{} }
	return g().Run()
}
