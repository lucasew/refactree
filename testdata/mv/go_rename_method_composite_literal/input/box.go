package box

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Use() int {
	return Box{}.Helper() + Box{}.Stay()
}

func UsePtr() int {
	return (&Box{}).Helper()
}
