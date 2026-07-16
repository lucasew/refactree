package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Channel receive short-vars — payload type follows the chan element.
func UseChan(ca <-chan *A, cb <-chan *B) int {
	a := <-ca
	b := <-cb
	a2, ok := <-ca
	if !ok {
		return a.Execute() + b.Run()
	}
	return a.Execute() + b.Run() + a2.Execute()
}

// Select receive cases — bind per arm so foreign same-leaf stays put.
func UseSelect(ca <-chan *A, cb <-chan *B) int {
	select {
	case a := <-ca:
		return a.Execute()
	case b := <-cb:
		return b.Run()
	}
	return 0
}

// Map range — keys and values both need concrete types when same-leaf competes.
func UseMap(m map[*B]*A) int {
	n := 0
	for b, a := range m {
		n += b.Run() + a.Execute()
	}
	for b := range m {
		n += b.Run()
	}
	return n
}
