package box

type A struct{}
type B struct{}

func (A) Execute() int { return 1 }
func (B) Run() int { return 2 }

func Get[K comparable, V any](m map[K]V, k K) V { return m[k] }

func First2[T, U any](a T, b U) T { return a }

func UseGet() int {
	return Get(map[string]A{"x": {}}, "x").Execute() + Get(map[string]B{"y": {}}, "y").Run()
}

func UseGetAssign() int {
	a := Get(map[string]A{"x": {}}, "x")
	b := Get(map[string]B{"y": {}}, "y")
	return a.Execute() + b.Run()
}

func UseFirst2() int {
	return First2(A{}, B{}).Execute() + First2(B{}, A{}).Run()
}

func UseFirst2Assign() int {
	a := First2(A{}, B{})
	b := First2(B{}, A{})
	return a.Execute() + b.Run()
}

func UsePreservesB() int {
	return Get(map[string]B{"y": {}}, "y").Run() + First2(B{}, A{}).Run()
}
