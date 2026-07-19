package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// (*&as)[0] / (*&aa)[0] / (*&am)["k"] / (&aa)[0] peel address-of (then deref)
// of collections under foreign same-leaf methods.
func UseAddrDerefSlice(as []*A, bs []*B) int {
	return (*&as)[0].Execute() + (*&bs)[0].Run()
}

func UseAddrDerefArray(aa [1]*A, bb [1]*B) int {
	return (*&aa)[0].Execute() + (*&bb)[0].Run()
}

func UseAddrArray(aa [1]*A, bb [1]*B) int {
	return (&aa)[0].Execute() + (&bb)[0].Run()
}

func UseAddrDerefMap(am map[string]*A, bm map[string]*B) int {
	return (*&am)["k"].Execute() + (*&bm)["k"].Run()
}

func UseAddrDerefOnlyA(as []*A) int {
	return (*&as)[0].Execute()
}

func UsePreservesB(bs []*B, bb [1]*B, bm map[string]*B) int {
	return (*&bs)[0].Run() + (*&bb)[0].Run() + (&bb)[0].Run() + (*&bm)["k"].Run()
}
