package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type SA struct {
	Items []*A
	M     map[string]*A
}
type SB struct {
	Items []*B
	M     map[string]*B
}

type WA struct {
	SA
}
type WB struct {
	SB
}

func UseNested(wa WA, wb WB) int {
	return wa.SA.Items[0].Run() + wb.SB.Items[0].Run()
}

func UseNestedMap(wa WA, wb WB) int {
	return wa.SA.M["k"].Run() + wb.SB.M["k"].Run()
}

func UseNestedAssign(wa WA, wb WB) int {
	items := wa.SA.Items
	other := wb.SB.Items
	return items[0].Run() + other[0].Run()
}

func UseNestedPtr(wa *WA, wb *WB) int {
	return wa.SA.Items[0].Run() + wb.SB.Items[0].Run()
}

func UsePreservesB(wb WB, wb2 *WB) int {
	return wb.SB.Items[0].Run() + wb2.SB.M["k"].Run()
}
