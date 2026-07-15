package box

type Box struct{}

func (Box) Helper() int { return 1 }
func (Box) Stay() int   { return 2 }

func Call(t interface{ Helper() int }) int {
	return t.Helper()
}

func Use() int {
	return Call(Box{}) + Box{}.Stay()
}
