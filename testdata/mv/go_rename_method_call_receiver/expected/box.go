package box

type Box struct{}

func (b Box) Assist() int { return 1 }
func (b Box) Stay() int   { return 2 }

func Make() Box { return Box{} }

func Use() int {
	return Make().Assist() + Make().Stay()
}
