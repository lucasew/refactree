package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func getA() ([]*A, error) { return []*A{&A{}}, nil }
func getB() ([]*B, error) { return []*B{&B{}}, nil }

func getANamed() (as []*A, err error) { return []*A{&A{}}, nil }
func getBNamed() (bs []*B, err error) { return []*B{&B{}}, nil }

func getAM() (map[string]*A, error) { return map[string]*A{"k": &A{}}, nil }
func getBM() (map[string]*B, error) { return map[string]*B{"k": &B{}}, nil }

// Multi-return same-file helpers: as, err := getA() must type as so as[0].Run
// renames while foreign same-leaf bs[0].Run is preserved.
func UseShort() int {
	as, err := getA()
	if err != nil {
		return 0
	}
	bs, err2 := getB()
	if err2 != nil {
		return 0
	}
	return as[0].Run() + bs[0].Run()
}

func UseVar() int {
	var as, err = getA()
	if err != nil {
		return 0
	}
	var bs, err2 = getB()
	if err2 != nil {
		return 0
	}
	return as[0].Run() + bs[0].Run()
}

func UseNamed() int {
	as, err := getANamed()
	if err != nil {
		return 0
	}
	bs, err2 := getBNamed()
	if err2 != nil {
		return 0
	}
	return as[0].Run() + bs[0].Run()
}

func UseMap() int {
	ma, err := getAM()
	if err != nil {
		return 0
	}
	mb, err2 := getBM()
	if err2 != nil {
		return 0
	}
	return ma["k"].Run() + mb["k"].Run()
}

func UseBlank() int {
	as, _ := getA()
	bs, _ := getB()
	return as[0].Run() + bs[0].Run()
}
