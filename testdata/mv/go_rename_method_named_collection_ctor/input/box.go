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
type AA [1]*A
type BB [1]*B

func getAS() AS { return AS{&A{}} }
func getBS() BS { return BS{&B{}} }
func getAM() AM { return AM{"k": &A{}} }
func getBM() BM { return BM{"k": &B{}} }
func getAA() AA { return AA{&A{}} }
func getBB() BB { return BB{&B{}} }

func UseMake() int {
	as := make(AS, 1)
	bs := make(BS, 1)
	return as[0].Run() + bs[0].Run()
}

func UseMakeMap() int {
	am := make(AM)
	bm := make(BM)
	return am["k"].Run() + bm["k"].Run()
}

func UseMakeInline() int {
	return make(AS, 1)[0].Run() + make(BS, 1)[0].Run()
}

func UseComposite() int {
	as := AS{&A{}}
	bs := BS{&B{}}
	return as[0].Run() + bs[0].Run()
}

func UseCompositeInline() int {
	return AS{&A{}}[0].Run() + BS{&B{}}[0].Run()
}

func UseCompositeMap() int {
	return AM{"k": &A{}}["k"].Run() + BM{"k": &B{}}["k"].Run()
}

func UseAssert(x, y any) int {
	as := x.(AS)
	bs := y.(BS)
	return as[0].Run() + bs[0].Run()
}

func UseAssertInline(x, y any) int {
	return x.(AS)[0].Run() + y.(BS)[0].Run()
}

func UseConvert(x, y any) int {
	as := (AS)(x)
	bs := (BS)(y)
	return as[0].Run() + bs[0].Run()
}

func UseConvertInline(x, y any) int {
	return (AS)(x)[0].Run() + (BS)(y)[0].Run()
}

func UseNew() int {
	aa := new(AA)
	bb := new(BB)
	return aa[0].Run() + bb[0].Run()
}

func UseNewInline() int {
	return new(AA)[0].Run() + new(BB)[0].Run()
}

func UseFuncReturn() int {
	as := getAS()
	bs := getBS()
	return as[0].Run() + bs[0].Run()
}

func UseFuncReturnInline() int {
	return getAS()[0].Run() + getBS()[0].Run()
}

func UseFuncReturnMap() int {
	return getAM()["k"].Run() + getBM()["k"].Run()
}

func UseAppendNamed() int {
	as := append(AS{}, &A{})
	bs := append(BS{}, &B{})
	return as[0].Run() + bs[0].Run()
}

func UsePreservesB(bs BS) int {
	return bs[0].Run()
}
