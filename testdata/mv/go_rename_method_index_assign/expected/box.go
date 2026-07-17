package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Short-var from slice/map index must bind the element type so same-leaf
// foreign methods on other vars stay put while ours rewrite.
func UseSlice(xs []*A, ys []*B) int {
	a := xs[0]
	b := ys[0]
	return a.Execute() + b.Run()
}

func UseMap(m map[string]*A, n map[string]*B) int {
	a := m["k"]
	b := n["k"]
	a2, ok := m["k2"]
	if !ok {
		return a.Execute() + b.Run()
	}
	b2, ok2 := n["k2"]
	if !ok2 {
		return a.Execute() + b.Run() + a2.Execute()
	}
	return a.Execute() + b.Run() + a2.Execute() + b2.Run()
}

func UseParen(xs []*A) int {
	a := (xs)[0]
	return a.Execute()
}

func UseVar(xs []*A, ys []*B) int {
	var a = xs[0]
	var b = ys[0]
	return a.Execute() + b.Run()
}

func UseComposite() int {
	a := []*A{{}}[0]
	b := []*B{{}}[0]
	return a.Execute() + b.Run()
}
