package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type FA func() []*A
type FB func() []*B

func UseCallIndex(fa FA, fb FB) int {
	return fa()[0].Execute() + fb()[0].Run()
}

func UseShort(fa FA, fb FB) int {
	as := fa()
	bs := fb()
	return as[0].Execute() + bs[0].Run()
}

func UseVar(fa FA, fb FB) int {
	var as = fa()
	var bs = fb()
	return as[0].Execute() + bs[0].Run()
}

type FMA = func() map[string]*A
type FMB = func() map[string]*B

func UseMap(fa FMA, fb FMB) int {
	return fa()["k"].Execute() + fb()["k"].Run()
}

func UseVarTyped() int {
	var fa FA = func() []*A { return []*A{&A{}} }
	var fb FB = func() []*B { return []*B{&B{}} }
	return fa()[0].Execute() + fb()[0].Run()
}

type FPA = func() ([]*A)
type FPB = func() ([]*B)

func UseParenResult(fa FPA, fb FPB) int {
	return fa()[0].Execute() + fb()[0].Run()
}

func UsePreservesB(fb FB) int {
	return fb()[0].Run()
}
