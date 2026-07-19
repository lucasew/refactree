package box

type A struct{}

func (a *A) Execute() int { return 1 }

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

func getSA() SA { return SA{} }
func getSB() SB { return SB{} }
func getSAP() *SA { return &SA{} }
func getSBP() *SB { return &SB{} }

func UseInline() int {
	return getSA().Items[0].Execute() + getSB().Items[0].Run()
}

func UseMap() int {
	return getSA().M["k"].Execute() + getSB().M["k"].Run()
}

func UseShort() int {
	sa := getSA()
	sb := getSB()
	return sa.Items[0].Execute() + sb.Items[0].Run()
}

func UsePtrReturn() int {
	return getSAP().Items[0].Execute() + getSBP().Items[0].Run()
}

func UseAssignField() int {
	items := getSA().Items
	other := getSB().Items
	return items[0].Execute() + other[0].Run()
}

func UsePreservesB() int {
	return getSB().Items[0].Run() + getSB().M["k"].Run()
}
