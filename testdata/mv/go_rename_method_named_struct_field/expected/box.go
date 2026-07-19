package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type SA struct {
	Items []*A
	M     map[string]*A
	Fa    func() []*A
}
type SB struct {
	Items []*B
	M     map[string]*B
	Fa    func() []*B
}

// Named non-embed fields (not promoted embeds).
type WA struct {
	Inner  SA
	InnerP *SA
}
type WB struct {
	Inner  SB
	InnerP *SB
}

// Multi-hop named path: wa.Mid.Inner.Items
type MidA struct {
	Inner SA
}
type MidB struct {
	Inner SB
}
type XA struct {
	Mid MidA
}
type XB struct {
	Mid MidB
}

func UseNamed(wa WA, wb WB) int {
	return wa.Inner.Items[0].Execute() + wb.Inner.Items[0].Run()
}

func UseNamedMap(wa WA, wb WB) int {
	return wa.Inner.M["k"].Execute() + wb.Inner.M["k"].Run()
}

func UseNamedAssign(wa WA, wb WB) int {
	items := wa.Inner.Items
	other := wb.Inner.Items
	return items[0].Execute() + other[0].Run()
}

func UseNamedPtr(wa *WA, wb *WB) int {
	return wa.Inner.Items[0].Execute() + wb.Inner.Items[0].Run()
}

func UseNamedPtrField(wa WA, wb WB) int {
	return wa.InnerP.Items[0].Execute() + wb.InnerP.Items[0].Run()
}

func UseNamedFuncField(wa WA, wb WB) int {
	return wa.Inner.Fa()[0].Execute() + wb.Inner.Fa()[0].Run()
}

func UseMultiHop(xa XA, xb XB) int {
	return xa.Mid.Inner.Items[0].Execute() + xb.Mid.Inner.Items[0].Run()
}

func UseStarMultiHop(xa *XA, xb *XB) int {
	return (*xa).Mid.Inner.Items[0].Execute() + (*xb).Mid.Inner.Items[0].Run()
}

func UsePreservesB(wb WB, wb2 *WB, xb XB) int {
	return wb.Inner.Items[0].Run() + wb2.Inner.M["k"].Run() + xb.Mid.Inner.Items[0].Run()
}
