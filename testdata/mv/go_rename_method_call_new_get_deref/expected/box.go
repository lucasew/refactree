package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func getA() *A { return &A{} }
func getB() *B { return &B{} }

// Inline new(T).M under foreign same-leaf methods.
func UseNew() int {
	return new(A).Execute() + new(B).Run()
}

// Nested paren / parenthesized type arg.
func UseNewParen() int {
	return (new(A)).Execute() + (new(B)).Run()
}

// Same-file helper return as method receiver.
func UseGet() int {
	return getA().Execute() + getB().Run()
}

// Pointer dereference of typed params.
func UseStar(pa *A, pb *B) int {
	return (*pa).Execute() + (*pb).Run()
}

// Regression: assigned forms already peel types.
func UseNewVar() int {
	a := new(A)
	b := new(B)
	return a.Execute() + b.Run()
}

func UseGetVar() int {
	a := getA()
	b := getB()
	return a.Execute() + b.Run()
}

// Regression: composite already peels.
func UseComposite() int {
	return (&A{}).Execute() + (&B{}).Run()
}
