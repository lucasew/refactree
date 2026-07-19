package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Chained aliases of named collections under foreign same-leaf.
type AS0 []*A
type BS0 []*B
type AS = AS0
type BS = BS0

func UseParam(as AS, bs BS) int {
	return as[0].Run() + bs[0].Run()
}

func UseMake() int {
	as := make(AS, 1)
	bs := make(BS, 1)
	return as[0].Run() + bs[0].Run()
}

func UseMakeInline() int {
	return make(AS, 1)[0].Run() + make(BS, 1)[0].Run()
}

func UseComposite() int {
	return AS{&A{}}[0].Run() + BS{&B{}}[0].Run()
}

func UseAssert(x, y any) int {
	return x.(AS)[0].Run() + y.(BS)[0].Run()
}

func UseConvert(x, y any) int {
	return (AS)(x)[0].Run() + (BS)(y)[0].Run()
}

func getAS() AS { return AS{&A{}} }
func getBS() BS { return BS{&B{}} }

func UseFuncReturn() int {
	return getAS()[0].Run() + getBS()[0].Run()
}

func UseFuncReturnShort() int {
	as := getAS()
	bs := getBS()
	return as[0].Run() + bs[0].Run()
}

type AM0 map[string]*A
type BM0 map[string]*B
type AM = AM0
type BM = BM0

func UseMap(am AM, bm BM) int {
	return am["k"].Run() + bm["k"].Run()
}

func UseMapMake() int {
	am := make(AM)
	bm := make(BM)
	return am["k"].Run() + bm["k"].Run()
}

type CA0 chan *A
type CB0 chan *B
type CA = CA0
type CB = CB0

func UseChan(ca CA, cb CB) int {
	return (<-ca).Run() + (<-cb).Run()
}

// Defined type whose base is a named collection (type AS1 AS0).
func UseDefinedBase() int {
	type AS1 AS0
	type BS1 BS0
	as := AS1{&A{}}
	bs := BS1{&B{}}
	return as[0].Run() + bs[0].Run()
}

// Multi-hop alias chain: AS2 = AS = AS0.
func UseMultiChain() int {
	type AS2 = AS
	type BS2 = BS
	as := make(AS2, 1)
	bs := make(BS2, 1)
	return as[0].Run() + bs[0].Run()
}

// Pointer-to-named collection: type PAS *AS0.
type PAS *AS0
type PBS *BS0
type PAS2 = PAS
type PBS2 = PBS

func UsePtrNamed(pas PAS, pbs PBS) int {
	return (*pas)[0].Run() + (*pbs)[0].Run()
}

func UsePtrNamedAlias(pas PAS2, pbs PBS2) int {
	return (*pas)[0].Run() + (*pbs)[0].Run()
}

// Chained aliases of named function types.
type FA0 func() []*A
type FB0 func() []*B
type FA = FA0
type FB = FB0

func UseFunc(fa FA, fb FB) int {
	return fa()[0].Run() + fb()[0].Run()
}

func UseFuncShort(fa FA, fb FB) int {
	as := fa()
	bs := fb()
	return as[0].Run() + bs[0].Run()
}

func getFA() FA { return func() []*A { return []*A{&A{}} } }
func getFB() FB { return func() []*B { return []*B{&B{}} } }

func UseFuncRetNested() int {
	return getFA()()[0].Run() + getFB()()[0].Run()
}

func UseFuncRetShort() int {
	fa := getFA()
	fb := getFB()
	return fa()[0].Run() + fb()[0].Run()
}

type Box struct {
	Fa FA
	Fb FB
}

func UseFuncField(xa, xb Box) int {
	return xa.Fa()[0].Run() + xb.Fb()[0].Run()
}

// Defined func type base + multi-hop.
func UseFuncDefined(fa0 FA0, fb0 FB0) int {
	type FA1 = FA
	type FB1 = FB
	var fa FA1 = fa0
	var fb FB1 = fb0
	return fa()[0].Run() + fb()[0].Run()
}

type AA0 [1]*A
type BB0 [1]*B
type AA = AA0
type BB = BB0

func UseArrayNew() int {
	aa := new(AA)
	bb := new(BB)
	return aa[0].Run() + bb[0].Run()
}

func UsePreservesB(bs BS, fb FB, bm BM, pbs PBS, cb CB) int {
	return bs[0].Run() + fb()[0].Run() + bm["k"].Run() + (*pbs)[0].Run() + (<-cb).Run()
}
