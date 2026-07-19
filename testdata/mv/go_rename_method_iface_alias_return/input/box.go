package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type AS []*A
type BS []*B
type FA func() []*A
type FB func() []*B

type GA0 interface {
	GetAS() AS
	GetFA() FA
}
type GB0 interface {
	GetBS() BS
	GetFB() FB
}
type GA = GA0
type GB = GB0

func Use(ga GA, gb GB) int {
	return ga.GetAS()[0].Run() + gb.GetBS()[0].Run()
}

func UseFunc(ga GA, gb GB) int {
	return ga.GetFA()()[0].Run() + gb.GetFB()()[0].Run()
}

func UsePreservesB(gb GB) int {
	return gb.GetBS()[0].Run() + gb.GetFB()()[0].Run()
}
