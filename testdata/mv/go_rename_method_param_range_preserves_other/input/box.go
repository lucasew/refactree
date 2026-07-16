package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// Params typed as A versus B — foreign same-leaf method must stay put.
func UseParams(a *A, b *B) int {
	return a.Run() + b.Run()
}

// Range over slice params — value vars follow element type.
func UseRange(as []*A, bs []*B) int {
	n := 0
	for _, a := range as {
		n += a.Run()
	}
	for _, b := range bs {
		n += b.Run()
	}
	return n
}

// Multi-name param form and method param.
func UseMulti(a, other *A, b *B) int {
	return a.Run() + other.Run() + b.Run()
}

func (r *A) Combine(b *B) int {
	return r.Run() + b.Run()
}
