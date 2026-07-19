package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// (*pas)[0] / (*pam)["k"] peel through pointer-to-collection params so foreign
// same-leaf methods on *[]*B / *map[string]*B are not rewritten.
func UsePtrSlice(pas *[]*A, pbs *[]*B) int {
	return (*pas)[0].Run() + (*pbs)[0].Run()
}

func UsePtrMap(pam *map[string]*A, pbm *map[string]*B) int {
	return (*pam)["k"].Run() + (*pbm)["k"].Run()
}

func UsePtrSliceOnlyA(pas *[]*A) int {
	return (*pas)[0].Run()
}

func UsePreservesB(pbs *[]*B, pbm *map[string]*B) int {
	return (*pbs)[0].Run() + (*pbm)["k"].Run()
}
