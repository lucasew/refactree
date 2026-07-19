package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type AS []*A
type BS []*B
type AM map[string]*A
type BM map[string]*B
type FA func() []*A
type FB func() []*B

type GA interface {
	GetAS() AS
	GetAM() AM
	GetFA() FA
}
type GB interface {
	GetBS() BS
	GetBM() BM
	GetFB() FB
}

func UseColl(ga GA, gb GB) int {
	return ga.GetAS()[0].Execute() + gb.GetBS()[0].Run()
}

func UseCollShort(ga GA, gb GB) int {
	as := ga.GetAS()
	bs := gb.GetBS()
	return as[0].Execute() + bs[0].Run()
}

func UseMap(ga GA, gb GB) int {
	return ga.GetAM()["k"].Execute() + gb.GetBM()["k"].Run()
}

func UseMapShort(ga GA, gb GB) int {
	am := ga.GetAM()
	bm := gb.GetBM()
	return am["k"].Execute() + bm["k"].Run()
}

func UseFunc(ga GA, gb GB) int {
	return ga.GetFA()()[0].Execute() + gb.GetFB()()[0].Run()
}

func UseFuncShort(ga GA, gb GB) int {
	fa := ga.GetFA()
	fb := gb.GetFB()
	return fa()[0].Execute() + fb()[0].Run()
}

func UsePreservesB(gb GB) int {
	return gb.GetBS()[0].Run() + gb.GetBM()["k"].Run() + gb.GetFB()()[0].Run()
}
