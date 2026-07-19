package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Type assert/convert collection locals and inline index operands must
// follow element/value type so foreign same-leaf methods are not rewritten.
func UseAssertShort(x, y any) int {
	as := x.([]*A)
	bs := y.([]*B)
	return as[0].Execute() + bs[0].Run()
}

func UseAssertOK(x, y any) int {
	as, ok1 := x.([]*A)
	bs, ok2 := y.([]*B)
	_, _ = ok1, ok2
	return as[0].Execute() + bs[0].Run()
}

func UseAssertVar(x, y any) int {
	var as = x.([]*A)
	var bs = y.([]*B)
	return as[0].Execute() + bs[0].Run()
}

func UseConvertShort(x, y any) int {
	as := ([]*A)(x)
	bs := ([]*B)(y)
	return as[0].Execute() + bs[0].Run()
}

func UseAssertMap(x, y any) int {
	ma := x.(map[string]*A)
	mb := y.(map[string]*B)
	return ma["k"].Execute() + mb["k"].Run()
}

func UseAssertInline(x, y any) int {
	return x.([]*A)[0].Execute() + y.([]*B)[0].Run()
}

func UseConvertInline(x, y any) int {
	return ([]*A)(x)[0].Execute() + ([]*B)(y)[0].Run()
}
