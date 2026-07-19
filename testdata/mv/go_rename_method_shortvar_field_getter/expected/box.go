package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type BoxA struct{ X *A }
type BoxB struct{ Y *B }

func (ba *BoxA) Get() *A { return ba.X }
func (bb *BoxB) Get() *B { return bb.Y }

type WA struct{ BA BoxA }
type WB struct{ BB BoxB }

func UseShort(wa *WA, wb *WB) int {
	ba := wa.BA
	bb := wb.BB
	return ba.Get().Execute() + bb.Get().Run()
}

func UseVar(wa *WA, wb *WB) int {
	var ba = wa.BA
	var bb = wb.BB
	return ba.Get().Execute() + bb.Get().Run()
}

func UseShortPtr(wa *WA, wb *WB) int {
	ba := &wa.BA
	bb := &wb.BB
	return ba.Get().Execute() + bb.Get().Run()
}

func UsePreservesB(wb *WB) int {
	bb := wb.BB
	return bb.Get().Run()
}
