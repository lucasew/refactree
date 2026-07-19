package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type BoxA struct{ X *A }
type BoxB struct{ Y *B }

func (ba *BoxA) Get() *A     { return ba.X }
func (bb *BoxB) Get() *B     { return bb.Y }
func (ba *BoxA) Self() *BoxA { return ba }
func (bb *BoxB) Self() *BoxB { return bb }

type WA struct{ BA BoxA }
type WB struct{ BB BoxB }

func UseGet(ba *BoxA, bb *BoxB) int {
	a := ba.Get()
	b := bb.Get()
	return a.Run() + b.Run()
}

func UseVarGet(ba *BoxA, bb *BoxB) int {
	var a = ba.Get()
	var b = bb.Get()
	return a.Run() + b.Run()
}

func UseSelf(ba *BoxA, bb *BoxB) int {
	xa := ba.Self()
	xb := bb.Self()
	return xa.Get().Run() + xb.Get().Run()
}

func UseFieldGet(wa *WA, wb *WB) int {
	a := wa.BA.Get()
	b := wb.BB.Get()
	return a.Run() + b.Run()
}

func UsePreservesB(bb *BoxB, wb *WB) int {
	b := bb.Get()
	xb := bb.Self()
	b2 := wb.BB.Get()
	return b.Run() + xb.Get().Run() + b2.Run()
}
