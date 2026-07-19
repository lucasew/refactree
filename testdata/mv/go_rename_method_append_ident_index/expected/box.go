package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// append(ident, …) short-var / var / inline index must follow the collection
// ident's element type so foreign same-leaf methods are not rewritten.
func UseAppendParam(as []*A, bs []*B) int {
	as2 := append(as, &A{})
	bs2 := append(bs, &B{})
	return as2[0].Execute() + bs2[0].Run()
}

func UseAppendParamVar(as []*A, bs []*B) int {
	var as2 = append(as, &A{})
	var bs2 = append(bs, &B{})
	return as2[0].Execute() + bs2[0].Run()
}

func UseAppendParamInline(as []*A, bs []*B) int {
	return append(as, &A{})[0].Execute() + append(bs, &B{})[0].Run()
}

func UseAppendLocal() int {
	as := []*A{&A{}}
	bs := []*B{&B{}}
	as2 := append(as, &A{})
	bs2 := append(bs, &B{})
	return as2[0].Execute() + bs2[0].Run()
}

func UseAppendChained(as []*A, bs []*B) int {
	as2 := append(append(as, &A{}), &A{})
	bs2 := append(append(bs, &B{}), &B{})
	return as2[0].Execute() + bs2[0].Run()
}
