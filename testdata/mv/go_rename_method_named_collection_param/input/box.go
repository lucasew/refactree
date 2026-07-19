package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Named collection types under foreign same-leaf.
type AS []*A
type BS []*B
type AM map[string]*A
type BM map[string]*B
type CA chan *A
type CB chan *B
type AA [1]*A
type BB [1]*B

func UseSlice(as AS, bs BS) int {
	return as[0].Run() + bs[0].Run()
}

func UseMap(am AM, bm BM) int {
	return am["k"].Run() + bm["k"].Run()
}

func UseChan(ca CA, cb CB) int {
	return (<-ca).Run() + (<-cb).Run()
}

func UsePtr(pas *AS, pbs *BS) int {
	return (*pas)[0].Run() + (*pbs)[0].Run()
}

func UseVar(as AS, bs BS) int {
	var xa AS = as
	var xb BS = bs
	return xa[0].Run() + xb[0].Run()
}

func UseRange(as AS, bs BS) int {
	n := 0
	for _, a := range as {
		n += a.Run()
	}
	for _, b := range bs {
		n += b.Run()
	}
	return n
}

func UseArray(aa AA, bb BB) int {
	return aa[0].Run() + bb[0].Run()
}

func UsePreservesB(bs BS) int {
	return bs[0].Run()
}
