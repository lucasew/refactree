package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Nested function-literal params — foreign same-leaf must stay put.
func UseNested() int {
	f := func(a *A, b *B) int {
		return a.Execute() + b.Run()
	}
	return f(&A{}, &B{})
}

// Immediately-invoked function literal.
func UseIIFE() int {
	return func(a *A, b *B) int {
		return a.Execute() + b.Run()
	}(&A{}, &B{})
}

// Package-level function literal.
var Handler = func(a *A, b *B) int {
	return a.Execute() + b.Run()
}
