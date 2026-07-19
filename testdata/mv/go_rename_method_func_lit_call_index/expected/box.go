package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// fa := func() []*A local func-literal collection return under foreign same-leaf.
func UseShortFuncLit() int {
	fa := func() []*A { return []*A{&A{}} }
	fb := func() []*B { return []*B{&B{}} }
	return fa()[0].Execute() + fb()[0].Run()
}

func UseShortFuncLitAssign() int {
	fa := func() []*A { return []*A{&A{}} }
	fb := func() []*B { return []*B{&B{}} }
	as := fa()
	bs := fb()
	return as[0].Execute() + bs[0].Run()
}

func UseShortFuncLitMap() int {
	fa := func() map[string]*A { return map[string]*A{"k": &A{}} }
	fb := func() map[string]*B { return map[string]*B{"k": &B{}} }
	return fa()["k"].Execute() + fb()["k"].Run()
}

func UseShortMulti() int {
	fa, fb := func() []*A { return []*A{&A{}} }, func() []*B { return []*B{&B{}} }
	return fa()[0].Execute() + fb()[0].Run()
}

func UseVarUnTyped() int {
	var fa = func() []*A { return []*A{&A{}} }
	var fb = func() []*B { return []*B{&B{}} }
	return fa()[0].Execute() + fb()[0].Run()
}

func UseParenResult() int {
	fa := func() ([]*A) { return []*A{&A{}} }
	fb := func() ([]*B) { return []*B{&B{}} }
	return fa()[0].Execute() + fb()[0].Run()
}

func UsePreservesB() int {
	fb := func() []*B { return []*B{&B{}} }
	return fb()[0].Run()
}
