package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func getA() []*A { return []*A{&A{}} }
func getB() []*B { return []*B{&B{}} }

func getANamed() (as []*A) { return []*A{&A{}} }
func getBNamed() (bs []*B) { return []*B{&B{}} }

func getAM() map[string]*A { return map[string]*A{"k": &A{}} }
func getBM() map[string]*B { return map[string]*B{"k": &B{}} }

// Same-file func-return collection short-var / var / inline index must follow
// the result element type so foreign same-leaf methods are not rewritten.
func UseShort() int {
	as := getA()
	bs := getB()
	return as[0].Execute() + bs[0].Run()
}

func UseVar() int {
	var as = getA()
	var bs = getB()
	return as[0].Execute() + bs[0].Run()
}

func UseInline() int {
	return getA()[0].Execute() + getB()[0].Run()
}

func UseNamed() int {
	as := getANamed()
	bs := getBNamed()
	return as[0].Execute() + bs[0].Run()
}

func UseMap() int {
	ma := getAM()
	mb := getBM()
	return ma["k"].Execute() + mb["k"].Run()
}

func UseMapInline() int {
	return getAM()["k"].Execute() + getBM()["k"].Run()
}
