package box

type A struct{}

func (a *A) Execute() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func getAs() ([]*A, error) { return []*A{&A{}}, nil }
func getBs() ([]*B, error) { return []*B{&B{}}, nil }

// Multi-return collection then index-assign local must bind element type so
// foreign same-leaf methods stay put while ours rewrite.
func Use() int {
	as, err := getAs()
	if err != nil {
		return 0
	}
	bs, err2 := getBs()
	if err2 != nil {
		return 0
	}
	a := as[0]
	b := bs[0]
	return a.Execute() + b.Run()
}
