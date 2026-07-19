package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type AS []*A
type BS []*B
type FA func() []*A
type FB func() []*B

type SA struct{}
type SB struct{}

func (s SA) GetAS() AS { return AS{&A{}} }
func (s SB) GetBS() BS { return BS{&B{}} }
func (s *SA) GetFA() FA { return func() []*A { return []*A{&A{}} } }
func (s *SB) GetFB() FB { return func() []*B { return []*B{&B{}} } }

func Use(sa SA, sb SB) int {
	return sa.GetAS()[0].Run() + sb.GetBS()[0].Run()
}

func UseShort(sa SA, sb SB) int {
	as := sa.GetAS()
	bs := sb.GetBS()
	return as[0].Run() + bs[0].Run()
}

func UsePtr(sa *SA, sb *SB) int {
	return sa.GetAS()[0].Run() + sb.GetBS()[0].Run()
}

func UseFunc(sa *SA, sb *SB) int {
	return sa.GetFA()()[0].Run() + sb.GetFB()()[0].Run()
}

func UseFuncShort(sa *SA, sb *SB) int {
	fa := sa.GetFA()
	fb := sb.GetFB()
	return fa()[0].Run() + fb()[0].Run()
}

func UsePreservesB(sb SB, psb *SB) int {
	return sb.GetBS()[0].Run() + psb.GetFB()()[0].Run()
}
