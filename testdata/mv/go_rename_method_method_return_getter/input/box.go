package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type BoxA struct {
	Item *A
}
type BoxB struct {
	Item *B
}

func (ba *BoxA) Get() *A  { return ba.Item }
func (bb *BoxB) Get() *B  { return bb.Item }
func (ba *BoxA) Self() *BoxA { return ba }
func (bb *BoxB) Self() *BoxB { return bb }

func UseGetter(ba *BoxA, bb *BoxB) int {
	return ba.Get().Run() + bb.Get().Run()
}

func UseSelfGet(ba *BoxA, bb *BoxB) int {
	return ba.Self().Get().Run() + bb.Self().Get().Run()
}

func UsePreservesB(bb *BoxB) int {
	return bb.Get().Run() + bb.Self().Get().Run()
}
