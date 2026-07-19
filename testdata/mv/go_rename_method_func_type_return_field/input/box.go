package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type FA func() []*A
type FB func() []*B

type BoxA struct{ Fa FA }
type BoxB struct{ Fb FB }

func getFA() FA { return func() []*A { return []*A{&A{}} } }
func getFB() FB { return func() []*B { return []*B{&B{}} } }

func UseField(xa BoxA, xb BoxB) int {
	return xa.Fa()[0].Run() + xb.Fb()[0].Run()
}

func UseFieldShort(xa BoxA, xb BoxB) int {
	fa := xa.Fa
	fb := xb.Fb
	return fa()[0].Run() + fb()[0].Run()
}

func UseGetCall() int {
	return getFA()()[0].Run() + getFB()()[0].Run()
}

func UseGetShort() int {
	fa := getFA()
	fb := getFB()
	return fa()[0].Run() + fb()[0].Run()
}

func UseGetVar() int {
	var fa = getFA()
	var fb = getFB()
	return fa()[0].Run() + fb()[0].Run()
}

func UsePreservesB(fb FB) int {
	return fb()[0].Run()
}
