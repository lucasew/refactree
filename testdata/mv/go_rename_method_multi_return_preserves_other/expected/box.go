package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func makeAB() (*A, *B) {
	return &A{}, &B{}
}

func makeNamed() (a *A, b *B) {
	return &A{}, &B{}
}

func makeAOK() (*A, bool) {
	return &A{}, true
}

func makeBOK() (*B, bool) {
	return &B{}, true
}

// Multi-return short-var from same-file helpers must bind positionally so
// foreign same-leaf methods on b are not rewritten.
func UseCall() int {
	a, b := makeAB()
	return a.Execute() + b.Run()
}

func UseNamedResultsCall() int {
	a, b := makeNamed()
	return a.Execute() + b.Run()
}

func UseCommaOK() int {
	a, ok := makeAOK()
	if !ok {
		return 0
	}
	b, ok2 := makeBOK()
	if !ok2 {
		return 0
	}
	return a.Execute() + b.Run()
}

func UseVarCall() int {
	var a, b = makeAB()
	return a.Execute() + b.Run()
}
