package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Named result params must keep foreign same-leaf methods on b.
func UseNamed() (a *A, b *B) {
	a, b = &A{}, &B{}
	_ = a.Execute() + b.Run()
	return
}

// Multi-value short-var with heterogeneous RHS types binds positionally.
func UseMultiShort() int {
	a, b := &A{}, &B{}
	return a.Execute() + b.Run()
}

// Same-type multi short-var renames both.
func UseMultiSame() int {
	a, other := &A{}, &A{}
	return a.Execute() + other.Execute()
}

// Type-assert comma-ok: only the value is our type.
func UseAssert(x any, y any) int {
	a, ok := x.(*A)
	if !ok {
		return 0
	}
	b, ok2 := y.(*B)
	if !ok2 {
		return 0
	}
	return a.Execute() + b.Run()
}
