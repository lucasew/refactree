package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Short-var from slice/map index must bind the element type so same-leaf
// foreign methods on other vars stay put while ours rewrite.
func UseSlice(xs []*A, ys []*B) int {
	a := xs[0]
	b := ys[0]
	return a.Run() + b.Run()
}

func UseMap(m map[string]*A, n map[string]*B) int {
	a := m["k"]
	b := n["k"]
	a2, ok := m["k2"]
	if !ok {
		return a.Run() + b.Run()
	}
	b2, ok2 := n["k2"]
	if !ok2 {
		return a.Run() + b.Run() + a2.Run()
	}
	return a.Run() + b.Run() + a2.Run() + b2.Run()
}

func UseParen(xs []*A) int {
	a := (xs)[0]
	return a.Run()
}

func UseVar(xs []*A, ys []*B) int {
	var a = xs[0]
	var b = ys[0]
	return a.Run() + b.Run()
}

func UseComposite() int {
	a := []*A{{}}[0]
	b := []*B{{}}[0]
	return a.Run() + b.Run()
}
