package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

type AS []*A
type BS []*B
type AM map[string]*A
type BM map[string]*B

func UseSlice(x any, y any) int {
	n := 0
	switch as := x.(type) {
	case []*A:
		n += as[0].Execute()
	}
	switch bs := y.(type) {
	case []*B:
		n += bs[0].Run()
	}
	return n
}

func UseNamed(x any, y any) int {
	n := 0
	switch as := x.(type) {
	case AS:
		n += as[0].Execute()
	}
	switch bs := y.(type) {
	case BS:
		n += bs[0].Run()
	}
	return n
}

func UseMap(x any, y any) int {
	n := 0
	switch am := x.(type) {
	case map[string]*A:
		n += am["k"].Execute()
	}
	switch bm := y.(type) {
	case map[string]*B:
		n += bm["k"].Run()
	}
	return n
}

func UseNamedMap(x any, y any) int {
	n := 0
	switch am := x.(type) {
	case AM:
		n += am["k"].Execute()
	}
	switch bm := y.(type) {
	case BM:
		n += bm["k"].Run()
	}
	return n
}

func UsePreservesB(y any) int {
	switch bs := y.(type) {
	case []*B:
		return bs[0].Run()
	case BS:
		return bs[0].Run()
	case map[string]*B:
		return bs["k"].Run()
	case BM:
		return bs["k"].Run()
	default:
		return 0
	}
}
