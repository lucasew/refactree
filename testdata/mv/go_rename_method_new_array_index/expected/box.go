package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// new([n]*T) short-var / var / inline index must follow the array element type
// so foreign same-leaf methods on the other pointer-to-array are not rewritten.
func UseNewShort() int {
	as := new([1]*A)
	bs := new([1]*B)
	return as[0].Execute() + bs[0].Run()
}

func UseNewVar() int {
	var as = new([1]*A)
	var bs = new([1]*B)
	return as[0].Execute() + bs[0].Run()
}

func UseNewInline() int {
	return new([1]*A)[0].Execute() + new([1]*B)[0].Run()
}

func UseNewParenType() int {
	as := new(([1]*A))
	bs := new(([1]*B))
	return as[0].Execute() + bs[0].Run()
}
