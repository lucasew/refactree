package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type AS []*A
type BS []*B
type FA func() []*A
type FB func() []*B

type GA interface {
	GetAS() AS
	GetFA() FA
}
type GB interface {
	GetBS() BS
	GetFB() FB
}

// Embedded interfaces promote GetAS/GetFA under foreign same-leaf.
type HA interface {
	GA
}
type HB interface {
	GB
}

func Use(ha HA, hb HB) int {
	return ha.GetAS()[0].Execute() + hb.GetBS()[0].Run()
}

func UseShort(ha HA, hb HB) int {
	as := ha.GetAS()
	bs := hb.GetBS()
	return as[0].Execute() + bs[0].Run()
}

func UseFunc(ha HA, hb HB) int {
	return ha.GetFA()()[0].Execute() + hb.GetFB()()[0].Run()
}

func UsePreservesB(hb HB) int {
	return hb.GetBS()[0].Run() + hb.GetFB()()[0].Run()
}
