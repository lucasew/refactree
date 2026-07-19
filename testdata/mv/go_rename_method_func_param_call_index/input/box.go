package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// fa func() []*A param call/index under foreign same-leaf.
func UseCallIndex(fa func() []*A, fb func() []*B) int {
	return fa()[0].Run() + fb()[0].Run()
}

func UseShort(fa func() []*A, fb func() []*B) int {
	as := fa()
	bs := fb()
	return as[0].Run() + bs[0].Run()
}

func UseVar(fa func() []*A, fb func() []*B) int {
	var as = fa()
	var bs = fb()
	return as[0].Run() + bs[0].Run()
}

func UseMap(fa func() map[string]*A, fb func() map[string]*B) int {
	return fa()["k"].Run() + fb()["k"].Run()
}

func UseVarFunc() int {
	var fa func() []*A = func() []*A { return []*A{&A{}} }
	var fb func() []*B = func() []*B { return []*B{&B{}} }
	return fa()[0].Run() + fb()[0].Run()
}

func UseParenResult(fa func() ([]*A), fb func() ([]*B)) int {
	return fa()[0].Run() + fb()[0].Run()
}

func UsePreservesB(fb func() []*B) int {
	return fb()[0].Run()
}
