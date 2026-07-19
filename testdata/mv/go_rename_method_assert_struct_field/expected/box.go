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

func UseAssertInline(x, y any) int {
	return x.(SA).Items[0].Execute() + y.(SB).Items[0].Run()
}

func UseAssertMap(x, y any) int {
	return x.(SA).M["k"].Execute() + y.(SB).M["k"].Run()
}

func UseAssertPtrInline(x, y any) int {
	return x.(*SA).Items[0].Execute() + y.(*SB).Items[0].Run()
}

func UseAssertShort(x, y any) int {
	sa := x.(SA)
	sb := y.(SB)
	return sa.Items[0].Execute() + sb.Items[0].Run()
}

func UseAssertColl(x, y any) int {
	return x.([]SA)[0].Items[0].Execute() + y.([]SB)[0].Items[0].Run()
}

func UsePreservesB(y any) int {
	return y.(SB).Items[0].Run() + y.(SB).M["k"].Run()
}
