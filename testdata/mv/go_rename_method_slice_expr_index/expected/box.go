package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Slice-expression index locals and inline as[:n][0] must follow the sliced
// collection's element type so foreign same-leaf methods are not rewritten.
func UseSliceInline(as []*A, bs []*B) int {
	return as[:1][0].Execute() + bs[:1][0].Run()
}

func UseSliceForms(as []*A, bs []*B) int {
	return as[1:][0].Execute() + bs[1:][0].Run() + as[:][0].Execute() + bs[:][0].Run() + as[0:1][0].Execute() + bs[0:1][0].Run()
}

func UseSliceShort(as []*A, bs []*B) int {
	sa := as[:1]
	sb := bs[:1]
	return sa[0].Execute() + sb[0].Run()
}

func UseSliceVar(as []*A, bs []*B) int {
	var sa = as[:1]
	var sb = bs[:1]
	return sa[0].Execute() + sb[0].Run()
}

func UseSliceOfMake() int {
	as := make([]*A, 2)
	bs := make([]*B, 2)
	return as[:1][0].Execute() + bs[:1][0].Run()
}

func UseSliceOfComposite() int {
	return []*A{&A{}}[:1][0].Execute() + []*B{&B{}}[:1][0].Run()
}
