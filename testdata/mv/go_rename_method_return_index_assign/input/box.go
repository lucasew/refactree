package box

type A struct{}

func (a *A) Run() int { return 1 }

type B struct{}

func (b *B) Run() int { return 2 }

func getAs() []*A { return []*A{&A{}} }
func getBs() []*B { return []*B{&B{}} }

func Use() int {
	as := getAs()
	bs := getBs()
	a := as[0]
	b := bs[0]
	return a.Run() + b.Run()
}
