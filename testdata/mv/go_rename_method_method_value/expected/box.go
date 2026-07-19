package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type SA struct{}
type SB struct{}

func (s SA) Get() []*A { return []*A{&A{}} }
func (s SB) Get() []*B { return []*B{&B{}} }
func (s *SA) GetM() map[string]*A { return map[string]*A{"k": &A{}} }
func (s *SB) GetM() map[string]*B { return map[string]*B{"k": &B{}} }

func Use(sa SA, sb SB) int {
	fa := sa.Get
	fb := sb.Get
	return fa()[0].Execute() + fb()[0].Run()
}

func UseVar(sa SA, sb SB) int {
	var fa = sa.Get
	var fb = sb.Get
	return fa()[0].Execute() + fb()[0].Run()
}

func UsePtr(sa *SA, sb *SB) int {
	fa := sa.Get
	fb := sb.Get
	return fa()[0].Execute() + fb()[0].Run()
}

func UseMap(sa *SA, sb *SB) int {
	fm := sa.GetM
	gm := sb.GetM
	return fm()["k"].Execute() + gm()["k"].Run()
}

func UsePreservesB(sb SB, psb *SB) int {
	fb := sb.Get
	gm := psb.GetM
	return fb()[0].Run() + gm()["k"].Run()
}
