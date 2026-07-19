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

func Use(wa *WA, wb *WB) int {
	return wa.BA.Get().Execute() + wb.BB.Get().Run()
}

func UsePreservesB(wb *WB) int {
	return wb.BB.Get().Run()
}
