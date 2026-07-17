package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

// append([]*T{}, …) and composite short-var index locals must follow
// element/value type so foreign same-leaf methods are not rewritten.
func UseAppendShort() int {
	as := append([]*A{}, &A{})
	bs := append([]*B{}, &B{})
	return as[0].Run() + bs[0].Run()
}

func UseAppendVar() int {
	var as = append([]*A{}, &A{})
	var bs = append([]*B{}, &B{})
	return as[0].Run() + bs[0].Run()
}

func UseAppendInline() int {
	return append([]*A{}, &A{})[0].Run() + append([]*B{}, &B{})[0].Run()
}

func UseAppendMake() int {
	as := append(make([]*A, 0), &A{})
	bs := append(make([]*B, 0), &B{})
	return as[0].Run() + bs[0].Run()
}

func UseCompositeShort() int {
	as := []*A{&A{}}
	bs := []*B{&B{}}
	return as[0].Run() + bs[0].Run()
}

func UseCompositeVar() int {
	var as = []*A{&A{}}
	var bs = []*B{&B{}}
	return as[0].Run() + bs[0].Run()
}

func UseCompositeMap() int {
	ma := map[string]*A{"k": &A{}}
	mb := map[string]*B{"k": &B{}}
	return ma["k"].Run() + mb["k"].Run()
}

func UseCompositeInline() int {
	return []*A{&A{}}[0].Run() + []*B{&B{}}[0].Run()
}
