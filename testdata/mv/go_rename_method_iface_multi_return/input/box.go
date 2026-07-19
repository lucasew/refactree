package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type AS []*A
type BS []*B
type AM map[string]*A
type BM map[string]*B

type GA interface {
	GetAS() (AS, error)
	GetAM() (AM, error)
}
type GB interface {
	GetBS() (BS, error)
	GetBM() (BM, error)
}

type SA struct{}
type SB struct{}

func (s SA) GetAS() (AS, error) { return AS{&A{}}, nil }
func (s SB) GetBS() (BS, error) { return BS{&B{}}, nil }

func UseIfaceShort(ga GA, gb GB) int {
	as, err := ga.GetAS()
	if err != nil {
		return 0
	}
	bs, err2 := gb.GetBS()
	if err2 != nil {
		return 0
	}
	return as[0].Run() + bs[0].Run()
}

func UseIfaceVar(ga GA, gb GB) int {
	var as, err = ga.GetAS()
	if err != nil {
		return 0
	}
	var bs, err2 = gb.GetBS()
	if err2 != nil {
		return 0
	}
	return as[0].Run() + bs[0].Run()
}

func UseIfaceMap(ga GA, gb GB) int {
	am, err := ga.GetAM()
	if err != nil {
		return 0
	}
	bm, err2 := gb.GetBM()
	if err2 != nil {
		return 0
	}
	return am["k"].Run() + bm["k"].Run()
}

func UseConcreteShort(sa SA, sb SB) int {
	as, err := sa.GetAS()
	if err != nil {
		return 0
	}
	bs, err2 := sb.GetBS()
	if err2 != nil {
		return 0
	}
	return as[0].Run() + bs[0].Run()
}

func UseBlank(ga GA, gb GB) int {
	as, _ := ga.GetAS()
	bs, _ := gb.GetBS()
	return as[0].Run() + bs[0].Run()
}

func UsePreservesB(gb GB, sb SB) int {
	bs, _ := gb.GetBS()
	bs2, _ := sb.GetBS()
	return bs[0].Run() + bs2[0].Run()
}
