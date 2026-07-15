package box

type Box struct{}

func (b Box) Assist() int { return 1 }
func (b Box) Stay() int   { return 2 }

func Use(xs []Box) int {
	return xs[0].Assist() + xs[0].Stay()
}
