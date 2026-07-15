package box

type Box struct{}

func (Box) Assist() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use() int {
	return Box{}.Assist() + Box{}.Stay()
}

func UsePtr() int {
	return (&Box{}).Assist()
}
